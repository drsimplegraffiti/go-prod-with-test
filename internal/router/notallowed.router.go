package router

import "net/http"

// methodNotAllowedRecorder intercepts the response so that when the
// wrapped handler (ultimately http.ServeMux) writes a 405, we can swap its
// default plain-text body for a JSON one — while leaving every other
// status code, and any headers already set (like ServeMux's own "Allow"
// header), untouched.
type methodNotAllowedRecorder struct {
	http.ResponseWriter
	status int
}

func (r *methodNotAllowedRecorder) WriteHeader(code int) {
	r.status = code
	if code == http.StatusMethodNotAllowed {
		r.ResponseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *methodNotAllowedRecorder) Write(b []byte) (int, error) {
	if r.status == http.StatusMethodNotAllowed {
		// Swallow ServeMux's own "405 method not allowed" plain-text body;
		// MethodNotAllowedJSON writes the real body once the handler returns.
		return len(b), nil
	}
	return r.ResponseWriter.Write(b)
}

// MethodNotAllowedJSON wraps a handler (typically http.ServeMux directly)
// so that a path matching a registered route, but with an unsupported
// method, returns a JSON body instead of net/http's default plain text —
// consistent with every other error response in this API.
//
// This only affects the 405 case. A genuinely unmatched path is unaffected
// and continues to reach the catch-all "/" 404 handler as normal, since
// ServeMux only returns 405 when the path itself matched a route but the
// method didn't.
func MethodNotAllowedJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &methodNotAllowedRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		if rec.status == http.StatusMethodNotAllowed {
			w.Write([]byte(`{"error":"Method Not Allowed","message":"the HTTP method is not allowed for this resource"}`))
		}
	})
}
