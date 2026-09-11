// Package admin ties together administration resources and handlers.
package admin

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"

	"github.com/dhax/go-base/auth/authorize"
	"github.com/dhax/go-base/database"
	"github.com/dhax/go-base/logging"
)

const (
	roleAdmin = "admin"
)

// ctxAccount is the context key under which the request scoped account is stored.
const ctxAccount = "admin_account"

// API provides admin application resources and handlers.
type API struct {
	Accounts *AccountResource
}

// NewAPI configures and returns admin application API.
func NewAPI(db *bun.DB) (*API, error) {
	accountStore := database.NewAdmAccountStore(db)
	accounts := NewAccountResource(accountStore)

	api := &API{
		Accounts: accounts,
	}
	return api, nil
}

// RegisterRoutes registers the admin application routes on the provided echo group.
func (a *API) RegisterRoutes(g *echo.Group) {
	g.Use(authorize.RequiresRole(roleAdmin))

	// both the bare and the trailing slash form are registered to preserve the
	// paths chi's Mount used to answer on.
	g.GET("", a.index)
	g.GET("/", a.index)

	a.Accounts.registerRoutes(g)
}

func (a *API) index(c echo.Context) error {
	log(c).Debug("admin access")
	return c.String(http.StatusOK, "Hello Admin")
}

func log(c echo.Context) logrus.FieldLogger {
	return logging.GetLogEntry(c)
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
