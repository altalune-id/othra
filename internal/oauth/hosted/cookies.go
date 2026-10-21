package hosted

import (
	"net/http"
	"strings"
	"time"

	"altalune.id/othra/internal/kernel/session"
)

// The hosted cookies; every one is Secure, HttpOnly, SameSite=Lax and scoped to one issuer's path.
const (
	BrowserCookie = "__Secure-othra_eu"
	linkCookie    = "__Secure-othra_ml"
	sentCookie    = "__Secure-othra_sent"
	noticeCookie  = "__Secure-othra_notice"
	flashTTL      = 15 * time.Minute
)

type cookies struct {
	key []byte
	now func() time.Time
}

// SECURITY: H4: the browser secret is signed, so a forged value never reaches the store; the store keeps only its hash.
func (c cookies) setBrowser(w http.ResponseWriter, path, secret string, expires time.Time) {
	maxAge := max(int(expires.Sub(c.now()).Seconds()), 1)
	http.SetCookie(w, cookie(BrowserCookie, path, session.Sign(c.key, BrowserCookie+"="+secret), maxAge))
}

func (c cookies) browser(r *http.Request) string {
	raw, err := r.Cookie(BrowserCookie)
	if err != nil {
		return ""
	}
	signed, err := session.Verify(c.key, raw.Value)
	if err != nil {
		return ""
	}
	secret, _ := strings.CutPrefix(signed, BrowserCookie+"=")
	return secret
}

// SECURITY: H3: the link token leaves the address bar for a cookie scoped to the verify page.
func (c cookies) setLink(w http.ResponseWriter, path, token string, ttl time.Duration) {
	http.SetCookie(w, cookie(linkCookie, path, token, int(ttl.Seconds())))
}

func (c cookies) link(r *http.Request) string {
	raw, err := r.Cookie(linkCookie)
	if err != nil {
		return ""
	}
	return raw.Value
}

func (c cookies) clearLink(w http.ResponseWriter, path string) {
	http.SetCookie(w, cookie(linkCookie, path, "", -1))
}

func (c cookies) flash(name, path string) session.SignedCookie {
	return session.SignedCookie{Name: name, Path: path, Secret: c.key, Secure: true, Now: c.now}
}

func cookie(name, path, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name: name, Value: value, Path: path, MaxAge: maxAge, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	}
}
