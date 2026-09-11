package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/dhax/go-base/auth/pwdless"
	"github.com/dhax/go-base/database"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/labstack/echo/v4"
)

// The list of error types returned from account resource.
var (
	ErrAccountValidation = errors.New("account validation error")
)

// AccountStore defines database operations for account management.
type AccountStore interface {
	List(*database.AccountFilter) ([]pwdless.Account, int, error)
	Create(*pwdless.Account) error
	Get(id int) (*pwdless.Account, error)
	Update(*pwdless.Account) error
	Delete(*pwdless.Account) error
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
	g := parent.Group("/accounts")

	// both the bare and the trailing slash form are registered to preserve the
	// paths chi's Mount used to answer on.
	g.GET("", rs.list)
	g.GET("/", rs.list)
	g.POST("", rs.create)
	g.POST("/", rs.create)

	item := g.Group("/:accountID", rs.accountCtx)
	item.GET("", rs.get)
	item.GET("/", rs.get)
	item.PUT("", rs.update)
	item.PUT("/", rs.update)
	item.DELETE("", rs.delete)
	item.DELETE("/", rs.delete)
}

func (rs *AccountResource) accountCtx(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("accountID"))
		if err != nil {
			return ErrBadRequest
		}
		account, err := rs.Store.Get(id)
		if err != nil {
			return ErrNotFound
		}
		c.Set(ctxAccount, account)
		return next(c)
	}
}

type accountRequest struct {
	*pwdless.Account
}

func (d *accountRequest) Bind(r *http.Request) error {
	return nil
}

type accountResponse struct {
	*pwdless.Account
}

func newAccountResponse(a *pwdless.Account) *accountResponse {
	resp := &accountResponse{Account: a}
	return resp
}

type accountListResponse struct {
	Accounts *[]pwdless.Account `json:"accounts"`
	Count    int                `json:"count"`
}

func newAccountListResponse(a *[]pwdless.Account, count int) *accountListResponse {
	resp := &accountListResponse{
		Accounts: a,
		Count:    count,
	}
	return resp
}

func (rs *AccountResource) list(c echo.Context) error {
	f, err := database.NewAccountFilter(c.Request().URL.Query())
	if err != nil {
		return ErrRender(err)
	}
	al, count, err := rs.Store.List(f)
	if err != nil {
		return ErrRender(err)
	}
	return c.JSON(http.StatusOK, newAccountListResponse(&al, count))
}

func (rs *AccountResource) create(c echo.Context) error {
	data := &accountRequest{}
	if err := bind(c, data); err != nil {
		return ErrInvalidRequest(err)
	}

	if err := rs.Store.Create(data.Account); err != nil {
		switch err := err.(type) {
		case validation.Errors:
			return ErrValidation(ErrAccountValidation, err)
		}
		return ErrInvalidRequest(err)
	}
	return c.JSON(http.StatusOK, newAccountResponse(data.Account))
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
		return ErrInvalidRequest(err)
	}

	return c.JSON(http.StatusOK, newAccountResponse(acc))
}

func (rs *AccountResource) delete(c echo.Context) error {
	acc := c.Get(ctxAccount).(*pwdless.Account)
	if err := rs.Store.Delete(acc); err != nil {
		return ErrInvalidRequest(err)
	}
	return c.JSON(http.StatusOK, struct{}{})
}
