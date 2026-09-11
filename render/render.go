// Package render provides the shared JSON error rendering for the api. It
// replaces the response half of go-chi/render, which the echo migration removed.
package render

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/dhax/go-base/logging"
)

// StatusError is implemented by the application error responses to report the
// http status code they should be rendered with.
type StatusError interface {
	Status() int
}

// ErrorHandler renders errors returned by handlers and middleware as JSON,
// preserving the status codes and body shapes of the application error
// responses. It is registered as echo's HTTPErrorHandler.
func ErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	var (
		status = http.StatusInternalServerError
		body   any
	)

	switch e := err.(type) {
	case StatusError:
		status = e.Status()
		body = err
	case *echo.HTTPError:
		status = e.Code
		body = map[string]any{"status": http.StatusText(e.Code)}
	default:
		body = map[string]any{"status": http.StatusText(http.StatusInternalServerError)}
	}

	if c.Request().Method == http.MethodHead {
		_ = c.NoContent(status)
		return
	}
	if err := c.JSON(status, body); err != nil {
		logging.GetLogEntry(c).Error(err)
	}
}
