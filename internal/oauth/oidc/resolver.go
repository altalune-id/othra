package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/ratelimit"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/web/layout"
)

const cacheTTL = 30 * time.Second

// ErrUnknownIssuer is an unknown, reserved or malformed issuer handle; every surface answers it with one identical not-found.
var ErrUnknownIssuer = errors.New("oidc: unknown issuer")

// LookupLimitedError refuses a cache-missing issuer lookup over LookupIPRule.
type LookupLimitedError struct{ RetryAfter int }

func (*LookupLimitedError) Error() string { return "oidc: issuer lookup rate limited" }

// LookupIPRule limits issuer lookups that miss the cache per client IP: 120 a minute, 60 at once.
func LookupIPRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "issuer-lookup-ip", Rate: 120, Per: time.Minute, Burst: 60}
}

// Resolver turns a request's {handle} into its issuer through a 30 s cache shared by the issuer documents and the hosted pages.
type Resolver struct {
	issuers    Issuers
	keys       Publisher
	publicBase string
	unexpected apperror.UnexpectedFunc
	limiter    ratelimit.Limiter
	hops       int
	cache      *issuerCache
	now        func() time.Time
	cimd       CIMDAdmissions
}

// NewResolver builds the Resolver; publicBase is the scheme, host and base path every issuer URL starts with.
func NewResolver(issuers Issuers, keys Publisher, publicBase string, unexpected apperror.UnexpectedFunc) *Resolver {
	return &Resolver{
		issuers: issuers, keys: keys, publicBase: publicBase, unexpected: unexpected, limiter: ratelimit.Disabled(),
		cache: &issuerCache{ttl: cacheTTL, entries: map[string]*issuerEntry{}}, now: time.Now,
	}
}

// WithLimiter limits cache-missing lookups per client IP, read through trustedHops proxies.
func (s *Resolver) WithLimiter(l ratelimit.Limiter, trustedHops int) *Resolver {
	s.limiter, s.hops = l, trustedHops
	return s
}

// WithCIMD reads whether each project admits CIMD clients, for client_id_metadata_document_supported.
func (s *Resolver) WithCIMD(c CIMDAdmissions) *Resolver {
	s.cimd = c
	return s
}

// SetClock replaces the cache's clock, for tests.
func (s *Resolver) SetClock(now func() time.Time) { s.now = now }

// Forget drops a project's cached issuer, after its keys or handle change.
func (s *Resolver) Forget(projectID uuid.UUID) { s.cache.forget(projectID) }

// Resolve returns the issuer of r's {handle}: ErrUnknownIssuer, a *LookupLimitedError, or another error the caller answers as unavailable.
func (s *Resolver) Resolve(r *http.Request) (*project.Issuer, error) {
	e, err := s.entry(r)
	if err != nil {
		return nil, err
	}
	return e.issuer, nil
}

// SECURITY: an unknown, reserved or malformed handle are the same ErrUnknownIssuer, and only a well-formed miss reaches the database.
func (s *Resolver) entry(r *http.Request) (*issuerEntry, error) {
	handle := r.PathValue("handle")
	if project.ValidateIssuerHandle(handle) != nil {
		return nil, ErrUnknownIssuer
	}
	now := s.now()
	if e := s.cache.get(handle, now); e != nil {
		return e, nil
	}
	if retry, limited := s.limited(r); limited {
		return nil, &LookupLimitedError{RetryAfter: retry}
	}
	e, err := s.load(r.Context(), handle)
	if err != nil {
		return nil, err
	}
	s.cache.put(handle, e, now)
	return e, nil
}

func (s *Resolver) load(ctx context.Context, handle string) (*issuerEntry, error) {
	i, err := s.issuers.ResolveIssuer(ctx, handle)
	if project.IsNotFoundError(err) {
		return nil, ErrUnknownIssuer
	}
	if err != nil {
		return nil, err
	}
	scoped := tenant.Into(ctx, tenant.Context{OrgID: i.OrgID, ProjectID: i.ProjectID})
	jwks, err := s.keys.JWKS(scoped, i.ProjectID)
	if err != nil {
		return nil, err
	}
	cimd := false
	if s.cimd != nil {
		if cimd, err = s.cimd.AcceptsCIMD(scoped, i.OrgID, i.ProjectID); err != nil {
			return nil, err
		}
	}
	discovery, err := json.Marshal(Discovery(project.IssuerURL(s.publicBase, i.Handle), cimd))
	if err != nil {
		return nil, err
	}
	return &issuerEntry{issuer: i, discovery: newDocument(discovery), jwks: newDocument(jwks)}, nil
}

// SECURITY: a broken limiter lets lookups through, so a Redis outage never takes every issuer's keys offline.
func (s *Resolver) limited(r *http.Request) (int, bool) {
	rule := LookupIPRule()
	res, err := s.limiter.Allow(r.Context(), ratelimit.Key(rule, ratelimit.IPSubject(layout.ClientIP(r, s.hops))), rule)
	if err != nil {
		if s.unexpected != nil {
			_ = s.unexpected(r.Context(), "oidc: rate limit", err)
		}
		return 0, false
	}
	if res.Allowed {
		return 0, false
	}
	return res.RetryAfterSeconds(), true
}
