// Package resourceserver holds a project's resource servers, their scopes, their links to OAuth clients and their RFC 9728 verification.
package resourceserver

import (
	"net"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// The limits on a resource server.
const (
	MaxIdentifierLength = 512
	MaxScopes           = 64
	MaxScopeLength      = 128
	// FailureLimit is how many scheduled checks in a row may fail before a verified server becomes failed.
	FailureLimit = 3
	// MaxClientHosts bounds the hosts a server admits metadata document clients from.
	MaxClientHosts = 64
	// MetadataPath is the RFC 9728 §3 well-known path, inserted between the host and the path of the identifier.
	MetadataPath = "/.well-known/oauth-protected-resource"
)

// State is where a resource server's RFC 9728 verification stands.
type State string

// The verification states: pending never passed a check; failed got a mismatching document or failed FailureLimit checks once verified.
const (
	StatePending  State = "pending"
	StateVerified State = "verified"
	StateFailed   State = "failed"
)

// FailureReason is the stable cause of a failed check.
type FailureReason string

// The causes of a failed check.
const (
	FailureNotFound         FailureReason = "not_found"
	FailureStatus           FailureReason = "status"
	FailureRedirect         FailureReason = "redirect"
	FailureNotJSON          FailureReason = "not_json"
	FailureTooLarge         FailureReason = "too_large"
	FailureResourceMismatch FailureReason = "resource_mismatch"
	FailureIssuerMissing    FailureReason = "issuer_missing"
	FailureTimeout          FailureReason = "timeout"
	FailureNetwork          FailureReason = "network"
	FailureBlocked          FailureReason = "blocked"
)

// Failure is why one check failed.
type Failure struct {
	Reason    FailureReason `json:"reason"`
	Status    int32         `json:"status,omitempty"`
	Resource  string        `json:"resource,omitempty"`
	CheckedAt time.Time     `json:"checked_at"`
}

// ResourceServer is an API or MCP endpoint that accepts this project's access tokens.
type ResourceServer struct {
	ID                  uuid.UUID
	PublicID            string
	OrgID               uuid.UUID
	ProjectID           uuid.UUID
	Name                string
	Identifier          string
	Scopes              []string
	State               State
	VerifiedAt          *time.Time
	LastCheckedAt       *time.Time
	LastError           *Failure
	ConsecutiveFailures int32
	Trusted             bool
	TrustedAt           *time.Time
	TrustedBy           uuid.UUID
	Admission           Admission
	// Descriptions are what each scope lets an app do, shown on the consent page; a scope may have none.
	Descriptions map[string]string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Pass records a check that passed at now.
func (r *ResourceServer) Pass(now time.Time) {
	r.State = StateVerified
	r.VerifiedAt = &now
	r.LastCheckedAt = &now
	r.LastError = nil
	r.ConsecutiveFailures = 0
}

// Fail records a scheduled check that failed: a mismatch fails the server at once, else a verified one fails after FailureLimit in a row.
func (r *ResourceServer) Fail(f Failure) {
	at := f.CheckedAt
	r.LastCheckedAt = &at
	r.LastError = &f
	r.ConsecutiveFailures++
	if f.mismatch() || (r.State == StateVerified && r.ConsecutiveFailures >= FailureLimit) {
		r.State = StateFailed
	}
}

// NOTE: RFC 9728 §3.3: a document that names another resource MUST NOT be used.
func (f Failure) mismatch() bool {
	return f.Reason == FailureResourceMismatch || f.Reason == FailureIssuerMissing
}

// Admissible reports whether tokens may be minted for the server; M6 /authorize, /token and CIMD admission call only this.
func (r *ResourceServer) Admissible(trustAvailable bool) bool {
	return r.State == StateVerified || (r.Trusted && trustAvailable)
}

// Admission is whether a server admits Client ID Metadata Document clients, from which hosts, with which scopes (nil: the catalog).
type Admission struct {
	Accept bool
	Hosts  []string
	Scopes []string
}

// SECURITY: M2, ADR-0021 D1: only an admissible server that accepts metadata document clients, from a listed host (none listed: any).
func (r *ResourceServer) AdmitsMetadataClient(host string, trustAvailable bool) bool {
	if !r.Admission.Accept || !r.Admissible(trustAvailable) {
		return false
	}
	return len(r.Admission.Hosts) == 0 || slices.Contains(r.Admission.Hosts, strings.ToLower(host))
}

// MetadataClientScopes returns the scopes a metadata document client may ask for: the admission pin, else the catalog.
func (r *ResourceServer) MetadataClientScopes() []string {
	if r.Admission.Scopes != nil {
		return slices.Clone(r.Admission.Scopes)
	}
	return slices.Clone(r.Scopes)
}

// ValidateClientHosts checks admission hosts: at most MaxClientHosts lower-case DNS names, no IP literal, none twice.
func ValidateClientHosts(hosts []string) error {
	if len(hosts) > MaxClientHosts {
		return &InvalidClientHostError{Reason: HostTooMany}
	}
	seen := make(map[string]bool, len(hosts))
	for _, h := range hosts {
		switch {
		case net.ParseIP(h) != nil:
			return &InvalidClientHostError{Host: h, Reason: HostIPLiteral}
		case !dnsName(h):
			return &InvalidClientHostError{Host: h, Reason: HostFormat}
		case seen[h]:
			return &InvalidClientHostError{Host: h, Reason: HostDuplicate}
		}
		seen[h] = true
	}
	return nil
}

func dnsName(h string) bool {
	if len(h) > 253 || !strings.Contains(h, ".") {
		return false
	}
	for label := range strings.SplitSeq(h, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := range len(label) {
			if c := label[i]; (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}
	return true
}

// Trust marks the server trusted without verification by userID at now.
func (r *ResourceServer) Trust(userID uuid.UUID, now time.Time) {
	r.Trusted = true
	r.TrustedAt = &now
	r.TrustedBy = userID
}

// Untrust returns a trusted server to pending; an http identifier can't leave trust.
func (r *ResourceServer) Untrust() error {
	if IsLoopbackHTTP(r.Identifier) {
		return &InvalidIdentifierError{Identifier: r.Identifier, Reason: IdentifierNeedsTrust}
	}
	r.Trusted, r.TrustedAt, r.TrustedBy = false, nil, uuid.Nil
	r.State, r.VerifiedAt, r.LastCheckedAt, r.LastError, r.ConsecutiveFailures = StatePending, nil, nil, nil, 0
	return nil
}

// ValidateIdentifier checks an identifier: a canonical https URL with no query, fragment or userinfo, or an http loopback URL.
func ValidateIdentifier(raw string) error {
	fail := func(reason IdentifierReason) error {
		return &InvalidIdentifierError{Identifier: raw, Reason: reason}
	}
	if len(raw) > MaxIdentifierLength {
		return fail(IdentifierTooLong)
	}
	u, err := url.Parse(raw)
	if err != nil || strings.ContainsAny(raw, " \t\r\n") || u.Opaque != "" || u.Host == "" {
		return fail(IdentifierUnparsable)
	}
	switch {
	case u.User != nil:
		return fail(IdentifierUserinfo)
	case strings.Contains(raw, "#"):
		return fail(IdentifierFragment)
	case u.RawQuery != "" || u.ForceQuery:
		return fail(IdentifierQuery)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && (scheme != "http" || !loopbackHost(u.Hostname())) {
		return fail(IdentifierHTTPSRequired)
	}
	if c := canonical(u); c != raw {
		return &InvalidIdentifierError{Identifier: raw, Reason: IdentifierNotCanonical, Canonical: c}
	}
	return nil
}

// IsLoopbackHTTP reports whether identifier is an http URL on a loopback host, which only trust allows.
func IsLoopbackHTTP(identifier string) bool {
	u, err := url.Parse(identifier)
	return err == nil && u.Scheme == "http" && loopbackHost(u.Hostname())
}

func loopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

func canonical(u *url.URL) string {
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port := u.Port(); port != "" && port != defaultPort(scheme) {
		host += ":" + port
	}
	p := normalizePercentEncoding(u.ResolveReference(&url.URL{}).EscapedPath())
	if p == "/" {
		p = ""
	}
	return scheme + "://" + host + p
}

func defaultPort(scheme string) string {
	if scheme == "http" {
		return "80"
	}
	return "443"
}

func normalizePercentEncoding(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '%' || i+2 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		hi, hiOK := unhex(s[i+1])
		lo, loOK := unhex(s[i+2])
		if !hiOK || !loOK {
			b.WriteByte(s[i])
			continue
		}
		if c := hi<<4 | lo; unreserved(c) {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteString(strings.ToUpper(s[i+1 : i+3]))
		}
		i += 2
	}
	return b.String()
}

func unhex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func unreserved(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~'
}

// MetadataURL is where the RFC 9728 metadata of an https identifier lives.
func MetadataURL(identifier string) string {
	u, err := url.Parse(identifier)
	if err != nil || u.Host == "" {
		return ""
	}
	return "https://" + u.Host + MetadataPath + strings.TrimSuffix(u.EscapedPath(), "/")
}

// ValidateScopes checks a scope list: 1 to MaxScopes RFC 6749 §3.3 tokens, none reserved and none twice.
func ValidateScopes(scopes []string) error {
	switch {
	case len(scopes) == 0:
		return &InvalidScopeError{Reason: ScopeFormat}
	case len(scopes) > MaxScopes:
		return &InvalidScopeError{Reason: ScopeTooMany}
	}
	seen := make(map[string]bool, len(scopes))
	for _, s := range scopes {
		switch {
		case !scopeToken(s):
			return &InvalidScopeError{Scope: s, Reason: ScopeFormat}
		case reservedScope(s):
			return &InvalidScopeError{Scope: s, Reason: ScopeReserved}
		case seen[s]:
			return &InvalidScopeError{Scope: s, Reason: ScopeDuplicate}
		}
		seen[s] = true
	}
	return nil
}

func reservedScope(s string) bool {
	switch s {
	case "openid", "profile", "email", "offline_access":
		return true
	}
	return false
}

func scopeToken(s string) bool {
	if s == "" || len(s) > MaxScopeLength {
		return false
	}
	for i := range len(s) {
		b := s[i]
		if b != 0x21 && (b < 0x23 || b > 0x5B) && (b < 0x5D || b > 0x7E) {
			return false
		}
	}
	return true
}

// MaxDescriptionLength bounds a scope description, in characters.
const MaxDescriptionLength = 120

// ValidateDescriptions checks scope descriptions: each names one of scopes and is 1 to MaxDescriptionLength characters.
func ValidateDescriptions(scopes []string, descriptions map[string]string) error {
	for scope, d := range descriptions {
		if n := utf8.RuneCountInString(strings.TrimSpace(d)); !slices.Contains(scopes, scope) || n == 0 || n > MaxDescriptionLength {
			return &InvalidScopeError{Scope: scope, Reason: ScopeDescription}
		}
	}
	return nil
}

// LinkScopes checks the scopes a link allows against the server's scopes and returns them without repeats.
func (r *ResourceServer) LinkScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return nil, &InvalidLinkScopesError{Reason: LinkScopesEmpty}
	}
	out := make([]string, 0, len(scopes))
	for _, s := range scopes {
		if !slices.Contains(r.Scopes, s) {
			return nil, &InvalidLinkScopesError{Scope: s, Reason: LinkScopesNotInCatalog}
		}
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out, nil
}

// Removed returns the scopes of r that next no longer has.
func (r *ResourceServer) Removed(next []string) []string {
	var out []string
	for _, s := range r.Scopes {
		if !slices.Contains(next, s) {
			out = append(out, s)
		}
	}
	return out
}

// Link is a client's permission to ask for some of a resource server's scopes.
type Link struct {
	OrgID          uuid.UUID
	ProjectID      uuid.UUID
	ClientID       uuid.UUID
	Client         ClientRef
	ResourceServer *ResourceServer
	Scopes         []string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ClientRef names an OAuth client of the project.
type ClientRef struct {
	ID       uuid.UUID
	PublicID string
	Name     string
	// MetadataURL and MetadataFetchedAt are set on a metadata document client, which only its document changes.
	MetadataURL       string
	MetadataFetchedAt *time.Time
}
