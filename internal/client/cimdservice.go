package client

import (
	"context"
	"log/slog"
	"net/url"
	"slices"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/ratelimit"
	"altalune.id/othra/internal/web/layout"
	"altalune.id/othra/nanoid"
)

// MetadataConfig is how a Service fetches and caches metadata documents; IssuerHost is refused as a client_id host.
type MetadataConfig struct {
	Cache      MetadataCache
	IssuerHost string
	// MaxBytes caps a document, at most MaxMetadataBytes; zero means MaxMetadataBytes.
	MaxBytes int
}

// AdmitRequest is a metadata document client at URL asking for the resource server that admits it, with the scopes it may ask for.
type AdmitRequest struct {
	URL              string
	ResourceServerID uuid.UUID
	Scopes           []string
}

// MetadataFetchURLRule bounds fetches of one metadata document: 6 a minute.
func MetadataFetchURLRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "cimd-fetch-url", Rate: 6, Per: time.Minute, Burst: 6}
}

// MetadataFetchHostCeilingRule bounds fetches from one host across every project: 300 a minute.
func MetadataFetchHostCeilingRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "cimd-fetch-host-all", Rate: 300, Per: time.Minute, Burst: 300}
}

// MetadataFetchHostRule bounds fetches from one host: 30 a minute.
func MetadataFetchHostRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "cimd-fetch-host", Rate: 30, Per: time.Minute, Burst: 30}
}

// WithLinks sets where an admitted metadata document client is linked to its resource server, which itself needs this Service.
func (s *Service) WithLinks(links Links) *Service {
	s.links = links
	return s
}

// WithFetchLimits counts metadata document fetches per URL and per host on l.
func (s *Service) WithFetchLimits(l layout.Limits) *Service {
	s.fetchLimits = l
	return s
}

// WithOnMetadataChange sets what runs when a refetched document changes the client's name or redirect URIs (CIMD §8.4).
func (s *Service) WithOnMetadataChange(fn func(ctx context.Context, orgID, projectID, clientID uuid.UUID) error) *Service {
	s.onMetadataChange = fn
	return s
}

// MetadataHost returns the host of a metadata document URL, or *NotAdmittedError when the URL breaks CIMD §3.
func (s *Service) MetadataHost(raw string) (string, error) {
	if err := ValidateMetadataURL(raw, s.meta.IssuerHost); err != nil {
		return "", &NotAdmittedError{Reason: AdmissionClientID}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", &NotAdmittedError{Reason: AdmissionClientID}
	}
	return u.Hostname(), nil
}

// SECURITY: a stored document is used until it expires; then it is fetched again, never used stale.
func (s *Service) Admit(ctx context.Context, orgID, projectID uuid.UUID, req AdmitRequest) (*Client, error) {
	if s.links == nil {
		return nil, s.unexpected(ctx, "client: admit", errNoLinks)
	}
	host, err := s.MetadataHost(req.URL)
	if err != nil {
		return nil, err
	}
	ctx = scope(ctx, orgID, projectID)
	now := s.now().UTC()
	old, err := s.store.ByPublicID(ctx, projectID, req.URL, now)
	switch {
	case IsNotFoundError(err):
		old = nil
	case err != nil:
		return nil, s.errOrReport(ctx, "client: admit", err)
	}
	var fresh *Client
	if old == nil || old.MetadataExpiresAt == nil || !old.MetadataExpiresAt.After(now) {
		if fresh, err = s.fetchShadow(ctx, orgID, projectID, old, req.URL, host, now); err != nil {
			return nil, err
		}
	}
	out := old
	err = s.uow.RunInTx(ctx, func(ctx context.Context) error {
		if fresh != nil {
			if out, err = s.store.SaveShadow(ctx, fresh); err != nil {
				return err
			}
			if changed(old, out) && s.onMetadataChange != nil {
				if err := s.onMetadataChange(ctx, orgID, projectID, out.ID); err != nil {
					return err
				}
			}
		}
		return s.links.LinkMetadataClient(ctx, orgID, projectID, out.ID, req.ResourceServerID, req.Scopes)
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "client: admit", err)
	}
	return out, nil
}

func (s *Service) fetchShadow(
	ctx context.Context, orgID, projectID uuid.UUID, old *Client, metadataURL, host string, now time.Time,
) (*Client, error) {
	// SECURITY: per project, so one tenant can't spend another's budget; the host ceiling guards the fetcher itself.
	limits := []struct {
		rule    ratelimit.Rule
		subject string
	}{
		{MetadataFetchURLRule(), ratelimit.ProjectSubject(projectID, ratelimit.URLSubject(metadataURL))},
		{MetadataFetchHostRule(), ratelimit.ProjectSubject(projectID, ratelimit.HostSubject(host))},
		{MetadataFetchHostCeilingRule(), ratelimit.HostSubject(host)},
	}
	for _, l := range limits {
		if err := s.fetchLimits.Allow(ctx, ratelimit.Key(l.rule, l.subject), l.rule); err != nil {
			return nil, err
		}
	}
	m, ttl, err := LoadMetadata(ctx, s.fetcher, s.meta, metadataURL)
	if err != nil {
		reason := MetadataFetch
		if me, ok := AsMetadataError(err); ok {
			reason = me.Reason
		}
		s.log.LogAttrs(ctx, slog.LevelWarn, "client: metadata document refused",
			slog.String("url", metadataURL), slog.String("reason", string(reason)))
		return nil, err
	}
	c := &Client{ID: uuid.New(), OrgID: orgID, ProjectID: projectID, CreatedAt: now, Secrets: []Secret{}}
	if old != nil {
		c.ID, c.PublicID, c.CreatedAt = old.ID, old.PublicID, old.CreatedAt
	} else if c.PublicID, err = nanoid.New(publicIDLen); err != nil {
		return nil, s.unexpected(ctx, "client: admit", err)
	}
	c.Preset(KindCLI)
	expires := now.Add(ttl)
	c.Name, c.RedirectURIs, c.PostLogoutRedirectURIs, c.ConsentPolicy = m.ClientName, m.RedirectURIs, []string{}, ConsentRequire
	c.GrantTypes, c.AllowedScopes = []string{GrantAuthorizationCode}, []string{"openid", "profile", "email"}
	if m.Refresh {
		c.GrantTypes, c.AllowedScopes = append(c.GrantTypes, GrantRefreshToken), append(c.AllowedScopes, ScopeOfflineAccess)
	}
	c.MetadataURL, c.ClientURI, c.MetadataFetchedAt, c.MetadataExpiresAt, c.UpdatedAt = metadataURL, m.ClientURI, &now, &expires, now
	s.log.LogAttrs(ctx, slog.LevelInfo, "client: metadata document fetched", slog.String("url", metadataURL), slog.Duration("ttl", ttl))
	return c, nil
}

// NOTE: CIMD §8.4: a new name or redirect URIs drop what end users approved, so they see the consent page again.
func changed(old, now *Client) bool {
	return old != nil && (old.Name != now.Name || !slices.Equal(old.RedirectURIs, now.RedirectURIs))
}

// ShadowKeep is how long a shadow client outlives its document's expiry while it holds no consent and no live grant.
const ShadowKeep = 7 * 24 * time.Hour

// PruneShadows deletes the project's unused shadow clients whose document expired over ShadowKeep ago.
func (s *Service) PruneShadows(ctx context.Context, orgID, projectID uuid.UUID) (int, error) {
	ctx = scope(ctx, orgID, projectID)
	n, err := s.store.PruneShadows(ctx, projectID, s.now().UTC().Add(-ShadowKeep))
	if err != nil {
		return 0, s.errOrReport(ctx, "client: prune shadows", err)
	}
	return n, nil
}
