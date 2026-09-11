package app

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/labstack/echo/v4"

	"github.com/dhax/go-base/auth/jwt"
	"github.com/dhax/go-base/auth/pwdless"
)

// The list of error types returned from account resource.
var (
	ErrAccountValidation = errors.New("account validation error")
)

// AccountStore defines database operations for account.
type AccountStore interface {
	Get(id int) (*pwdless.Account, error)
	Update(*pwdless.Account) error
	Delete(*pwdless.Account) error
	UpdateToken(*jwt.Token) error
	DeleteToken(*jwt.Token) error
}

// AccountResource implements account management handler.
type AccountResource struct {
	Store AccountStore
}

// NewAccountResource creates and returns an account resource.
func NewAccountResource(store AccountStore) *AccountResource {
	return &AccountResource{
		Store: store,
	}
}

func (rs *AccountResource) registerRoutes(parent *echo.Group) {
	g := parent.Group("/account", rs.accountCtx)

	// both the bare and the trailing slash form are registered to preserve the
	// paths chi's Mount used to answer on.
	g.GET("", rs.get)
	g.GET("/", rs.get)
	g.PUT("", rs.update)
	g.PUT("/", rs.update)
	g.DELETE("", rs.delete)
	g.DELETE("/", rs.delete)

	g.PUT("/token/:tokenID", rs.updateToken)
	g.PUT("/token/:tokenID/", rs.updateToken)
	g.DELETE("/token/:tokenID", rs.deleteToken)
	g.DELETE("/token/:tokenID/", rs.deleteToken)
}

func (rs *AccountResource) accountCtx(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		claims := jwt.ClaimsFromCtx(c)
		log(c).WithField("account_id", claims.ID)
		account, err := rs.Store.Get(claims.ID)
		if err != nil {
			// account deleted while access token still valid
			return ErrUnauthorized
		}
		c.Set(ctxAccount, account)
		return next(c)
	}
}

type accountRequest struct {
	*pwdless.Account
	// override protected data here, although not really necessary here
	// as we limit updated database columns in store as well
	ProtectedID     int      `json:"id"`
	ProtectedActive bool     `json:"active"`
	ProtectedRoles  []string `json:"roles"`
}

func (d *accountRequest) Bind(r *http.Request) error {
	// d.ProtectedActive = true
	// d.ProtectedRoles = []string{}
	return nil
}

type accountResponse struct {
	*pwdless.Account
}

func newAccountResponse(a *pwdless.Account) *accountResponse {
	resp := &accountResponse{Account: a}
	return resp
}

func (rs *AccountResource) get(c echo.Context) error {
	acc := c.Get(ctxAccount).(*pwdless.Account)
	return c.JSON(http.StatusOK, newAccountResponse(acc))
}

func (rs *AccountResource) update(c echo.Context) error {
	acc := c.Get(ctxAccount).(*pwdless.Account)
	data := &accountRequest{Account: acc}
	if err := bind(c, data); err != nil {
		return ErrInvalidRequest(err)
	}

	if err := rs.Store.Update(acc); err != nil {
		switch err := err.(type) {
		case validation.Errors:
			return ErrValidation(ErrAccountValidation, err)
		}
		return ErrRender(err)
	}

	return c.JSON(http.StatusOK, newAccountResponse(acc))
}

func (rs *AccountResource) delete(c echo.Context) error {
	acc := c.Get(ctxAccount).(*pwdless.Account)
	if err := rs.Store.Delete(acc); err != nil {
		return ErrRender(err)
	}
	return c.JSON(http.StatusOK, struct{}{})
}

type tokenRequest struct {
	Identifier  string
	ProtectedID int `json:"id"`
}

func (d *tokenRequest) Bind(r *http.Request) error {
	d.Identifier = strings.TrimSpace(d.Identifier)
	return nil
}

func (rs *AccountResource) updateToken(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("tokenID"))
	if err != nil {
		return ErrBadRequest
	}
	data := &tokenRequest{}
	if err := bind(c, data); err != nil {
		return ErrInvalidRequest(err)
	}
	acc := c.Get(ctxAccount).(*pwdless.Account)
	for _, t := range acc.Token {
		if t.ID == id {
			if err := rs.Store.UpdateToken(&jwt.Token{
				ID:         t.ID,
				Identifier: data.Identifier,
			}); err != nil {
				return ErrInvalidRequest(err)
			}
		}
	}
	return c.JSON(http.StatusOK, struct{}{})
}

func (rs *AccountResource) deleteToken(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("tokenID"))
	if err != nil {
		return ErrBadRequest
	}
	acc := c.Get(ctxAccount).(*pwdless.Account)
	for _, t := range acc.Token {
		if t.ID == id {
			rs.Store.DeleteToken(&jwt.Token{ID: t.ID})
		}
	}
	return c.JSON(http.StatusOK, struct{}{})
}
