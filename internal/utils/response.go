package utils

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

// ErrorResponse is the standard JSON shape for all error replies.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// WriteJSON writes v as a JSON response body with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}

// WriteError writes a standardized error envelope.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    code,
	})
}

// DecodeJSON reads and strictly decodes a JSON request body into dst,
// rejecting unknown fields and bodies that contain more than one JSON value.
//
//	func DecodeJSON(r *http.Request, dst interface{}) error {
//		dec := json.NewDecoder(r.Body)
//		dec.DisallowUnknownFields()
//		if err := dec.Decode(dst); err != nil {
//			return err
//		}
//		return nil
//	}
const maxJSONBody = 1 << 20 // 1 MiB

// DecodeJSON reads a single JSON object from the request body into dst.
// It enforces a size cap, rejects unknown fields (catches client typos
// instead of silently dropping them), and rejects bodies with extra data
// after the JSON value.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func SetCookie(w http.ResponseWriter, cookie *http.Cookie) {
	w.Header().Set("Set-Cookie", cookie.String())
} // SetCookie

func setCookie(w http.ResponseWriter, name, value string) {
	cookie := &http.Cookie{
		Name:  name,
		Value: value,
		Path:  "/",
	}
	SetCookie(w, cookie)
}

func ReadCookie(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// ReadJSON decodes a single JSON object from the request body into dst.
//
// On success it returns true. On failure it writes the appropriate error
// response (400 or 413) and returns false — the handler should simply
// `return` in that case:
//
//	var req models.LoginRequest
//	if !ReadJSON(w, r, &req) {
//	    return
//	}
//
// It enforces a size cap, rejects unknown fields (catches client typos
// instead of silently dropping them), and rejects trailing data after the
// JSON value.
func ReadJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			WriteError(w, http.StatusRequestEntityTooLarge,
				"payload_too_large", "request body too large")
			return false
		}
		WriteError(w, http.StatusBadRequest, "bad_request", jsonErrDetail(err))
		return false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		WriteError(w, http.StatusBadRequest,
			"bad_request", "request body must contain a single JSON object")
		return false
	}
	return true
}

// jsonErrDetail maps decoder errors to safe, client-friendly messages.
// The raw error string is never leaked: it can expose Go internals.
func jsonErrDetail(err error) string {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntaxErr):
		return "malformed JSON"
	case errors.As(err, &typeErr):
		return "field '" + typeErr.Field + "' has the wrong type"
	case errors.Is(err, io.EOF):
		return "request body is empty"
	default:
		return "invalid request body"
	}
}
