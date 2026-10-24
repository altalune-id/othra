package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/oauth/redirecturi"
)

// The limits on a Client ID Metadata Document client (CIMD -02).
const (
	MaxMetadataURLLength = 512
	// MaxMetadataBytes is the CIMD §8.7 recommended size cap.
	MaxMetadataBytes = 5 << 10
)

const metadataURLPrefix = "https://"

// MetadataReason is the stable cause of a *MetadataError; it goes only to the log.
type MetadataReason string

// The causes of a *MetadataError.
const (
	MetadataFetch        MetadataReason = "fetch"
	MetadataTooLarge     MetadataReason = "too_large"
	MetadataContentType  MetadataReason = "content_type"
	MetadataNotJSON      MetadataReason = "not_json"
	MetadataClientID     MetadataReason = "client_id"
	MetadataClientName   MetadataReason = "client_name"
	MetadataRedirectURIs MetadataReason = "redirect_uris"
	MetadataAuthMethod   MetadataReason = "auth_method"
	MetadataGrantTypes   MetadataReason = "grant_types"
	MetadataURI          MetadataReason = "uri"
)

// MetadataError signals a client metadata document that can't be used; Err is the fetch failure, for the log.
type MetadataError struct {
	Reason MetadataReason
	Err    error
}

func (e *MetadataError) Error() string { return "client: metadata document: " + string(e.Reason) }

func (e *MetadataError) Unwrap() error { return e.Err }

// ToAppError maps MetadataError to SIG011; the reason goes only to the log.
func (e *MetadataError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAuthorizeClientMetadataUnusable, "This app's details can't be used").WithCause(e)
}

// AdmissionReason is the stable cause of a *NotAdmittedError; it goes only to the log.
type AdmissionReason string

// The causes of a *NotAdmittedError.
const (
	AdmissionClientID AdmissionReason = "client_id"
	AdmissionResource AdmissionReason = "resource"
	AdmissionOff      AdmissionReason = "off"
	AdmissionHost     AdmissionReason = "host"
)

// NotAdmittedError signals a metadata document client no resource server of the request admits (ADR-0021 D1, D3).
type NotAdmittedError struct {
	Reason AdmissionReason
}

func (e *NotAdmittedError) Error() string { return "client: not admitted: " + string(e.Reason) }

// IsNotAdmittedError reports whether err's tree contains a *NotAdmittedError.
func IsNotAdmittedError(err error) bool {
	_, ok := errors.AsType[*NotAdmittedError](err)
	return ok
}

// ToAppError maps NotAdmittedError to SIG010, one answer for every miss so the page is no oracle.
func (e *NotAdmittedError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAuthorizeClientNotAdmitted, "This app isn't allowed to sign in").WithCause(e)
}

// IsMetadataError reports whether err's tree contains a *MetadataError.
func IsMetadataError(err error) bool {
	_, ok := AsMetadataError(err)
	return ok
}

// AsMetadataError returns the *MetadataError in err's tree.
func AsMetadataError(err error) (*MetadataError, bool) {
	return errors.AsType[*MetadataError](err)
}

// LoadMetadata fetches and validates the document of clientID and returns how long it may be used.
func LoadMetadata(ctx context.Context, f MetadataFetcher, cfg MetadataConfig, clientID string) (*Metadata, time.Duration, error) {
	limit := MaxMetadataBytes
	if cfg.MaxBytes > 0 {
		limit = min(cfg.MaxBytes, MaxMetadataBytes)
	}
	got, err := f.FetchMetadata(ctx, clientID)
	switch {
	case err != nil:
		return nil, 0, &MetadataError{Reason: MetadataFetch, Err: err}
	case len(got.Body) > limit:
		return nil, 0, &MetadataError{Reason: MetadataTooLarge}
	case !IsJSONMediaType(got.ContentType):
		return nil, 0, &MetadataError{Reason: MetadataContentType}
	}
	m, err := ParseMetadata(clientID, got.Body)
	if err != nil {
		return nil, 0, err
	}
	return m, cfg.Cache.TTL(got.CacheControl), nil
}

// Metadata is the part of a client metadata document Othra keeps.
type Metadata struct {
	ClientID     string
	ClientName   string
	ClientURI    string
	RedirectURIs []string
	Refresh      bool
}

// Host is the host of the client_id URL, the name the consent page shows (CIMD §8.5).
func (m *Metadata) Host() string {
	u, err := url.Parse(m.ClientID)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

type document struct {
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name"`
	ClientURI               string   `json:"client_uri"`
	LogoURI                 string   `json:"logo_uri"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	ClientSecret            *string  `json:"client_secret"`
	ClientSecretExpiresAt   *int64   `json:"client_secret_expires_at"`
}

// IsMetadataURL reports whether a client_id names a metadata document; a registered client_id never starts with https://.
func IsMetadataURL(clientID string) bool {
	return strings.HasPrefix(clientID, metadataURLPrefix)
}

// SECURITY: CIMD §3, §8.6: an https URL with a path, no query, fragment, userinfo or dot segment, on a DNS name that isn't the issuer's.
func ValidateMetadataURL(raw, issuerHost string) error {
	fail := &MetadataError{Reason: MetadataClientID}
	if len(raw) > MaxMetadataURLLength || !IsMetadataURL(raw) || strings.ContainsAny(raw, "?# \t\r\n\\") {
		return fail
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Path == "" || u.Path == "/" {
		return fail
	}
	for seg := range strings.SplitSeq(u.Path, "/") {
		if seg == "." || seg == ".." {
			return fail
		}
	}
	host := u.Hostname()
	if net.ParseIP(host) != nil || !strings.Contains(host, ".") || strings.EqualFold(host, issuerHost) {
		return fail
	}
	return nil
}

// ParseMetadata validates the document fetched from clientID (CIMD §4, MCP registration) and returns what Othra keeps.
func ParseMetadata(clientID string, body []byte) (*Metadata, error) {
	var d document
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(&d); err != nil {
		return nil, &MetadataError{Reason: MetadataNotJSON}
	}
	name := strings.TrimSpace(d.ClientName)
	switch {
	case d.ClientID != clientID:
		return nil, &MetadataError{Reason: MetadataClientID}
	case name == "" || utf8.RuneCountInString(name) > maxNameLen:
		return nil, &MetadataError{Reason: MetadataClientName}
	case redirecturi.ValidateList(redirecturi.Native, d.RedirectURIs, true) != nil:
		return nil, &MetadataError{Reason: MetadataRedirectURIs}
	// NOTE: R-11, RFC 7591 §2: an omitted token_endpoint_auth_method is client_secret_basic, which a CIMD client can't use.
	case d.ClientSecret != nil || d.ClientSecretExpiresAt != nil || d.TokenEndpointAuthMethod != AuthNone:
		return nil, &MetadataError{Reason: MetadataAuthMethod}
	case !subset(d.GrantTypes, GrantAuthorizationCode, GrantRefreshToken) || !subset(d.ResponseTypes, "code"):
		return nil, &MetadataError{Reason: MetadataGrantTypes}
	case !optionalHTTPS(d.ClientURI) || !optionalHTTPS(d.LogoURI):
		return nil, &MetadataError{Reason: MetadataURI}
	}
	return &Metadata{
		ClientID: d.ClientID, ClientName: name, ClientURI: d.ClientURI, RedirectURIs: d.RedirectURIs,
		Refresh: slices.Contains(d.GrantTypes, GrantRefreshToken),
	}, nil
}

func subset(values []string, allowed ...string) bool {
	for _, v := range values {
		if !slices.Contains(allowed, v) {
			return false
		}
	}
	return true
}

func optionalHTTPS(raw string) bool {
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}

// IsJSONMediaType reports whether a Content-Type is application/json or a +json type.
func IsJSONMediaType(raw string) bool {
	mt, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return false
	}
	return mt == "application/json" || strings.HasSuffix(mt, "+json")
}

// MetadataCache bounds how long a fetched document is used (CIMD §5.2).
type MetadataCache struct {
	Min     time.Duration
	Max     time.Duration
	Default time.Duration
}

// TTL returns how long a document is used: max-age within the bounds, Min for no-store or no-cache, else Default (CIMD §5.2).
func (c MetadataCache) TTL(cacheControl string) time.Duration {
	maxAge := time.Duration(-1)
	for directive := range strings.SplitSeq(strings.ToLower(cacheControl), ",") {
		directive = strings.TrimSpace(directive)
		switch {
		case directive == "no-store" || directive == "no-cache":
			return c.Min
		case strings.HasPrefix(directive, "max-age="):
			if secs, err := strconv.Atoi(strings.TrimPrefix(directive, "max-age=")); err == nil && secs >= 0 {
				maxAge = time.Duration(secs) * time.Second
			}
		}
	}
	if maxAge < 0 {
		return c.Default
	}
	return min(max(maxAge, c.Min), c.Max)
}
