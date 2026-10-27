package boot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/install"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/oauth/redirecturi"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/upstream"
)

// NOTE: the one source of the console client's URIs; install seeds them and every boot reconciles them.
func consoleURIs(c config.HTTPConfig) install.Console {
	return install.Console{RedirectURI: publicBase(c) + consoleCallback, PostLogoutURI: newConsolePages(c).root()}
}

// NOTE: the console SPA's routes (ui/console/app/pages), the one builder of every server-side link into the console.
type consolePages struct{ base string }

func newConsolePages(c config.HTTPConfig) consolePages {
	return consolePages{base: publicBase(c) + "/console"}
}

func (p consolePages) root() string { return p.base + "/" }

func (p consolePages) client(orgSlug, projectSlug, clientPublicID string) string {
	return p.base + "/o/" + url.PathEscape(orgSlug) + "/projects/" + url.PathEscape(projectSlug) + "/clients/" + url.PathEscape(clientPublicID)
}

const consoleCallback = "/oauth/callback"

// SECURITY: the console client's redirect URI comes from http.baseURL, so the URL must pass the web redirect rules before boot.
func checkPublicBase(c config.HTTPConfig) error {
	err := redirecturi.Validate(redirecturi.Web, consoleURIs(c).RedirectURI)
	if err == nil {
		return nil
	}
	base := publicBase(c)
	reason := fmt.Sprintf(
		"%q can't be the console's address: use https://, or http:// on localhost, 127.0.0.1 or [::1] with a port other than 80", base)
	if re, ok := errors.AsType[*redirecturi.Error](err); ok {
		reason += " (" + string(re.Reason) + ")"
		if re.Reason == redirecturi.NotCanonical && re.Canonical != "" {
			want := strings.TrimSuffix(re.Canonical, strings.TrimSuffix(c.BasePath, "/")+consoleCallback)
			reason = fmt.Sprintf("%q isn't in canonical form (%s): use %q", strings.TrimSuffix(c.BaseURL, "/"), re.Reason, want)
		}
	}
	return &config.InvalidSettingError{Key: "http.baseURL", Reason: reason}
}

func buildInstall(cfg config.Config, s *Services, uow db.Transactor) *install.Service {
	return install.NewService(install.Deps{
		Transactor: uow, Users: s.Users, Orgs: s.Orgs, Members: s.Members, Projects: s.Projects, Clients: s.Clients,
		Invites: s.Invites, Terms: s.Legal, Console: consoleURIs(cfg.HTTP), Passwords: cfg.PasswordsOn(),
		Seed: seedUpstreams(cfg.Upstreams, s.Upstreams),
	})
}

func consoleProject(ctx context.Context, s *Services) (*org.Org, *project.Project, error) {
	sys, err := s.Orgs.System(ctx)
	if org.IsNotFoundError(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	p, err := s.Projects.BySlug(ctx, sys.ID, project.SystemSlug)
	if project.IsNotFoundError(err) || err == nil && !p.IsSystem() {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return sys, p, nil
}

// SECURITY: every boot puts the console client's URIs back to the ones derived from http.baseURL, so a URL change never locks admins out.
func reconcileConsole(ctx context.Context, c config.HTTPConfig, s *Services, log *slog.Logger) error {
	sys, p, err := consoleProject(ctx, s)
	if err != nil || p == nil {
		return wrapBoot("console project", err)
	}
	uris := consoleURIs(c)
	changed, err := s.Clients.ReconcileSystem(ctx, sys.ID, p.ID, uris.RedirectURI, uris.PostLogoutURI)
	if err != nil {
		return wrapBoot("reconcile the console client", err)
	}
	if changed {
		log.Info("boot: the console client's URIs now follow http.baseURL", slog.String("redirect_uri", uris.RedirectURI))
	}
	return nil
}

// NOTE: upstreams.google.* seeds /p/system once at install; afterwards the console owns the row, and boot only says so.
func logUpstreamDrift(ctx context.Context, cfg config.UpstreamsConfig, s *Services, log *slog.Logger) error {
	if cfg.Google.ClientID == "" {
		return nil
	}
	sys, p, err := consoleProject(ctx, s)
	if err != nil || p == nil {
		return wrapBoot("console project", err)
	}
	list, err := s.Upstreams.List(ctx, sys.ID, p.ID)
	if err != nil {
		return wrapBoot("console upstream IdPs", err)
	}
	for _, u := range list {
		if u.Kind == upstream.KindGoogle && u.ClientID == cfg.Google.ClientID {
			return nil
		}
	}
	log.Warn("boot: upstreams.google differs from /p/system's Google upstream IdP; the console's setting is used",
		slog.String("upstream", string(upstream.KindGoogle)))
	return nil
}

func wrapBoot(what string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("boot: %s: %w", what, err)
}
