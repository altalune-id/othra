// Package client holds a project's OAuth clients: their redirect URIs, secrets and consent policy.
package client

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/kernel/secret"
	"github.com/altalune-id/othra/internal/oauth/redirecturi"
)

// Kind is the preset a client is created from; it fixes the client type, application type and auth method.
type Kind string

// The client kinds.
const (
	KindWeb Kind = "web"
	KindCLI Kind = "cli"
)

// ConsentPolicy says whether end users see the consent page.
type ConsentPolicy string

// The consent policies.
const (
	ConsentRequire  ConsentPolicy = "require"
	ConsentImplicit ConsentPolicy = "implicit"
)

// The RFC 6749 client types.
const (
	TypeConfidential = "confidential"
	TypePublic       = "public"
)

// The OIDC application types.
const (
	AppWeb    = string(redirecturi.Web)
	AppNative = string(redirecturi.Native)
)

// The token endpoint auth methods.
const (
	AuthClientSecretBasic = "client_secret_basic"
	AuthClientSecretPost  = "client_secret_post"
	AuthNone              = "none"
)

// The grant types and the scopes a client may hold.
const (
	GrantAuthorizationCode = "authorization_code"
	GrantRefreshToken      = "refresh_token"
	ScopeOfflineAccess     = "offline_access"
)

// The field names a redirect URI error names.
const (
	FieldRedirectURIs           = "redirect_uris"
	FieldPostLogoutRedirectURIs = "post_logout_redirect_uris"
)

// SecretPrefix starts every client secret, so secret scanners can find a leaked one.
const SecretPrefix = "othra_cs_" //nolint:gosec // G101: a prefix, not a credential.

const maxNameLen = 100

// Client is an OAuth client of a project; OAuthID is its client_id.
type Client struct {
	ID                      uuid.UUID
	PublicID                string
	OrgID                   uuid.UUID
	ProjectID               uuid.UUID
	Name                    string
	ClientType              string
	ApplicationType         string
	TokenEndpointAuthMethod string
	GrantTypes              []string
	AllowedScopes           []string
	RedirectURIs            []string
	PostLogoutRedirectURIs  []string
	ConsentPolicy           ConsentPolicy
	PKCERequired            bool
	// MetadataURL is set on a Client ID Metadata Document client, a read-only shadow row its document defines.
	MetadataURL       string
	ClientURI         string
	MetadataFetchedAt *time.Time
	MetadataExpiresAt *time.Time
	// System marks the console client Othra seeds at install; only boot changes it, and only its derived URIs.
	System    bool
	CreatedAt time.Time
	UpdatedAt time.Time
	Secrets   []Secret
}

// Secret is one of a client's live secrets; the plaintext is never kept.
type Secret = secret.Stored

// Settings are the fields of a client a person sets.
type Settings struct {
	Name                   string
	RedirectURIs           []string
	PostLogoutRedirectURIs []string
	RefreshTokens          bool
	ConsentPolicy          ConsentPolicy
	// AuthMethod is how a web client sends its secret; empty keeps the kind's default.
	AuthMethod string
	// PKCERequired nil means required; only a web client may turn it off.
	PKCERequired *bool
}

// Preset sets the protocol fields kind fixes: web is confidential with client_secret_basic, CLI is a public native client.
func (c *Client) Preset(kind Kind) {
	c.PKCERequired = true
	if kind == KindCLI {
		c.ClientType, c.ApplicationType, c.TokenEndpointAuthMethod = TypePublic, AppNative, AuthNone
		return
	}
	c.ClientType, c.ApplicationType, c.TokenEndpointAuthMethod = TypeConfidential, AppWeb, AuthClientSecretBasic
}

const systemName = "Othra console"

// NewSystem builds the console client: a first-party public web client with PKCE, implicit consent and no secret.
func NewSystem(orgID, projectID uuid.UUID, publicID, redirectURI, postLogoutURI string, now time.Time) (*Client, error) {
	c := &Client{
		ID: uuid.New(), PublicID: publicID, OrgID: orgID, ProjectID: projectID, System: true, CreatedAt: now, UpdatedAt: now,
		Secrets: []Secret{},
	}
	c.Preset(KindWeb)
	err := c.Apply(Settings{
		Name: systemName, RedirectURIs: []string{redirectURI}, PostLogoutRedirectURIs: []string{postLogoutURI},
		ConsentPolicy: ConsentImplicit,
	})
	if err != nil {
		return nil, err
	}
	c.ClientType, c.TokenEndpointAuthMethod = TypePublic, AuthNone
	return c, nil
}

// SetSystemURIs replaces the console client's derived redirect and sign-out URIs; it reports whether either changed.
func (c *Client) SetSystemURIs(redirectURI, postLogoutURI string) (bool, error) {
	if err := redirectError(FieldRedirectURIs, redirecturi.Validate(redirecturi.Web, redirectURI)); err != nil {
		return false, err
	}
	if err := redirectError(FieldPostLogoutRedirectURIs, redirecturi.Validate(redirecturi.Web, postLogoutURI)); err != nil {
		return false, err
	}
	redirects, postLogout := []string{redirectURI}, []string{postLogoutURI}
	if slices.Equal(c.RedirectURIs, redirects) && slices.Equal(c.PostLogoutRedirectURIs, postLogout) {
		return false, nil
	}
	c.RedirectURIs, c.PostLogoutRedirectURIs = redirects, postLogout
	return true, nil
}

// OAuthID is the client_id on the wire: the metadata document URL of a shadow client, else the public id.
func (c *Client) OAuthID() string {
	if c.MetadataURL != "" {
		return c.MetadataURL
	}
	return c.PublicID
}

// Managed reports whether the client is defined by its metadata document and so can't be changed here.
func (c *Client) Managed() bool { return c.MetadataURL != "" }

// Kind is the preset the client's application type came from.
func (c *Client) Kind() Kind {
	if c.ApplicationType == AppNative {
		return KindCLI
	}
	return KindWeb
}

// RefreshTokens reports whether the client may use the refresh_token grant.
func (c *Client) RefreshTokens() bool { return slices.Contains(c.GrantTypes, GrantRefreshToken) }

// UsesSecret reports whether the client's token endpoint auth method needs a client secret.
func (c *Client) UsesSecret() bool {
	switch c.TokenEndpointAuthMethod {
	case AuthClientSecretBasic, AuthClientSecretPost:
		return true
	default:
		return false
	}
}

// Apply checks settings against the client's application type and sets them.
func (c *Client) Apply(s Settings) error {
	name := strings.TrimSpace(s.Name)
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		return apperror.Validation([]apperror.Violation{{Field: "name", Reason: apperror.FieldRequired}})
	case n > maxNameLen:
		return apperror.Validation([]apperror.Violation{{Field: "name", Reason: apperror.FieldTooLong, Param: strconv.Itoa(maxNameLen)}})
	}
	appType := redirecturi.ApplicationType(c.ApplicationType)
	if err := redirectError(FieldRedirectURIs, redirecturi.ValidateList(appType, s.RedirectURIs, true)); err != nil {
		return err
	}
	if c.ApplicationType == AppNative && len(s.PostLogoutRedirectURIs) > 0 {
		return &InvalidSettingsError{Reason: SettingsPostLogoutNative}
	}
	if err := redirectError(FieldPostLogoutRedirectURIs, redirecturi.ValidateList(appType, s.PostLogoutRedirectURIs, false)); err != nil {
		return err
	}
	method, pkce, err := c.authSettings(s)
	if err != nil {
		return err
	}
	c.TokenEndpointAuthMethod, c.PKCERequired = method, pkce
	c.Name = name
	c.RedirectURIs = slices.Clone(s.RedirectURIs)
	c.PostLogoutRedirectURIs = slices.Clone(s.PostLogoutRedirectURIs)
	if c.PostLogoutRedirectURIs == nil {
		c.PostLogoutRedirectURIs = []string{}
	}
	c.GrantTypes = []string{GrantAuthorizationCode}
	c.AllowedScopes = []string{"openid", "profile", "email"}
	if s.RefreshTokens {
		c.GrantTypes = append(c.GrantTypes, GrantRefreshToken)
		c.AllowedScopes = append(c.AllowedScopes, ScopeOfflineAccess)
	}
	c.ConsentPolicy = s.ConsentPolicy
	if c.ConsentPolicy == "" {
		c.ConsentPolicy = ConsentRequire
	}
	return nil
}

func (c *Client) authSettings(s Settings) (method string, pkce bool, err error) {
	pkce = s.PKCERequired == nil || *s.PKCERequired
	if c.ApplicationType == AppNative {
		switch {
		case s.AuthMethod != "" && s.AuthMethod != AuthNone:
			return "", false, &InvalidSettingsError{Reason: SettingsAuthMethodNative}
		case !pkce:
			return "", false, &InvalidSettingsError{Reason: SettingsPKCENative}
		}
		return AuthNone, true, nil
	}
	switch s.AuthMethod {
	case "":
		return AuthClientSecretBasic, pkce, nil
	case AuthClientSecretBasic, AuthClientSecretPost:
		return s.AuthMethod, pkce, nil
	}
	return "", false, apperror.Invalid("auth_method", apperror.FieldNotAllowed)
}

func redirectError(field string, err error) error {
	if err == nil {
		return nil
	}
	e, ok := errors.AsType[*redirecturi.Error](err)
	if !ok {
		return err
	}
	if e.Index >= 0 {
		field = fmt.Sprintf("%s[%d]", field, e.Index)
	}
	return &InvalidRedirectURIError{Field: field, URI: e.URI, Reason: RedirectURIReason(e.Reason), Canonical: e.Canonical}
}

// CurrentSecret returns the client's current secret, if it has one.
func (c *Client) CurrentSecret() (Secret, bool) {
	for _, s := range c.Secrets {
		if s.Current() {
			return s, true
		}
	}
	return Secret{}, false
}

// SecretScheme mints client secrets: "othra_cs_" and 256 random bits, so secret scanners can find a leaked one.
var SecretScheme = secret.Scheme{Prefix: SecretPrefix} //nolint:gochecknoglobals // the client-secret format, fixed.

// NewSecret returns a fresh plaintext secret, its stored hash and its hint.
func NewSecret() (plaintext, hash, hint string, err error) {
	m, err := SecretScheme.Mint()
	if err != nil {
		return "", "", "", err
	}
	return m.Plaintext, secret.Versioned(m.Plaintext), m.Hint, nil
}

// VerifySecret reports whether presented matches a secret of the client that is live at now; every live secret is compared.
func (c *Client) VerifySecret(presented string, now time.Time) bool {
	return c.UsesSecret() && secret.MatchesAny(c.Secrets, presented, now)
}
