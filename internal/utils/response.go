package utils

import (
	"encoding/json"
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
func DecodeJSON(r *http.Request, dst interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
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
