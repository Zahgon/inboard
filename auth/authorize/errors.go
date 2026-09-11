package authorize

import (
	"net/http"
)

// ErrResponse is the error response type returned to the client. It implements
// error so middleware can return it directly to echo.
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

// The list of default error types without specific error message.
var (
	ErrForbidden = &ErrResponse{
		HTTPStatusCode: http.StatusForbidden,
		StatusText:     http.StatusText(http.StatusForbidden),
	}
)
