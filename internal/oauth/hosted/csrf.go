// Package hosted serves the branded sign-in pages of each project's issuer: login, magic links, the account picker and terms.
package hosted

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"time"

	"altalune.id/othra/internal/web/layout"
)

// CSRFCookie is the cookie a hosted form token is bound to; layout.CSRFField carries the token.
const CSRFCookie = "__Secure-othra_csrf"

const (
	csrfMACLen = sha256.Size
	csrfSkew   = time.Minute
)

// CSRF issues and checks hosted form tokens: issued_at ‖ HMAC(key, cookie ‖ scope ‖ form ‖ issued_at), with no server storage (D3a).
type CSRF struct {
	key    []byte
	maxAge time.Duration
	now    func() time.Time
}

// NewCSRF builds a CSRF whose tokens are signed with key and live for maxAge.
func NewCSRF(key []byte, maxAge time.Duration) *CSRF {
	return &CSRF{key: key, maxAge: maxAge, now: time.Now}
}

// Cookie returns the browser's CSRF cookie value, setting a new one at path when it has none.
func (c *CSRF) Cookie(w http.ResponseWriter, r *http.Request, path string) string {
	if got, err := r.Cookie(CSRFCookie); err == nil && got.Value != "" {
		return got.Value
	}
	value := c.Renew(w, path)
	r.AddCookie(&http.Cookie{Name: CSRFCookie, Value: value, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	return value
}

// Renew sets a new CSRF cookie at path, so tokens issued before it stop verifying; sign-in calls it.
func (c *CSRF) Renew(w http.ResponseWriter, path string) string {
	value := rand.Text()
	http.SetCookie(w, &http.Cookie{
		Name: CSRFCookie, Value: value, Path: path, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	return value
}

// Token returns the token for form within scope (the project), bound to cookie.
func (c *CSRF) Token(cookie, scope, form string) string {
	return c.token(cookie, scope, form, c.now().Unix())
}

func (c *CSRF) token(cookie, scope, form string, issued int64) string {
	buf := binary.BigEndian.AppendUint64(make([]byte, 0, 8+csrfMACLen), uint64(issued)) //nolint:gosec // unix seconds are positive.
	return encodeToken(append(buf, c.mac(cookie, scope, form, issued)...))
}

func encodeToken(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func decodeToken(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

// Check verifies the form token r carries for form within scope against r's CSRF cookie; it returns *FormExpiredError.
func (c *CSRF) Check(r *http.Request, scope, form string) error {
	cookie, err := r.Cookie(CSRFCookie)
	if err != nil || cookie.Value == "" {
		return &FormExpiredError{Reason: FormNoCookie}
	}
	raw := r.PostFormValue(layout.CSRFField)
	if raw == "" {
		return &FormExpiredError{Reason: FormNoToken}
	}
	b, err := decodeToken(raw)
	if err != nil || len(b) != 8+csrfMACLen {
		return &FormExpiredError{Reason: FormBadToken}
	}
	issued := int64(binary.BigEndian.Uint64(b[:8])) //nolint:gosec // a forged value fails the MAC first.
	if !hmac.Equal(b[8:], c.mac(cookie.Value, scope, form, issued)) {
		return &FormExpiredError{Reason: FormBadToken}
	}
	at, now := time.Unix(issued, 0), c.now()
	if at.After(now.Add(csrfSkew)) || now.Sub(at) > c.maxAge {
		return &FormExpiredError{Reason: FormExpired}
	}
	return nil
}

func (c *CSRF) mac(cookie, scope, form string, issued int64) []byte {
	h := hmac.New(sha256.New, c.key)
	for _, part := range []string{"othra-hosted-csrf", cookie, scope, form} {
		h.Write(binary.BigEndian.AppendUint32(nil, uint32(len(part)))) //nolint:gosec // parts are short strings.
		h.Write([]byte(part))
	}
	h.Write(binary.BigEndian.AppendUint64(nil, uint64(issued))) //nolint:gosec // the issued time as signed.
	return h.Sum(nil)
}
