// Package pwdless provides JSON Web Token (JWT) authentication and authorization middleware.
// It implements a passwordless authentication flow by sending login tokens vie email which are then exchanged for JWT access and refresh tokens.
package pwdless

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dhax/go-base/auth/jwt"
	"github.com/dhax/go-base/email"
	"github.com/dhax/go-base/logging"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"github.com/gofrs/uuid"
	"github.com/labstack/echo/v4"
	"github.com/mssola/user_agent"
	"github.com/sirupsen/logrus"
)

// AuthStorer defines database operations on accounts and tokens.
type AuthStorer interface {
	GetAccount(id int) (*Account, error)
	GetAccountByEmail(email string) (*Account, error)
	UpdateAccount(a *Account) error

	GetToken(token string) (*jwt.Token, error)
	CreateOrUpdateToken(t *jwt.Token) error
	DeleteToken(t *jwt.Token) error
	PurgeExpiredToken() error
}

// Resource implements passwordless account authentication against a database.
type Resource struct {
	LoginAuth *LoginTokenAuth
	TokenAuth *jwt.TokenAuth
	Store     AuthStorer
	Mailer    email.Mailer
}

// NewResource returns a configured authentication resource.
func NewResource(authStore AuthStorer, mailer email.Mailer) (*Resource, error) {
	loginAuth, err := NewLoginTokenAuth()
	if err != nil {
		return nil, err
	}

	tokenAuth, err := jwt.NewTokenAuth()
	if err != nil {
		return nil, err
	}

	resource := &Resource{
		LoginAuth: loginAuth,
		TokenAuth: tokenAuth,
		Store:     authStore,
		Mailer:    mailer,
	}

	resource.choresTicker()

	return resource, nil
}

// RegisterRoutes registers the necessary routes for passwordless authentication
// flow on the provided echo group.
func (rs *Resource) RegisterRoutes(g *echo.Group) {
	g.POST("/login", rs.login)
	g.POST("/token", rs.token)

	// route level middleware keeps the refresh flow scoped to these two routes
	// without registering the catch-all routes an echo group would add.
	refreshAuth := []echo.MiddlewareFunc{rs.TokenAuth.Verifier(), jwt.AuthenticateRefreshJWT}
	g.POST("/refresh", rs.refresh, refreshAuth...)
	g.POST("/logout", rs.logout, refreshAuth...)
}

func log(c echo.Context) logrus.FieldLogger {
	return logging.GetLogEntry(c)
}

type loginRequest struct {
	Email string
}

func (body *loginRequest) Bind(r *http.Request) error {
	body.Email = strings.TrimSpace(body.Email)
	body.Email = strings.ToLower(body.Email)

	return validation.ValidateStruct(body,
		validation.Field(&body.Email, validation.Required, is.Email),
	)
}

func (rs *Resource) login(c echo.Context) error {
	body := &loginRequest{}
	if err := bind(c, body); err != nil {
		log(c).WithField("email", body.Email).Warn(err)
		return ErrUnauthorized(ErrInvalidLogin)
	}

	acc, err := rs.Store.GetAccountByEmail(body.Email)
	if err != nil {
		log(c).WithField("email", body.Email).Warn(err)
		return ErrUnauthorized(ErrUnknownLogin)
	}

	if !acc.CanLogin() {
		return ErrUnauthorized(ErrLoginDisabled)
	}

	lt := rs.LoginAuth.CreateToken(acc.ID)
	tokenURL, _ := url.JoinPath(rs.LoginAuth.loginURL, lt.Token)

	go func() {
		content := ContentLoginToken{
			Email:  acc.Email,
			Name:   acc.Name,
			URL:    tokenURL,
			Token:  lt.Token,
			Expiry: lt.Expiry,
		}

		msg := LoginTokenEmail(acc.Name, acc.Email, content)

		if err := rs.Mailer.Send(msg); err != nil {
			log(c).WithField("module", "email").Error(err)
		}
	}()

	return c.JSON(http.StatusOK, struct{}{})
}

type tokenRequest struct {
	Token string `json:"token"`
}

type tokenResponse struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
}

func (body *tokenRequest) Bind(r *http.Request) error {
	body.Token = strings.TrimSpace(body.Token)

	return validation.ValidateStruct(body,
		validation.Field(&body.Token, validation.Required, is.Alphanumeric),
	)
}

func (rs *Resource) token(c echo.Context) error {
	body := &tokenRequest{}
	if err := bind(c, body); err != nil {
		log(c).Warn(err)
		return ErrUnauthorized(ErrLoginToken)
	}

	id, err := rs.LoginAuth.GetAccountID(body.Token)
	if err != nil {
		return ErrUnauthorized(ErrLoginToken)
	}

	acc, err := rs.Store.GetAccount(id)
	if err != nil {
		// account deleted before login token expired
		return ErrUnauthorized(ErrUnknownLogin)
	}

	if !acc.CanLogin() {
		return ErrUnauthorized(ErrLoginDisabled)
	}

	ua := user_agent.New(c.Request().UserAgent())
	browser, _ := ua.Browser()

	token := &jwt.Token{
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     time.Now().Add(rs.TokenAuth.JwtRefreshExpiry),
		UpdatedAt:  time.Now(),
		AccountID:  acc.ID,
		Mobile:     ua.Mobile(),
		Identifier: fmt.Sprintf("%s on %s", browser, ua.OS()),
	}

	if err := rs.Store.CreateOrUpdateToken(token); err != nil {
		log(c).Error(err)
		return ErrInternalServerError
	}

	access, refresh, err := rs.TokenAuth.GenTokenPair(acc.Claims(), token.Claims())
	if err != nil {
		log(c).Error(err)
		return ErrInternalServerError
	}

	acc.LastLogin = time.Now()
	if err := rs.Store.UpdateAccount(acc); err != nil {
		log(c).Error(err)
		return ErrInternalServerError
	}

	return c.JSON(http.StatusOK, &tokenResponse{
		Access:  access,
		Refresh: refresh,
	})
}

func (rs *Resource) refresh(c echo.Context) error {
	rt := jwt.RefreshTokenFromCtx(c)

	token, err := rs.Store.GetToken(rt)
	if err != nil {
		return ErrUnauthorized(jwt.ErrTokenExpired)
	}

	if time.Now().After(token.Expiry) {
		rs.Store.DeleteToken(token)
		return ErrUnauthorized(jwt.ErrTokenExpired)
	}

	acc, err := rs.Store.GetAccount(token.AccountID)
	if err != nil {
		return ErrUnauthorized(ErrUnknownLogin)
	}

	if !acc.CanLogin() {
		return ErrUnauthorized(ErrLoginDisabled)
	}

	token.Token = uuid.Must(uuid.NewV4()).String()
	token.Expiry = time.Now().Add(rs.TokenAuth.JwtRefreshExpiry)
	token.UpdatedAt = time.Now()

	access, refresh, err := rs.TokenAuth.GenTokenPair(acc.Claims(), token.Claims())
	if err != nil {
		log(c).Error(err)
		return ErrInternalServerError
	}

	if err := rs.Store.CreateOrUpdateToken(token); err != nil {
		log(c).Error(err)
		return ErrInternalServerError
	}

	acc.LastLogin = time.Now()
	if err := rs.Store.UpdateAccount(acc); err != nil {
		log(c).Error(err)
		return ErrInternalServerError
	}

	return c.JSON(http.StatusOK, &tokenResponse{
		Access:  access,
		Refresh: refresh,
	})
}

func (rs *Resource) logout(c echo.Context) error {
	rt := jwt.RefreshTokenFromCtx(c)
	token, err := rs.Store.GetToken(rt)
	if err != nil {
		return ErrUnauthorized(jwt.ErrTokenExpired)
	}
	rs.Store.DeleteToken(token)

	return c.JSON(http.StatusOK, struct{}{})
}

// binder is implemented by request types that validate themselves after the
// request body has been decoded.
type binder interface {
	Bind(r *http.Request) error
}

// bind decodes the request body into v and then runs its own validation,
// mirroring the decode-then-validate order of the original implementation.
func bind(c echo.Context, v binder) error {
	if err := c.Bind(v); err != nil {
		return err
	}
	return v.Bind(c.Request())
}
