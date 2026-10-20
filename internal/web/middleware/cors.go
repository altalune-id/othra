package middleware

import "net/http"

// PublicCORS lets any origin read a public, cookie-less resource with GET or HEAD; a preflight answers 204 without reaching next.
func PublicCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Expose-Headers", "ETag")
		if r.Method == http.MethodOptions {
			h.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type, If-None-Match")
			h.Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
