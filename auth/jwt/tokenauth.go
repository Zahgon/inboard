package jwt

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/spf13/viper"
)

// Context keys under which the verified token, its claims and the verification
// error are stored on the echo context by Verifier.
const (
	ContextKeyToken       = "jwt_token"
	ContextKeyTokenClaims = "jwt_token_claims"
	ContextKeyTokenError  = "jwt_token_error"
)

// TokenAuth implements JWT authentication flow.
type TokenAuth struct {
	JwtSecret        []byte
	JwtExpiry        time.Duration
	JwtRefreshExpiry time.Duration
}

// NewTokenAuth configures and returns a JWT authentication instance.
func NewTokenAuth() (*TokenAuth, error) {
	secret := viper.GetString("auth_jwt_secret")
	knownWeakSecrets := map[string]bool{
		"":                true,
		"random":          true,
		"CHANGE-ME":       true,
		"secret":          true,
		"changeme":        true,
		"jwt_secret":      true,
		"your-secret-key": true,
	}
	if knownWeakSecrets[secret] {
		return nil, ErrWeakSecret
	}
	if len(secret) < 32 {
		return nil, ErrSecretTooShort
	}

	a := &TokenAuth{
		JwtSecret:        []byte(secret),
		JwtExpiry:        viper.GetDuration("auth_jwt_expiry"),
		JwtRefreshExpiry: viper.GetDuration("auth_jwt_refresh_expiry"),
	}

	return a, nil
}

// Verifier echo middleware verifies a jwt string from a http request and stores
// the resulting token, claims and error on the request context. It never aborts
// the request itself - Authenticator and AuthenticateRefreshJWT decide that.
func (a *TokenAuth) Verifier() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token, err := a.VerifyToken(tokenFromRequest(c.Request()))

			var claims map[string]any
			if token != nil {
				claims, _ = token.AsMap(context.Background())
			}

			c.Set(ContextKeyToken, token)
			c.Set(ContextKeyTokenClaims, claims)
			c.Set(ContextKeyTokenError, err)

			return next(c)
		}
	}
}

// VerifyToken parses and verifies the signature and claims of a jwt string.
func (a *TokenAuth) VerifyToken(tokenString string) (jwt.Token, error) {
	if tokenString == "" {
		return nil, ErrTokenUnauthorized
	}

	token, err := jwt.Parse([]byte(tokenString), jwt.WithKey(jwa.HS256, a.JwtSecret), jwt.WithValidate(false))
	if err != nil {
		return nil, err
	}
	if err := jwt.Validate(token); err != nil {
		return token, err
	}
	return token, nil
}

// TokenFromCtx retrieves the verified token, its claims and the verification
// error from the request context.
func TokenFromCtx(c echo.Context) (jwt.Token, map[string]any, error) {
	token, _ := c.Get(ContextKeyToken).(jwt.Token)
	claims, _ := c.Get(ContextKeyTokenClaims).(map[string]any)
	err, _ := c.Get(ContextKeyTokenError).(error)
	return token, claims, err
}

// tokenFromRequest extracts a jwt string from the Authorization header or, if
// absent, from the "jwt" cookie.
func tokenFromRequest(r *http.Request) string {
	bearer := r.Header.Get("Authorization")
	if len(bearer) > 7 && strings.ToUpper(bearer[0:6]) == "BEARER" {
		return bearer[7:]
	}
	if cookie, err := r.Cookie("jwt"); err == nil {
		return cookie.Value
	}
	return ""
}

// GenTokenPair returns both an access token and a refresh token.
func (a *TokenAuth) GenTokenPair(accessClaims AppClaims, refreshClaims RefreshClaims) (string, string, error) {
	access, err := a.CreateJWT(accessClaims)
	if err != nil {
		return "", "", err
	}
	refresh, err := a.CreateRefreshJWT(refreshClaims)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// CreateJWT returns an access token for provided account claims.
func (a *TokenAuth) CreateJWT(c AppClaims) (string, error) {
	c.IssuedAt = time.Now().Unix()
	c.ExpiresAt = time.Now().Add(a.JwtExpiry).Unix()

	claims, err := ParseStructToMap(c)
	if err != nil {
		return "", err
	}

	return a.Encode(claims)
}

// Encode signs the provided claims and returns the serialized jwt string.
func (a *TokenAuth) Encode(claims map[string]any) (string, error) {
	b := jwt.NewBuilder()
	for k, v := range claims {
		b = b.Claim(k, v)
	}
	token, err := b.Build()
	if err != nil {
		return "", err
	}

	signed, err := jwt.Sign(token, jwt.WithKey(jwa.HS256, a.JwtSecret))
	if err != nil {
		return "", err
	}
	return string(signed), nil
}

func ParseStructToMap(c any) (map[string]any, error) {
	var claims map[string]any
	inrec, _ := json.Marshal(c)
	err := json.Unmarshal(inrec, &claims)
	if err != nil {
		return nil, err
	}

	return claims, err
}

// CreateRefreshJWT returns a refresh token for provided token Claims.
func (a *TokenAuth) CreateRefreshJWT(c RefreshClaims) (string, error) {
	c.IssuedAt = time.Now().Unix()
	c.ExpiresAt = time.Now().Add(a.JwtRefreshExpiry).Unix()

	claims, err := ParseStructToMap(c)
	if err != nil {
		return "", err
	}

	return a.Encode(claims)
}
