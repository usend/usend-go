package usend

import (
	"fmt"
	"time"
)

// Error is an error returned by the usend API. It carries the API's
// { statusCode, name, message } shape, for example
// {422, "validation_error", "The `to` field is required."}.
//
// Network failures and cancelled contexts are returned as the underlying
// error (wrapped), not as *Error. Use errors.As to inspect:
//
//	var apiErr *usend.Error
//	if errors.As(err, &apiErr) && apiErr.StatusCode == 429 { ... }
type Error struct {
	StatusCode int    `json:"statusCode"`
	Name       string `json:"name"`
	Message    string `json:"message"`

	// RetryAfter is the delay the server asked for (Retry-After header), if any.
	RetryAfter time.Duration `json:"-"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("usend: %s (HTTP %d): %s", e.Name, e.StatusCode, e.Message)
}
