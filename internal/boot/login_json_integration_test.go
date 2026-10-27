//go:build integration

package boot

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The CLI signs in through the JSON login under row-level security, and the view names the active org and project.
func TestLogin_JSONSignsInUnderRLS(t *testing.T) {
	l := localLockdown(t)
	header := http.Header{
		"Content-Type":   {"application/x-www-form-urlencoded"},
		"Accept":         {"application/json"},
		"Sec-Fetch-Site": {"same-origin"},
	}
	form := url.Values{"email": {"root@example.com"}, "password": {"genesis-password"}}.Encode()
	rec := l.do(http.MethodPost, "/login", header, form)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body struct {
		Principal map[string]any `json:"principal"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.NotEmpty(t, body.Principal["active_org_id"], "%v", body.Principal)
	assert.NotEmpty(t, body.Principal["active_project_id"], "%v", body.Principal)
}
