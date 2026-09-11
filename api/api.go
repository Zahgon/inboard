// Package api configures an http server for administration and application resources.
package api

import (
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	emw "github.com/labstack/echo/v4/middleware"

	"github.com/dhax/go-base/api/admin"
	"github.com/dhax/go-base/api/app"
	"github.com/dhax/go-base/auth/jwt"
	"github.com/dhax/go-base/auth/pwdless"
	"github.com/dhax/go-base/database"
	"github.com/dhax/go-base/email"
	"github.com/dhax/go-base/logging"
	"github.com/dhax/go-base/render"
)

// New configures application resources and routes.
func New(enableCORS bool) (*echo.Echo, error) {
	logger := logging.NewLogger()

	db, err := database.DBConn()
	if err != nil {
		logger.WithField("module", "database").Error(err)
		return nil, err
	}

	mailer, err := email.NewMailer()
	if err != nil {
		logger.WithField("module", "email").Error(err)
		return nil, err
	}

	authStore := database.NewAuthStore(db)
	authResource, err := pwdless.NewResource(authStore, mailer)
	if err != nil {
		logger.WithField("module", "auth").Error(err)
		return nil, err
	}

	adminAPI, err := admin.NewAPI(db)
	if err != nil {
		logger.WithField("module", "admin").Error(err)
		return nil, err
	}

	appAPI, err := app.NewAPI(db)
	if err != nil {
		logger.WithField("module", "app").Error(err)
		return nil, err
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = render.ErrorHandler

	e.Use(emw.Recover())
	e.Use(emw.RequestID())
	// e.Use(emw.ExtractIP...)
	e.Use(emw.ContextTimeout(15 * time.Second))

	e.Use(logging.NewStructuredLogger(logger))

	// use CORS middleware if client is not served by this api, e.g. from other domain or CDN
	if enableCORS {
		e.Use(emw.CORSWithConfig(corsConfig()))
	}

	authResource.RegisterRoutes(e.Group("/auth"))

	authMiddleware := []echo.MiddlewareFunc{
		authResource.TokenAuth.Verifier(),
		jwt.Authenticator,
	}
	adminAPI.RegisterRoutes(e.Group("/admin", authMiddleware...))
	appAPI.RegisterRoutes(e.Group("/api", authMiddleware...))

	e.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	e.GET("/*", SPAHandler("public"))

	return e, nil
}

func corsConfig() emw.CORSConfig {
	// Basic CORS
	// for more ideas, see: https://developer.github.com/v3/#cross-origin-resource-sharing
	return emw.CORSConfig{
		// AllowOrigins: []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposeHeaders:    []string{"Link"},
		AllowCredentials: true,
		MaxAge:           86400, // Maximum value not ignored by any of major browsers
	}
}

// SPAHandler serves the public Single Page Application.
func SPAHandler(publicDir string) echo.HandlerFunc {
	return func(c echo.Context) error {
		indexPage := path.Join(publicDir, "index.html")
		serviceWorker := path.Join(publicDir, "service-worker.js")

		requestedAsset := path.Join(publicDir, c.Request().URL.Path)
		if strings.Contains(requestedAsset, "service-worker.js") {
			requestedAsset = serviceWorker
		}
		if info, err := os.Stat(requestedAsset); err != nil || info.IsDir() {
			return c.File(indexPage)
		}
		return c.File(requestedAsset)
	}
}
