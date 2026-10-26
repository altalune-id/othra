// Package oidcrptest runs a fake upstream IdP for tests of oidcrp relying parties.
package oidcrptest

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
)

// User is who the fake upstream IdP signs in.
type User struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// IDP is a fake upstream IdP: discovery, JWKS, and a token endpoint that redeems the codes Authorize hands out.
type IDP struct {
	srv *httptest.Server
	key *rsa.PrivateKey
	t   testing.TB

	mu    sync.Mutex
	codes map[string]grant
	// User is who the next Authorize signs in.
	User User
	// Claims are merged over the ID token's claims, so a test can break one.
	Claims map[string]any
	// TokenStatus, when set, is the token endpoint's error status.
	TokenStatus int
	// Now is the upstream IdP's clock.
	Now func() time.Time
}

type grant struct {
	nonce    string
	clientID string
	user     User
}

// New starts a fake upstream IdP that closes with t.
func New(t testing.TB) *IDP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &IDP{
		key: key, t: t, codes: map[string]grant{}, Now: time.Now,
		User: User{Subject: "110169484474386276334", Email: "ana@gmail.com", EmailVerified: true, Name: "Ana"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", f.discovery)
	mux.HandleFunc("GET /jwks", f.jwks)
	mux.HandleFunc("POST /token", f.token)
	f.srv = httptest.NewTLSServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// Issuer is the upstream IdP's issuer URL.
func (f *IDP) Issuer() string { return f.srv.URL }

// Client is an HTTP client that trusts the upstream IdP's test certificate.
func (f *IDP) Client() *http.Client { return f.srv.Client() }

// Authorize plays the browser at the authorization URL: it signs User in and returns the redirect's query.
func (f *IDP) Authorize(authURL string) url.Values {
	f.t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		f.t.Fatal(err)
	}
	q := u.Query()
	code := rand.Text()
	f.mu.Lock()
	f.codes[code] = grant{nonce: q.Get("nonce"), clientID: q.Get("client_id"), user: f.User}
	f.mu.Unlock()
	return url.Values{"code": {code}, "state": {q.Get("state")}, "iss": {f.Issuer()}}
}

// Deny returns the redirect's query of a person who cancelled at the upstream IdP.
func (f *IDP) Deny(authURL string) url.Values {
	u, _ := url.Parse(authURL)
	return url.Values{"error": {"access_denied"}, "state": {u.Query().Get("state")}, "iss": {f.Issuer()}}
}

func (f *IDP) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer": f.Issuer(), "authorization_endpoint": f.Issuer() + "/authorize", "token_endpoint": f.Issuer() + "/token",
		"jwks_uri": f.Issuer() + "/jwks", "code_challenge_methods_supported": []string{"S256"},
		"authorization_response_iss_parameter_supported": true, "id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (f *IDP) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &f.key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
}

func (f *IDP) token(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.codes[r.PostFormValue("code")]
	delete(f.codes, r.PostFormValue("code"))
	if f.TokenStatus != 0 || !ok || r.PostFormValue("code_verifier") == "" {
		w.WriteHeader(max(f.TokenStatus, http.StatusBadRequest))
		writeJSON(w, map[string]string{"error": "invalid_grant"})
		return
	}
	now := f.Now()
	claims := map[string]any{
		"iss": f.Issuer(), "sub": g.user.Subject, "aud": g.clientID, "azp": g.clientID, "exp": now.Add(time.Hour).Unix(),
		"iat": now.Unix(), "nonce": g.nonce, "email": g.user.Email, "email_verified": g.user.EmailVerified, "name": g.user.Name,
	}
	maps.Copy(claims, f.Claims)
	writeJSON(w, map[string]any{"access_token": "upstream-at", "token_type": "Bearer", "id_token": f.sign(claims)})
}

func (f *IDP) sign(claims map[string]any) string {
	opts := (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1")
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: f.key}, opts)
	if err != nil {
		f.t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		f.t.Fatal(err)
	}
	obj, err := signer.Sign(payload)
	if err != nil {
		f.t.Fatal(err)
	}
	out, err := obj.CompactSerialize()
	if err != nil {
		f.t.Fatal(err)
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
