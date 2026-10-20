// Package client holds a project's OAuth clients: their redirect URIs, secrets and consent policy.
package client

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/oauth/redirecturi"
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

const (
	maxNameLen  = 100
	secretBytes = 32
	hashPrefix  = "sha256$"
	hintLen     = 4
)

// Client is a registered OAuth client; its PublicID is the OAuth client_id.
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
	CreatedAt               time.Time
	UpdatedAt               time.Time
	Secrets                 []Secret
}

// Secret is one of a client's live secrets; the plaintext is never kept.
type Secret struct {
	ID        uuid.UUID
	PublicID  string
	Hint      string
	CreatedAt time.Time
	ExpiresAt *time.Time
}

// Current reports whether s is the client's current secret rather than an old one in its grace period.
func (s Secret) Current() bool { return s.ExpiresAt == nil }

// Settings are the fields of a client a person sets.
type Settings struct {
	Name                   string
	RedirectURIs           []string
	PostLogoutRedirectURIs []string
	RefreshTokens          bool
	ConsentPolicy          ConsentPolicy
}

// Preset sets the protocol fields kind fixes: web is confidential with client_secret_basic, CLI is a public native client.
func (c *Client) Preset(kind Kind) {
	if kind == KindCLI {
		c.ClientType, c.ApplicationType, c.TokenEndpointAuthMethod = TypePublic, AppNative, AuthNone
		return
	}
	c.ClientType, c.ApplicationType, c.TokenEndpointAuthMethod = TypeConfidential, AppWeb, AuthClientSecretBasic
}

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
	case AuthClientSecretBasic:
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

// NewSecret returns a fresh plaintext secret, its stored hash and its hint.
func NewSecret() (plaintext, hash, hint string, err error) {
	raw := make([]byte, secretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", "", err
	}
	plaintext = SecretPrefix + base64.RawURLEncoding.EncodeToString(raw)
	return plaintext, HashSecret(plaintext), plaintext[len(plaintext)-hintLen:], nil
}

// HashSecret returns the stored form of a secret: "sha256$" and its hex SHA-256.
func HashSecret(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hashPrefix + hex.EncodeToString(sum[:])
}

// SecretMatches compares a presented secret with a stored hash in constant time.
func SecretMatches(hash, presented string) bool {
	return subtle.ConstantTimeCompare([]byte(hash), []byte(HashSecret(presented))) == 1
}
