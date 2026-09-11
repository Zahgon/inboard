package jwt

import (
	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/dhax/go-base/logging"
)

// Context keys under which the parsed application claims and refresh token are
// stored on the echo context.
const (
	ContextKeyClaims       = "app_claims"
	ContextKeyRefreshToken = "app_refresh_token"
)

// ClaimsFromCtx retrieves the parsed AppClaims from request context.
func ClaimsFromCtx(c echo.Context) AppClaims {
	return c.Get(ContextKeyClaims).(AppClaims)
}

// RefreshTokenFromCtx retrieves the parsed refresh token from context.
func RefreshTokenFromCtx(c echo.Context) string {
	return c.Get(ContextKeyRefreshToken).(string)
}

// Authenticator is a default authentication middleware to enforce access from the
// Verifier middleware request context values. The Authenticator sends a 401 Unauthorized
// response for any unverified tokens and passes the good ones through.
func Authenticator(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		token, claims, err := TokenFromCtx(c)

		if err != nil {
			logging.GetLogEntry(c).Warn(err)
			return ErrUnauthorized(ErrTokenUnauthorized)
		}

		if err := jwt.Validate(token); err != nil {
			return ErrUnauthorized(ErrTokenExpired)
		}

		// Token is authenticated, parse claims
		var ac AppClaims
		if err := ac.ParseClaims(claims); err != nil {
			logging.GetLogEntry(c).Error(err)
			return ErrUnauthorized(ErrInvalidAccessToken)
		}

		// Set AppClaims on context
		c.Set(ContextKeyClaims, ac)
		return next(c)
	}
}

// AuthenticateRefreshJWT checks validity of refresh tokens and is only used for access token refresh and logout requests. It responds with 401 Unauthorized for invalid or expired refresh tokens.
func AuthenticateRefreshJWT(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		token, claims, err := TokenFromCtx(c)
		if err != nil {
			logging.GetLogEntry(c).Warn(err)
			return ErrUnauthorized(ErrTokenUnauthorized)
		}

		if err := jwt.Validate(token); err != nil {
			return ErrUnauthorized(ErrTokenExpired)
		}

		// Token is authenticated, parse refresh token string
		var rc RefreshClaims
		if err := rc.ParseClaims(claims); err != nil {
			logging.GetLogEntry(c).Error(err)
			return ErrUnauthorized(ErrInvalidRefreshToken)
		}

		// Set refresh token string on context
		c.Set(ContextKeyRefreshToken, rc.Token)
		return next(c)
	}
}
