package pwdless

import (
	"errors"
	"net/http"
)

// The list of error types presented to the end user as error message.
var (
	ErrInvalidLogin  = errors.New("invalid email address")
	ErrUnknownLogin  = errors.New("email not registered")
	ErrLoginDisabled = errors.New("login for account disabled")
	ErrLoginToken    = errors.New("invalid or expired login token")
)

// ErrResponse is the error response type returned to the client. It implements
// error so handlers can return it directly to echo.
type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"status"`          // user-level status message
	AppCode    int64  `json:"code,omitempty"`  // application-specific error code
	ErrorText  string `json:"error,omitempty"` // application-level error message, for debugging
}

// Error implements the error interface.
func (e *ErrResponse) Error() string {
	if e.ErrorText != "" {
		return e.ErrorText
	}
	return e.StatusText
}

// Status returns the http response status code of the error response.
func (e *ErrResponse) Status() int {
	return e.HTTPStatusCode
}

// ErrUnauthorized renders status 401 Unauthorized with custom error message.
func ErrUnauthorized(err error) error {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     http.StatusText(http.StatusUnauthorized),
		ErrorText:      err.Error(),
	}
}

// The list of default error types without specific error message.
var (
	ErrInternalServerError = &ErrResponse{
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     http.StatusText(http.StatusInternalServerError),
	}
)
