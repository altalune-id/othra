package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/a-h/templ"
)

// SecurityHeaders sets anti-framing, no-sniff and referrer headers; a handler may still override them.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "frame-ancestors 'none'")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// NoReferrer sends no Referer from the page, so a token in its URL never leaves the browser.
func NoReferrer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// NoStore forbids every cache from keeping the response; pre-sign-in pages carry tokens and one-time verdicts.
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// HostedCSP sets the hosted pages' strict policy: this origin, imageBases' origins and a per-response style nonce on the context.
func HostedCSP(imageBases ...string) func(http.Handler) http.Handler {
	sources := []string{"'self'", "data:"}
	for _, base := range imageBases {
		if o := origin(base); o != "" && !slices.Contains(sources, o) {
			sources = append(sources, o)
		}
	}
	images := strings.Join(sources, " ")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			nonce := base64.RawStdEncoding.EncodeToString(b)
			w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src "+images+"; style-src 'self' 'nonce-"+nonce+
				"'; form-action 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'")
			next.ServeHTTP(w, r.WithContext(templ.WithNonce(r.Context(), nonce)))
		})
	}
}

func origin(base string) string {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || strings.ContainsAny(u.Host, " ;'") {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// AllowNonceScripts lets a page under HostedCSP run the inline scripts that carry nonce, the response's style nonce.
func AllowNonceScripts(w http.ResponseWriter, nonce string) {
	h := w.Header()
	policy := h.Get("Content-Security-Policy")
	if policy == "" || nonce == "" {
		return
	}
	scripts := "default-src 'self'; script-src 'self' 'nonce-" + nonce + "';"
	h.Set("Content-Security-Policy", strings.Replace(policy, "default-src 'self';", scripts, 1))
}
