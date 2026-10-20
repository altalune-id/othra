package controlplane

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	apperrorv1 "altalune.id/othra/gen/go/apperror/v1"
	"altalune.id/othra/internal/apikey"
	"altalune.id/othra/internal/auth"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/install"
	"altalune.id/othra/internal/kernel/authn"
	"altalune.id/othra/internal/kernel/authz"
	"altalune.id/othra/internal/kernel/events"
	"altalune.id/othra/internal/kernel/storage"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/keys"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/org/invite"
	"altalune.id/othra/internal/org/member"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/resourceserver"
	"altalune.id/othra/internal/user"
	"altalune.id/othra/internal/webhook"
	"altalune.id/othra/password"
	"altalune.id/othra/scope"
)

func errOf[T any](_ T, err error) error { return err }

func samples() []error {
	nobody := uuid.New()
	hook := []events.Type{events.InviteCreated}
	return []error{
		&tenant.MissingError{},
		&tenant.UnscopedError{},
		&authn.UnauthorizedError{},
		apperror.Invalid("organization_id", apperror.FieldInvalid),
		apperror.Invalid("owner", apperror.FieldRequired),
		apperror.Validation([]apperror.Violation{{Field: "slug", Reason: apperror.FieldTooShort, Param: "3"}}),
		apperror.Validation([]apperror.Violation{{Field: "name", Reason: apperror.FieldTooLong, Param: "200"}}),
		apperror.Validation([]apperror.Violation{{Field: "id", Reason: apperror.FieldInvalidFormat}}),
		apperror.Validation([]apperror.Violation{{Field: "role", Reason: apperror.FieldNotAllowed}}),
		apperror.Validation([]apperror.Violation{{Field: "events", Reason: apperror.FieldTooMany, Param: "20"}}),
		errOf(member.New(uuid.Nil, nobody, member.RoleOwner)),
		apperror.New(apperror.CodeNotFound, "Not found"),
		apperror.New(apperror.CodeAlreadyExists, "Already exists"),
		&authz.ForbiddenError{Permission: scope.WebhookWrite},
		&tenant.ProjectOutsideOrgError{},
		apperror.NewReporter(nil, false).Unexpected(context.Background(), "boom", errors.New("secret detail")),

		&org.NotFoundError{Slug: "acme"},
		&org.AlreadyExistsError{Slug: "acme"},
		errOf(org.NewOrg("a", "Acme", nobody)),
		errOf(org.NewOrg("Not_A_Slug", "Acme", nobody)),
		errOf(org.NewOrg("new", "Acme", nobody)),
		errOf(org.NewOrg("acme", "", nobody)),
		&org.CreationDisabledError{},

		&project.NotFoundError{Slug: "web"},
		&project.AlreadyExistsError{Slug: "web"},
		errOf(project.NewProject(nobody, "a", "Web", nobody)),
		errOf(project.NewProject(nobody, "Not_A_Slug", "Web", nobody)),
		errOf(project.NewProject(nobody, "web", "", nobody)),
		&project.IssuerHandleTakenError{Handle: "acme-web"},
		&project.InvalidIssuerHandleError{Handle: "Acme", Reason: project.IssuerHandleFormat},
		&project.InvalidIssuerHandleError{Handle: "ab", Reason: project.IssuerHandleLength},
		&project.InvalidIssuerHandleError{Handle: "system", Reason: project.IssuerHandleReserved},
		&project.IssuerHandleLockedError{},
		&project.InvalidLogoError{Reason: project.LogoType},
		&project.InvalidLogoError{Reason: project.LogoSize},
		&project.TermsNotSetError{},

		&apikey.NotFoundError{ID: "x"},
		apikey.ValidateName(""),
		apikey.ValidateName(strings.Repeat("n", 101)),
		&apikey.UnknownScopeError{Scope: "nope:read"},
		&apikey.ScopeLevelError{Scope: scope.OrgWrite},
		&apikey.ExpiryRequiredError{},
		&apikey.ExpiryInPastError{},
		&apikey.ExpiryTooLongError{},
		&apikey.GrantConflictError{},
		&apikey.EmptyGrantError{},
		&apikey.GrantOutsideOrgError{},
		&apikey.PersonOnlyScopeError{Scope: scope.MemberWrite},
		&apikey.ScopeExceedsRoleError{Scope: scope.OrgWrite},
		&apikey.ScopeExceedsRoleError{},
		&apikey.PersonalKeysDisabledError{},

		&user.NotFoundError{},
		&user.AlreadyExistsError{Email: "someone@example.com"},
		errOf(user.New("not-an-email", "Someone")),
		errOf(user.New("someone@example.com", strings.Repeat("n", 201))),

		&member.NotFoundError{},
		&member.AlreadyMemberError{OrgID: nobody, UserID: nobody},
		errOf(member.New(nobody, nobody, member.Role("root"))),
		&member.LastOwnerError{Reason: member.LastOwnerRemove},
		&member.LastOwnerError{Reason: member.LastOwnerDemote},
		&member.LastOwnerError{Reason: member.LastOwnerLeave},
		&member.RoleNotAllowedError{Role: string(member.RoleOwner)},
		&member.SelfChangeError{},

		&invite.NotUsableError{Reason: invite.ReasonExpired},
		&invite.NotUsableError{Reason: invite.ReasonRevoked},
		&invite.NotUsableError{Reason: invite.ReasonUsed},
		&invite.NotUsableError{Reason: invite.ReasonNotFound},
		&invite.EmailMismatchError{},
		&invite.DraftExpiredError{},
		&invite.NotFoundError{},
		&invite.PendingError{},

		&auth.InvalidCredentialsError{},
		&auth.RateLimitedError{RetryAfter: 90 * time.Second},
		&auth.ResetNotUsableError{Reason: auth.ResetExpired},
		&auth.ResetNotUsableError{Reason: auth.ResetUsed},
		&auth.ResetNotUsableError{Reason: auth.ResetNotFound},
		password.Check("short", password.Hints{}),
		password.Check(strings.Repeat("a", password.MaxLen+1), password.Hints{}),
		password.Check("iloveyou1234", password.Hints{}),
		password.Check("abababababab", password.Hints{}),
		password.Check("abcdefghijklm", password.Hints{}),
		password.Check("alice-is-great-2026", password.Hints{Email: "alice@example.com"}),
		password.Check("maria-loves-tea", password.Hints{Name: "Ana Maria"}),

		&install.AlreadyInstalledError{},
		apperror.New(apperror.CodeInstallRequired, "Setup required"),

		&webhook.NotFoundError{ID: "x"},
		errOf(webhook.New(nobody, nobody, "", "", hook)),
		errOf(webhook.New(nobody, nobody, "https://e.com/"+strings.Repeat("a", webhook.MaxURLLen), "", hook)),
		errOf(webhook.New(nobody, nobody, "https://e.com/\xff", "", hook)),
		errOf(webhook.New(nobody, nobody, "https://e.com/%zz", "", hook)),
		errOf(webhook.New(nobody, nobody, "ftp://e.com", "", hook)),
		errOf(webhook.New(nobody, nobody, "https://u:p@e.com", "", hook)),
		errOf(webhook.New(nobody, nobody, "https://", "", hook)),
		errOf(webhook.New(nobody, nobody, "https://e.com", "", nil)),
		errOf(webhook.New(nobody, nobody, "https://e.com", "", []events.Type{"x.y"})),
		errOf(webhook.New(nobody, nobody, "https://e.com", "", []events.Type{events.WebhookPing})),
		errOf(webhook.New(nobody, nobody, "https://e.com", "", []events.Type{events.InviteCreated, events.InviteCreated})),
		errOf(webhook.New(nobody, nobody, "https://e.com", "\xff", hook)),
		errOf(webhook.New(nobody, nobody, "https://e.com", strings.Repeat("d", webhook.MaxDescriptionRunes+1), hook)),
		&webhook.EndpointLimitError{Limit: 3},
		&webhook.EndpointInactiveError{ID: "x"},
		&webhook.SecretConflictError{ID: "x"},
		&webhook.DeliveryNotFoundError{ID: "x"},
		&webhook.DeliveryNotRetryableError{ID: "x"},

		&storage.UploadNotFoundError{PublicID: "x"},
		&storage.UploadMismatchError{PublicID: "x", Reason: storage.MismatchMissing},
		&storage.UploadMismatchError{PublicID: "x", Reason: storage.MismatchSize},
		&storage.UploadMismatchError{PublicID: "x", Reason: storage.MismatchType},
		&storage.UploadMismatchError{PublicID: "x", Reason: storage.MismatchContent},
		&storage.NotConfiguredError{},

		&keys.NoActiveKeyError{ProjectID: "x"},
		&keys.RotationPendingError{ProjectID: "x"},

		&client.NotFoundError{},
		&client.InvalidRedirectURIError{Field: "redirect_uris", URI: "https://A.example/cb", Reason: client.RedirectURINotCanonical, Canonical: "https://a.example/cb"},
		&client.InvalidRedirectURIError{Field: "redirect_uris", URI: "x", Reason: client.RedirectURIRequired},
		&client.InvalidRedirectURIError{Field: "redirect_uris", URI: "x", Reason: client.RedirectURIUnparsable},
		&client.InvalidRedirectURIError{Field: "redirect_uris", URI: "x", Reason: client.RedirectURIFragment},
		&client.InvalidRedirectURIError{Field: "redirect_uris", URI: "x", Reason: client.RedirectURIWildcard},
		&client.InvalidRedirectURIError{Field: "redirect_uris", URI: "x", Reason: client.RedirectURIUserinfo},
		&client.InvalidRedirectURIError{Field: "redirect_uris", URI: "x", Reason: client.RedirectURIDotSegments},
		&client.InvalidRedirectURIError{Field: "redirect_uris", URI: "x", Reason: client.RedirectURIHTTPSRequired},
		&client.InvalidRedirectURIError{Field: "redirect_uris", URI: "x", Reason: client.RedirectURILoopbackOnly},
		&client.InvalidRedirectURIError{Field: "redirect_uris", URI: "x", Reason: client.RedirectURIPortIgnored},
		&client.InvalidRedirectURIError{Field: "redirect_uris", URI: "x", Reason: client.RedirectURITooLong},
		&client.InvalidRedirectURIError{Field: "redirect_uris", URI: "x", Reason: client.RedirectURITooMany},
		&client.InvalidRedirectURIError{Field: "redirect_uris", URI: "x", Reason: client.RedirectURIDuplicate},
		&client.InvalidSettingsError{Reason: client.SettingsPostLogoutNative},
		&client.NoSecretError{},
		&client.SecretPersonOnlyError{},
		&client.SecretNotFoundError{},
		&client.SecretExistsError{},

		&resourceserver.NotFoundError{},
		&resourceserver.InvalidIdentifierError{Identifier: "https://A.example", Reason: resourceserver.IdentifierNotCanonical, Canonical: "https://a.example"},
		&resourceserver.InvalidIdentifierError{Identifier: "x", Reason: resourceserver.IdentifierUnparsable},
		&resourceserver.InvalidIdentifierError{Identifier: "x", Reason: resourceserver.IdentifierHTTPSRequired},
		&resourceserver.InvalidIdentifierError{Identifier: "x", Reason: resourceserver.IdentifierQuery},
		&resourceserver.InvalidIdentifierError{Identifier: "x", Reason: resourceserver.IdentifierFragment},
		&resourceserver.InvalidIdentifierError{Identifier: "x", Reason: resourceserver.IdentifierUserinfo},
		&resourceserver.InvalidIdentifierError{Identifier: "x", Reason: resourceserver.IdentifierTooLong},
		&resourceserver.InvalidIdentifierError{Identifier: "x", Reason: resourceserver.IdentifierNeedsTrust},
		&resourceserver.IdentifierTakenError{Identifier: "https://a.example"},
		&resourceserver.InvalidScopeError{Scope: "x", Reason: resourceserver.ScopeFormat},
		&resourceserver.InvalidScopeError{Scope: "x", Reason: resourceserver.ScopeReserved},
		&resourceserver.InvalidScopeError{Scope: "x", Reason: resourceserver.ScopeDuplicate},
		&resourceserver.InvalidScopeError{Scope: "x", Reason: resourceserver.ScopeTooMany},
		&resourceserver.ScopePinnedError{Scope: "read", Clients: []string{"Web"}},
		&resourceserver.LinkedError{Clients: []string{"Web"}},
		&resourceserver.TrustSelfHostedOnlyError{},
		&resourceserver.VerifyTooSoonError{RetryAt: time.Unix(0, 0)},
		&resourceserver.InvalidLinkScopesError{Reason: resourceserver.LinkScopesEmpty},
		&resourceserver.InvalidLinkScopesError{Scope: "x", Reason: resourceserver.LinkScopesNotInCatalog},
	}
}

var diagnostic = []string{"trace_id", "request_id", "underlying_error"}

type catalogEntry struct {
	Code    string   `json:"code"`
	Params  []string `json:"params"`
	Reasons []string `json:"reasons"`
}

func catalog(t *testing.T) []catalogEntry {
	t.Helper()
	raw, err := os.ReadFile("../../apperror/codes.json")
	require.NoError(t, err)
	var entries []catalogEntry
	require.NoError(t, json.Unmarshal(raw, &entries))
	require.NotEmpty(t, entries)
	return entries
}

func wireDetail(t *testing.T, err error) (*apperrorv1.ErrorDetail, error) {
	t.Helper()
	unexpected := func(context.Context, string, error, ...any) *apperror.AppError {
		t.Errorf("%T reached the wire as an unexpected error", err)
		return apperror.New(apperror.CodeUnexpected, "reported")
	}
	out := Translate(context.Background(), err, unexpected)
	var cerr *connect.Error
	require.ErrorAs(t, out, &cerr)
	require.Len(t, cerr.Details(), 1, "%T: no ErrorDetail", err)
	value, verr := cerr.Details()[0].Value()
	require.NoError(t, verr)
	det, ok := value.(*apperrorv1.ErrorDetail)
	require.True(t, ok)
	ae, _ := apperror.AsAppError(err)
	return det, ae.Unwrap()
}

func TestEveryCodeSendsItsCatalogParams(t *testing.T) {
	// A code's samples together send every param and reason it declares; a sample without meta.reason gets the code's default message.
	entries := catalog(t)
	byCode := map[string]catalogEntry{}
	for _, e := range entries {
		byCode[e.Code] = e
	}

	sent := map[string]map[string]bool{}
	for _, err := range samples() {
		require.Error(t, err)
		det, cause := wireDetail(t, err)
		code := det.GetCode()
		entry, ok := byCode[code]
		require.True(t, ok, "%s (from %T) is not in codes.json", code, err)
		if sent[code] == nil {
			sent[code] = map[string]bool{}
		}

		meta := det.GetMeta()
		if reason, ok := meta["reason"]; ok {
			assert.Contains(t, entry.Reasons, reason, "%s from %T: meta.reason", code, err)
			sent[code]["reason="+reason] = true
		}
		for k, v := range meta {
			switch {
			case k == "reason":
			case code == string(apperror.CodeUnexpected) && slices.Contains(diagnostic, k):
				// Operator diagnostics: the console never shows them, so the catalog doesn't declare them.
			default:
				assert.Contains(t, entry.Params, k, "%s from %T sends %q, which codes.json does not declare", code, err, k)
			}
			sent[code][k] = true
			if cause != nil && code != string(apperror.CodeUnexpected) {
				assert.NotEqual(t, cause.Error(), v, "%s: meta %q repeats the cause", code, k)
			}
		}
	}

	for _, e := range entries {
		got, ok := sent[e.Code]
		if !assert.True(t, ok, "%s has no sample error", e.Code) {
			continue
		}
		for _, p := range e.Params {
			assert.True(t, got[p], "%s: no sample sends param %q", e.Code, p)
		}
		for _, r := range e.Reasons {
			assert.True(t, got["reason="+r], "%s: no sample sends reason %q", e.Code, r)
		}
	}
}

func TestMetaNeverEchoesAnEmail(t *testing.T) {
	err := &user.AlreadyExistsError{Email: "someone@example.com"}
	det, _ := wireDetail(t, err)
	for _, v := range det.GetMeta() {
		assert.NotContains(t, v, "@")
	}
	assert.NotContains(t, Translate(context.Background(), err, nil).Error(), "@")
}

func TestEchoedInputIsCapped(t *testing.T) {
	long := strings.Repeat("x", 10*apperror.EchoLimit)
	for _, err := range []error{
		&webhook.InvalidEventTypesError{Event: events.Type(long), Reason: webhook.EventTypeUnknown},
		&webhook.InvalidEventTypesError{Event: events.Type(long), Reason: webhook.EventTypeNotSubscribable},
		&webhook.InvalidEventTypesError{Event: events.Type(long), Reason: webhook.EventTypeDuplicate},
		&apikey.UnknownScopeError{Scope: long},
		&apikey.ScopeLevelError{Scope: long},
		&apikey.PersonOnlyScopeError{Scope: long},
		&apikey.ScopeExceedsRoleError{Scope: long},
		&member.InvalidRoleError{Role: long},
		&org.InvalidSlugError{Slug: long, Reason: org.SlugReserved},
		&org.InvalidSlugError{Slug: long, Reason: "other"},
		&project.InvalidSlugError{Slug: long, Reason: "other"},
		&project.IssuerHandleTakenError{Handle: long},
		&project.InvalidIssuerHandleError{Handle: long, Reason: project.IssuerHandleReserved},
	} {
		det, _ := wireDetail(t, err)
		for k, v := range det.GetMeta() {
			assert.LessOrEqual(t, len(v), apperror.EchoLimit, "%T: meta %q", err, k)
		}
		var cerr *connect.Error
		require.ErrorAs(t, Translate(context.Background(), err, nil), &cerr)
		assert.Less(t, len(cerr.Message()), 2*apperror.EchoLimit, "%T: message", err)
		assert.Less(t, len(err.Error()), 2*apperror.EchoLimit, "%T: Error()", err)
	}
}

func TestEveryInvalidFieldHasALabel(t *testing.T) {
	fields := map[string]string{}
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && slices.Contains([]string{".git", "node_modules", "gen", "ui", "docs"}, d.Name()) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Invalid" || fmt.Sprint(sel.X) != "apperror" {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !assert.True(t, ok && lit.Kind == token.STRING, "%s: apperror.Invalid needs a literal field name", path) {
				return true
			}
			name, _ := strconv.Unquote(lit.Value)
			fields[name] = path
			return true
		})
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, fields)

	locales, err := filepath.Glob("../../ui/console/i18n/locales/*.json")
	require.NoError(t, err)
	require.NotEmpty(t, locales)
	for _, path := range locales {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		var messages map[string]string
		require.NoError(t, json.Unmarshal(raw, &messages))
		for field, src := range fields {
			assert.Contains(t, messages, "field."+field, "%s: no label for field %q (used in %s)", filepath.Base(path), field, src)
		}
	}
}
