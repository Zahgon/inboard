package authorize

import (
	"slices"

	"github.com/labstack/echo/v4"

	"github.com/dhax/go-base/auth/jwt"
)

// RequiresRole middleware restricts access to accounts having role parameter in their jwt claims.
func RequiresRole(role string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims := jwt.ClaimsFromCtx(c)
			if !hasRole(role, claims.Roles) {
				return ErrForbidden
			}
			return next(c)
		}
	}
}

func hasRole(role string, roles []string) bool {
	return slices.Contains(roles, role)
}
