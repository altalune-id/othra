//go:build integration

package boot

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/grant"
	"altalune.id/othra/internal/pat"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/resourceserver"
	"altalune.id/othra/oauthrs"
)

type toTestServer struct{ target *url.URL }

func (t toTestServer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Scheme, r.URL.Host, r.Host = t.target.Scheme, t.target.Host, ""
	return http.DefaultTransport.RoundTrip(r)
}

// An app's resource server built on oauthrs accepts Othra's own tokens: its at+jwt for that audience and its personal tokens.
func TestOAuthRSVerifiesOthraTokens(t *testing.T) {
	ts, svcs := newIssuerServer(t, "")
	ctx := t.Context()
	target, err := url.Parse(ts.URL)
	require.NoError(t, err)
	sys, err := svcs.Orgs.System(ctx)
	require.NoError(t, err)
	p, err := svcs.Projects.Create(ctx, project.CreateRequest{OrgID: sys.ID, Slug: "web", Name: "Web", IssuerHandle: "acme-web"})
	require.NoError(t, err)
	issuer := "https://id.example.test/p/acme-web"

	c, err := svcs.Clients.Create(ctx, sys.ID, p.ID, client.KindWeb, client.Settings{
		Name: "App", RedirectURIs: []string{"https://app.example/cb"}, ConsentPolicy: client.ConsentImplicit,
	})
	require.NoError(t, err)
	_, clientSecret, err := svcs.Clients.GenerateSecret(ctx, sys.ID, p.ID, c.PublicID)
	require.NoError(t, err)
	server := func(name, identifier string) (*resourceserver.ResourceServer, string) {
		rs, err := svcs.ResourceServers.Create(ctx, resourceserver.CreateRequest{
			OrgID: sys.ID, ProjectID: p.ID, Name: name, Identifier: identifier, Scopes: []string{"read", "write"}, Trusted: true, Person: uuid.New(),
		})
		require.NoError(t, err)
		_, s, err := svcs.ResourceServers.GenerateIntrospectionSecret(ctx, sys.ID, p.ID, rs.PublicID)
		require.NoError(t, err)
		return rs, s
	}
	notes, notesSecret := server("Notes", "https://notes.example.test/mcp")
	other, _ := server("Other", "https://other.example.test/mcp")
	_, err = svcs.ResourceServers.LinkClient(ctx, sys.ID, p.ID, c.PublicID, notes.PublicID, []string{"read"})
	require.NoError(t, err)
	_, err = svcs.ResourceServers.SetPersonalTokenAccess(ctx, sys.ID, p.ID, notes.PublicID, true)
	require.NoError(t, err)

	eu := enduser.Project{OrgID: sys.ID, ID: p.ID, IssuerURL: issuer, EmailLink: true}
	link, err := svcs.EndUsers.IssueLink(ctx, eu, enduser.LinkRequest{Email: "ada@example.com", ReturnTo: "/p/acme-web/account/"})
	require.NoError(t, err)
	u, err := url.Parse(link.URL)
	require.NoError(t, err)
	signed, err := svcs.EndUsers.Redeem(ctx, eu, enduser.RedeemRequest{Token: u.Query().Get("t")})
	require.NoError(t, err)
	code, err := svcs.Grants.IssueCode(ctx, grant.Project{OrgID: sys.ID, ID: p.ID}, grant.CodeRequest{
		Client: c.PublicID, RedirectURI: "https://app.example/cb", Scopes: []string{"openid", "read"},
		UserSession: signed.Account.PublicID, EndUser: signed.Account.Subject, AuthTime: signed.Account.AuthTime, ResourceServerID: &notes.ID,
	})
	require.NoError(t, err)
	status, _, tokens := postForm(t, ts.URL+"/p/acme-web/oauth2/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {"https://app.example/cb"}, "resource": {notes.Identifier},
	}, "Authorization", oauthBasic(c.PublicID, clientSecret))
	require.Equal(t, http.StatusOK, status, "%v", tokens)
	access, _ := tokens["access_token"].(string)
	idToken, _ := tokens["id_token"].(string)
	require.NotEmpty(t, idToken)

	httpClient := &http.Client{Transport: toTestServer{target: target}}
	verifier := func(audience string) *oauthrs.Verifier {
		v, err := oauthrs.New(oauthrs.Config{
			Issuer: issuer, Audience: audience, HTTPClient: httpClient,
			Introspection: &oauthrs.Introspection{ClientID: notes.PublicID, ClientSecret: notesSecret, CacheTTL: -1},
		})
		require.NoError(t, err)
		return v
	}
	v := verifier(notes.Identifier)

	tok, err := v.Verify(ctx, access)
	require.NoError(t, err)
	assert.Equal(t, issuer, tok.Issuer)
	assert.Equal(t, signed.Account.Subject, tok.Subject)
	assert.Equal(t, c.PublicID, tok.ClientID)
	assert.True(t, tok.HasScope("read"))
	assert.False(t, tok.Personal)

	_, err = verifier(other.Identifier).Verify(ctx, access)
	assert.True(t, oauthrs.IsInvalidTokenError(err), "RFC 8707: another resource's audience: %v", err)
	_, err = v.Verify(ctx, idToken)
	assert.True(t, oauthrs.IsInvalidTokenError(err), "RFC 9068 §4: an ID token is not an access token: %v", err)

	pp := pat.Project{OrgID: sys.ID, ID: p.ID, PublicID: p.PublicID, Name: p.Name}
	created, err := svcs.PersonalTokens.Create(ctx, pp, pat.CreateRequest{
		EndUserID: signed.Account.EndUserID, Name: "CLI", ServerPublicID: notes.PublicID, Scopes: []string{"read"}, Expiry: pat.Expiry30Days,
	})
	require.NoError(t, err)
	tok, err = v.Verify(ctx, created.Secret)
	require.NoError(t, err)
	assert.True(t, tok.Personal)
	assert.Equal(t, signed.Account.Subject, tok.Subject)
	assert.True(t, tok.HasScope("read"))

	require.NoError(t, svcs.PersonalTokens.Revoke(ctx, pp, signed.Account.EndUserID, created.Token.PublicID))
	_, err = v.Verify(ctx, created.Secret)
	assert.True(t, oauthrs.IsInvalidTokenError(err), "RFC 7662: a revoked personal token is inactive at once: %v", err)
}
