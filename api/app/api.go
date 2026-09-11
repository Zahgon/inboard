// Package app ties together application resources and handlers.
package app

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"

	"github.com/dhax/go-base/database"
	"github.com/dhax/go-base/logging"
)

// Context keys under which the request scoped account and profile are stored.
const (
	ctxAccount = "app_account"
	ctxProfile = "app_profile"
)

// API provides application resources and handlers.
type API struct {
	Account *AccountResource
	Profile *ProfileResource
}

// NewAPI configures and returns application API.
func NewAPI(db *bun.DB) (*API, error) {
	accountStore := database.NewAccountStore(db)
	account := NewAccountResource(accountStore)

	profileStore := database.NewProfileStore(db)
	profile := NewProfileResource(profileStore)

	api := &API{
		Account: account,
		Profile: profile,
	}
	return api, nil
}

// RegisterRoutes registers the application routes on the provided echo group.
func (a *API) RegisterRoutes(g *echo.Group) {
	a.Account.registerRoutes(g)
	a.Profile.registerRoutes(g)
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
