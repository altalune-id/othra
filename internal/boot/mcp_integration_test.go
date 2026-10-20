//go:build integration

package boot

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/internal/apikey"
)

func (f *apiFixture) mcpCall(t *testing.T, bearer, body string) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, f.mcpURL, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, resp.Header, out
}

func TestMCP_KeyScopesTheToolList(t *testing.T) {
	f := newAPIFixture(t)
	key := f.mint(t, apikey.KindOrg, "webhook:read")

	status, _, body := f.mcpCall(t, key, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	require.Equal(t, http.StatusOK, status, string(body))
	var list struct {
		Result struct {
			Tools []struct{ Name string } `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(body, &list))
	var names []string
	for _, tool := range list.Result.Tools {
		names = append(names, tool.Name)
	}
	assert.ElementsMatch(t, []string{"webhook_list", "webhook_deliveries", "auth_whoami"}, names)
}

func TestMCP_WhoamiWithAKey(t *testing.T) {
	f := newAPIFixture(t)
	key := f.mint(t, apikey.KindOrg, "webhook:read")

	status, _, body := f.mcpCall(t, key, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"auth_whoami","arguments":{}}}`)
	require.Equal(t, http.StatusOK, status, string(body))
	var call struct {
		Result struct {
			IsError           bool           `json:"isError"`
			StructuredContent map[string]any `json:"structuredContent"`
			Content           []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(body, &call))
	require.False(t, call.Result.IsError, string(body))
	assert.Contains(t, string(body), `\"source\":\"apikey\"`)
}

func TestMCP_BadKeyIs401(t *testing.T) {
	f := newAPIFixture(t)
	status, header, body := f.mcpCall(t, "othra_nope", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "Bearer", header.Get("WWW-Authenticate"))
	assert.Contains(t, string(body), `"code":"GEN002"`)
}

// Review Focus 1: no MCP tool answers with a UUID, for a key that sees org B.
func TestMCP_ToolResultsHoldNoUUID(t *testing.T) {
	f := newAPIFixture(t)
	s := seedReach(t, f)
	key := f.mintKey(t, apikey.CreateRequest{OrgID: s.orgB, Kind: apikey.KindOrg, Scopes: mintableScopes(), AllProjects: true})
	status, _, body := f.mcpCall(t, key, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	require.Equal(t, http.StatusOK, status, string(body))
	var list struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				InputSchema struct {
					Properties map[string]any `json:"properties"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(body, &list))
	require.NotEmpty(t, list.Result.Tools)
	values := map[string]any{
		"org_id": s.pub.org, "project_id": s.pub.project, "invite_id": s.pub.invite, "user_id": s.pub.member,
		"delivery_id": s.pub.delivery, "name": "Probe", "email": "probe@example.com", "role": "member",
		"url": "https://example.com/probe", "event_types": []string{"invite.created"},
		"client_id": s.pub.client, "kind": "CLIENT_KIND_WEB", "redirect_uris": []string{"https://example.com/probe"},
		"consent_policy": "CONSENT_POLICY_REQUIRE",
	}
	ids := map[string]string{"org": s.pub.org, "project": s.pub.project, "webhook": s.pub.webhook}
	answered := 0
	for _, tool := range list.Result.Tools {
		args := map[string]any{}
		for prop := range tool.InputSchema.Properties {
			if v, ok := values[snake(prop)]; ok {
				args[prop] = v
			}
			if prop == "id" {
				args[prop] = ids[strings.SplitN(tool.Name, "_", 2)[0]]
			}
		}
		raw, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": tool.Name, "arguments": args},
		})
		require.NoError(t, err)
		status, _, body := f.mcpCall(t, key, string(raw))
		require.Equal(t, http.StatusOK, status, "%s: %s", tool.Name, body)
		if !strings.Contains(string(body), `"isError":true`) {
			answered++
		}
		assert.Empty(t, uuidShaped.FindAllString(string(body), -1), "%s answered with a UUID: %s", tool.Name, body)
	}
	assert.Greater(t, answered, len(list.Result.Tools)/2, "most tools must succeed for the check to mean anything")
}

// SECURITY: D4, no MCP tool hands out a client secret; a web client created over MCP has none.
func TestMCP_ClientToolsNeverReturnASecret(t *testing.T) {
	f := newAPIFixture(t)
	s := seedReach(t, f)
	key := f.mintKey(t, apikey.CreateRequest{OrgID: s.orgB, Kind: apikey.KindOrg, Scopes: mintableScopes(), AllProjects: true})
	status, _, body := f.mcpCall(t, key, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	require.Equal(t, http.StatusOK, status, string(body))
	var list struct {
		Result struct {
			Tools []struct{ Name string } `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(body, &list))
	require.NotEmpty(t, list.Result.Tools)
	for _, tool := range list.Result.Tools {
		assert.NotContains(t, tool.Name, "secret", "no secret tool is published")
		assert.NotContains(t, tool.Name, "trust", "the trust switch is person-only")
	}

	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{
		"name": "client_create", "arguments": map[string]any{
			"projectId": s.pub.project, "name": "MCP web", "kind": "CLIENT_KIND_WEB", "redirectUris": []string{"https://example.com/cb"},
		},
	}})
	require.NoError(t, err)
	status, _, body = f.mcpCall(t, key, string(raw))
	require.Equal(t, http.StatusOK, status, string(body))
	assert.NotContains(t, string(body), `"isError":true`, string(body))
	assert.Contains(t, string(body), "MCP web")
	assert.NotContains(t, string(body), "othra_cs_")
	assert.NotContains(t, string(body), `\"secret\"`)
	assert.NotContains(t, string(body), `"secret"`)
	var n int
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM oauth_client_secrets WHERE org_id = $1`, s.orgB).Scan(&n))
	assert.Zero(t, n, "the client has no secret until a person generates one")
}

func snake(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}
