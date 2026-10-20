package resourceserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/httpclient"
)

const testIssuer = "https://othra.example.com/p/acme"

type metadataServer struct {
	srv        *httptest.Server
	identifier string
	handler    http.HandlerFunc
	mu         sync.Mutex
	paths      []string
}

func newMetadataServer(t *testing.T) *metadataServer {
	t.Helper()
	m := &metadataServer{}
	m.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.paths = append(m.paths, r.URL.Path)
		m.mu.Unlock()
		m.handler(w, r)
	}))
	t.Cleanup(m.srv.Close)
	m.identifier = m.srv.URL + "/mcp"
	m.serve(map[string]any{"resource": m.identifier, "authorization_servers": []string{"https://other.example", testIssuer}})
	return m
}

func (m *metadataServer) fetched() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.paths
}

func (m *metadataServer) serve(doc any) {
	m.handler = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}
}

func (m *metadataServer) verifier(allowPrivateHosts bool) *Verifier {
	c := httpclient.New(append(clientOptions(2*time.Second, allowPrivateHosts), httpclient.WithOtel(false))...)
	pool := x509.NewCertPool()
	pool.AddCert(m.srv.Certificate())
	c.Transport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	return &Verifier{client: c, now: func() time.Time { return time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC) }}
}

func TestVerifier_Pass(t *testing.T) {
	m := newMetadataServer(t)
	assert.Nil(t, m.verifier(true).Check(context.Background(), m.identifier, testIssuer))
	assert.Equal(t, []string{"/.well-known/oauth-protected-resource/mcp"}, m.fetched())
}

func TestVerifier_Failures(t *testing.T) {
	big := strings.Repeat("x", MaxMetadataBytes)
	for name, tc := range map[string]struct {
		handler  func(m *metadataServer) http.HandlerFunc
		reason   FailureReason
		status   int32
		resource string
	}{
		"a wrong resource": {
			handler: func(m *metadataServer) http.HandlerFunc {
				m.serve(map[string]any{"resource": m.identifier + "/", "authorization_servers": []string{testIssuer}})
				return m.handler
			},
			reason: FailureResourceMismatch,
		},
		"a missing issuer": {
			handler: func(m *metadataServer) http.HandlerFunc {
				m.serve(map[string]any{"resource": m.identifier, "authorization_servers": []string{testIssuer + "/"}})
				return m.handler
			},
			reason: FailureIssuerMissing,
		},
		"a redirect": {
			handler: func(m *metadataServer) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/elsewhere", http.StatusFound) }
			},
			reason: FailureRedirect, status: http.StatusFound,
		},
		"an oversized body": {
			handler: func(m *metadataServer) http.HandlerFunc {
				m.serve(map[string]any{"resource": m.identifier, "authorization_servers": []string{testIssuer}, "pad": big})
				return m.handler
			},
			reason: FailureTooLarge,
		},
		"not found": {
			handler: func(*metadataServer) http.HandlerFunc { return http.NotFound },
			reason:  FailureNotFound, status: http.StatusNotFound,
		},
		"a server error": {
			handler: func(*metadataServer) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }
			},
			reason: FailureStatus, status: http.StatusServiceUnavailable,
		},
		"not an object": {
			handler: func(m *metadataServer) http.HandlerFunc { m.serve([]string{m.identifier}); return m.handler },
			reason:  FailureNotJSON,
		},
		"keys in another case": {
			handler: func(m *metadataServer) http.HandlerFunc {
				m.serve(map[string]any{"Resource": m.identifier, "Authorization_Servers": []string{testIssuer}})
				return m.handler
			},
			reason: FailureResourceMismatch,
		},
		"servers not a list": {
			handler: func(m *metadataServer) http.HandlerFunc {
				m.serve(map[string]any{"resource": m.identifier, "authorization_servers": testIssuer})
				return m.handler
			},
			reason: FailureNotJSON,
		},
		"not json": {
			handler: func(*metadataServer) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{oops")) }
			},
			reason: FailureNotJSON,
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := newMetadataServer(t)
			m.handler = tc.handler(m)
			f := m.verifier(true).Check(context.Background(), m.identifier, testIssuer)
			require.NotNil(t, f)
			assert.Equal(t, tc.reason, f.Reason)
			assert.Equal(t, tc.status, f.Status)
			assert.Equal(t, time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC), f.CheckedAt)
			if name == "a wrong resource" {
				assert.Equal(t, m.identifier+"/", f.Resource)
			}
		})
	}
}

// SECURITY: the SSRF filter refuses a private or loopback host unless the dev-only allowPrivateHosts is on.
func TestVerifier_PrivateHostBlockedUnlessAllowed(t *testing.T) {
	m := newMetadataServer(t)
	f := m.verifier(false).Check(context.Background(), m.identifier, testIssuer)
	require.NotNil(t, f)
	assert.Equal(t, FailureBlocked, f.Reason)
	assert.Empty(t, m.fetched(), "nothing was fetched")
	assert.Nil(t, m.verifier(true).Check(context.Background(), m.identifier, testIssuer))
}

func TestVerifier_TimeoutAndNetwork(t *testing.T) {
	m := newMetadataServer(t)
	release := make(chan struct{})
	m.handler = func(http.ResponseWriter, *http.Request) { <-release }
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	f := m.verifier(true).Check(ctx, m.identifier, testIssuer)
	require.NotNil(t, f)
	assert.Equal(t, FailureTimeout, f.Reason)

	close(release)
	m.srv.Close()
	f = m.verifier(true).Check(context.Background(), m.identifier, testIssuer)
	require.NotNil(t, f)
	assert.Equal(t, FailureNetwork, f.Reason)

	f = m.verifier(true).Check(context.Background(), "https://bad host", testIssuer)
	require.NotNil(t, f)
	assert.Equal(t, FailureNetwork, f.Reason)
}

func TestNewVerifier(t *testing.T) {
	v := NewVerifier(5*time.Second, false)
	assert.Equal(t, 5*time.Second, v.client.Timeout)
	assert.NotNil(t, v.client.CheckRedirect, "redirects are never followed")
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", truncate("abc", 5))
	assert.Equal(t, "ab", truncate("abcdef", 2))
	assert.Equal(t, "a", truncate("aé", 2), "a cut never splits a rune")
}
