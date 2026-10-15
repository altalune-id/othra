package apiclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authv1 "altalune.id/othra/gen/go/auth/v1"
)

func headerRecorder(t *testing.T) (string, *http.Header) {
	t.Helper()
	var seen http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(ts.Close)
	return ts.URL, &seen
}

func TestNew_Credential(t *testing.T) {
	tests := []struct {
		name       string
		cred       Credential
		wantAuthz  string
		wantCookie string
	}{
		{name: "token is a bearer", cred: Credential{Token: "othra_k"}, wantAuthz: "Bearer othra_k"},
		{name: "session is the cookie", cred: Credential{Session: "sid.sig"}, wantCookie: "othra_sid=sid.sig"},
		{name: "token wins over session", cred: Credential{Token: "othra_k", Session: "sid.sig"}, wantAuthz: "Bearer othra_k"},
		{name: "none sends nothing", cred: Credential{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, seen := headerRecorder(t)
			_, err := New(url, tt.cred).Auth.Whoami(context.Background(), connect.NewRequest(&authv1.WhoamiRequest{}))
			require.Error(t, err)
			assert.Equal(t, tt.wantAuthz, seen.Get("Authorization"))
			assert.Equal(t, tt.wantCookie, seen.Get("Cookie"))
		})
	}
}
