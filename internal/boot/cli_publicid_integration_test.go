//go:build integration

package boot

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apikeyv1connect "github.com/altalune-id/othra/gen/go/apikey/v1/apikeyv1connect"
	authv1connect "github.com/altalune-id/othra/gen/go/auth/v1/authv1connect"
	invitev1connect "github.com/altalune-id/othra/gen/go/invite/v1/invitev1connect"
	memberv1connect "github.com/altalune-id/othra/gen/go/member/v1/memberv1connect"
	orgv1connect "github.com/altalune-id/othra/gen/go/org/v1/orgv1connect"
	projectv1connect "github.com/altalune-id/othra/gen/go/project/v1/projectv1connect"
	webhookv1connect "github.com/altalune-id/othra/gen/go/webhook/v1/webhookv1connect"
	"github.com/altalune-id/othra/internal/apiclient"
	"github.com/altalune-id/othra/internal/apikey"
	"github.com/altalune-id/othra/internal/auth"
	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/org"
	"github.com/altalune-id/othra/internal/org/invite"
	"github.com/altalune-id/othra/internal/org/member"
	"github.com/altalune-id/othra/internal/project"
	"github.com/altalune-id/othra/internal/webhook"
)

func cliRoot(c *apiclient.Client) *cobra.Command {
	root := &cobra.Command{Use: "othra", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("output", "", "")
	for _, g := range []string{"tenancy", "domain", "auth"} {
		root.AddGroup(&cobra.Group{ID: g, Title: g})
	}
	root.AddCommand(
		org.NewCmd(func(*cobra.Command) (orgv1connect.OrgServiceClient, error) { return c.Org, nil }),
		project.NewCmd(func(*cobra.Command) (projectv1connect.ProjectServiceClient, error) { return c.Projects, nil }),
		apikey.NewCmd(func(*cobra.Command) (apikeyv1connect.APIKeyServiceClient, error) { return c.APIKeys, nil }),
		webhook.NewCmd(func(*cobra.Command) (webhookv1connect.WebhookServiceClient, error) { return c.Webhooks, nil }),
		member.NewCmd(func(*cobra.Command) (memberv1connect.MemberServiceClient, error) { return c.Members, nil }),
		invite.NewCmd(func(*cobra.Command) (invitev1connect.InviteServiceClient, error) { return c.Invites, nil }),
		auth.NewCmd(nil, func(*cobra.Command) (authv1connect.AuthServiceClient, bool, error) { return c.Auth, false, nil }, nil),
	)
	return root
}

// every command group's JSON output holds public ids only.
func TestCLI_JSONHoldsNoUUID(t *testing.T) {
	f := newAPIFixture(t)
	s := seedReach(t, f)
	cookie, _, _ := f.personCookieIn(t, s.orgB, "b-owner@example.com", member.RoleOwner)
	c := apiclient.New(strings.TrimSuffix(f.url, "/api"), apiclient.Credential{Session: strings.TrimPrefix(cookie, session.CookieName+"=")})
	scope := []string{"--org-id", s.pub.org, "--project-id", s.pub.project}
	commands := [][]string{
		{"org", "list"},
		{"org", "get", s.pub.org},
		{"project", "list", "--org-id", s.pub.org},
		{"project", "get", s.pub.project},
		{"apikey", "create", "--org-id", s.pub.org, "--name", "probe", "--scopes", "webhook:read", "--all-projects"},
		{"apikey", "list", "--org-id", s.pub.org},
		append([]string{"webhook", "list"}, scope...),
		append([]string{"webhook", "get", s.pub.webhook}, scope...),
		append([]string{"webhook", "deliveries", s.pub.webhook}, scope...),
		append([]string{"webhook", "attempts", s.pub.webhook, s.pub.delivery}, scope...),
		{"member", "list", "--org-id", s.pub.org},
		{"invite", "list", "--org-id", s.pub.org},
		{"auth", "whoami"},
	}
	for _, args := range commands {
		buf := new(bytes.Buffer)
		root := cliRoot(c)
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs(append(args, "--output", "json"))
		require.NoError(t, root.ExecuteContext(t.Context()), "%v: %s", args, buf.String())
		assert.NotEmpty(t, strings.TrimSpace(buf.String()), "%v", args)
		assert.Empty(t, uuidShaped.FindAllString(buf.String(), -1), "%v printed a UUID: %s", args, buf.String())
	}
}
