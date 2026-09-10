package service

import "errors"

// ErrValidation wraps client input errors. Handlers map errors.Is(err,
// ErrValidation) to HTTP 400, distinguishing it from unexpected internal
// failures which map to 500.
var ErrValidation = errors.New("validation error")

// ErrForbidden indicates the caller is authenticated but not permitted to
// perform the requested action (e.g. editing another user's post).
var ErrForbidden = errors.New("forbidden")

// ErrNotFound indicates the requested resource does not exist.
var ErrNotFound = errors.New("not found")
