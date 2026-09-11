package app

import (
	"errors"
	"net/http"

	"github.com/dhax/go-base/auth/jwt"
	"github.com/dhax/go-base/models"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/labstack/echo/v4"
)

// The list of error types returned from account resource.
var (
	ErrProfileValidation = errors.New("profile validation error")
)

// ProfileStore defines database operations for a profile.
type ProfileStore interface {
	Get(accountID int) (*models.Profile, error)
	Update(p *models.Profile) error
}

// ProfileResource implements profile management handler.
type ProfileResource struct {
	Store ProfileStore
}

// NewProfileResource creates and returns a profile resource.
func NewProfileResource(store ProfileStore) *ProfileResource {
	return &ProfileResource{
		Store: store,
	}
}

func (rs *ProfileResource) registerRoutes(parent *echo.Group) {
	g := parent.Group("/profile", rs.profileCtx)

	// both the bare and the trailing slash form are registered to preserve the
	// paths chi's Mount used to answer on.
	g.GET("", rs.get)
	g.GET("/", rs.get)
	g.PUT("", rs.update)
	g.PUT("/", rs.update)
}

func (rs *ProfileResource) profileCtx(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		claims := jwt.ClaimsFromCtx(c)
		p, err := rs.Store.Get(claims.ID)
		if err != nil {
			log(c).WithField("profileCtx", claims.Sub).Error(err)
			return ErrInternalServerError
		}
		c.Set(ctxProfile, p)
		return next(c)
	}
}

type profileRequest struct {
	*models.Profile
	ProtectedID int `json:"id"`
}

func (d *profileRequest) Bind(r *http.Request) error {
	return nil
}

type profileResponse struct {
	*models.Profile
}

func newProfileResponse(p *models.Profile) *profileResponse {
	return &profileResponse{
		Profile: p,
	}
}

func (rs *ProfileResource) get(c echo.Context) error {
	p := c.Get(ctxProfile).(*models.Profile)
	return c.JSON(http.StatusOK, newProfileResponse(p))
}

func (rs *ProfileResource) update(c echo.Context) error {
	p := c.Get(ctxProfile).(*models.Profile)
	data := &profileRequest{Profile: p}
	if err := bind(c, data); err != nil {
		// NOTE: the original chi implementation does not return here; the
		// fall-through is preserved verbatim rather than silently fixed.
		_ = c.JSON(http.StatusUnprocessableEntity, ErrInvalidRequest(err))
	}

	if err := rs.Store.Update(p); err != nil {
		switch err := err.(type) {
		case validation.Errors:
			return ErrValidation(ErrProfileValidation, err)
		}
		return ErrRender(err)
	}
	return c.JSON(http.StatusOK, newProfileResponse(p))
}
