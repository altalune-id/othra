package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/httpclient"
)

type cimdServer struct {
	srv      *httptest.Server
	mu       sync.Mutex
	requests map[string]int
	fetcher  *HTTPMetadataFetcher
}

func (m *cimdServer) count(path string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requests[path]
}

func newCIMDServer(t *testing.T, allowPrivate bool) *cimdServer {
	t.Helper()
	m := &cimdServer{requests: map[string]int{}}
	m.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.requests[r.URL.Path]++
		m.mu.Unlock()
		switch r.URL.Path {
		case "/ok":
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "max-age=600")
			_, _ = w.Write([]byte(strings.ReplaceAll(claudeDocument, claudeURL, m.srv.URL+"/ok")))
		case "/redirect":
			http.Redirect(w, r, "/ok", http.StatusFound)
		case "/big":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"client_name":"` + strings.Repeat("x", MaxMetadataBytes) + `"}`))
		case "/html":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`{}`))
		case "/slow":
			time.Sleep(300 * time.Millisecond)
		default:
			http.Error(w, "boom", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(m.srv.Close)
	c := httpclient.New(append(metadataClientOptions(100*time.Millisecond, allowPrivate), httpclient.WithOtel(false))...)
	pool := x509.NewCertPool()
	pool.AddCert(m.srv.Certificate())
	c.Transport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	m.fetcher = &HTTPMetadataFetcher{client: c}
	return m
}

var testCache = MetadataConfig{Cache: MetadataCache{Min: 5 * time.Minute, Max: 24 * time.Hour, Default: time.Hour}}

// a 200 JSON document under 5 KB is used for its clamped max-age.
func TestLoadMetadata_OK(t *testing.T) {
	m := newCIMDServer(t, true)
	got, ttl, err := LoadMetadata(context.Background(), m.fetcher, testCache, m.srv.URL+"/ok")
	require.NoError(t, err)
	assert.Equal(t, "Claude Code", got.ClientName)
	assert.Equal(t, 10*time.Minute, ttl)
}

// no redirect is followed; any other status, a body over 5 KB, a non-JSON type or a timeout fails.
func TestLoadMetadata_Failures(t *testing.T) {
	m := newCIMDServer(t, true)
	for path, reason := range map[string]MetadataReason{
		"/redirect": MetadataFetch,
		"/missing":  MetadataFetch,
		"/big":      MetadataTooLarge,
		"/html":     MetadataContentType,
		"/slow":     MetadataFetch,
	} {
		_, _, err := LoadMetadata(context.Background(), m.fetcher, testCache, m.srv.URL+path)
		me, ok := errors.AsType[*MetadataError](err)
		require.True(t, ok, "%s: %v", path, err)
		assert.Equal(t, reason, me.Reason, path)
		assert.True(t, IsMetadataError(err))
	}
	assert.Zero(t, m.count("/ok"), "the redirect target is never fetched")
}

// a private address is refused at dial time unless the dev switch is on.
func TestLoadMetadata_PrivateHostBlocked(t *testing.T) {
	m := newCIMDServer(t, false)
	_, _, err := LoadMetadata(context.Background(), m.fetcher, testCache, m.srv.URL+"/ok")
	me, ok := errors.AsType[*MetadataError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, MetadataFetch, me.Reason)
	assert.True(t, httpclient.IsPrivateAddressError(err))
	assert.Zero(t, m.count("/ok"))
}

func TestHTTPMetadataFetcher_BadURL(t *testing.T) {
	_, err := NewMetadataFetcher(time.Second, false).FetchMetadata(context.Background(), "https://bad host/x")
	assert.Error(t, err)
}

// oauth.cimd.maxBytes caps the document below 5 KB.
func TestLoadMetadata_ConfiguredLimit(t *testing.T) {
	m := newCIMDServer(t, true)
	small := testCache
	small.MaxBytes = 100
	_, _, err := LoadMetadata(context.Background(), m.fetcher, small, m.srv.URL+"/ok")
	me, ok := errors.AsType[*MetadataError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, MetadataTooLarge, me.Reason)
}
