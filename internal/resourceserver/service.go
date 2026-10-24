package resourceserver

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/authz"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/nanoid"
)

// VerifyCooldown is how long a manual check waits after the last one of the same server.
const VerifyCooldown = 10 * time.Second

const (
	publicIDLen  = 21
	recheckBatch = 100
)

// Service registers resource servers, links clients to them and verifies them.
type Service struct {
	store      Store
	uow        db.Transactor
	issuers    Issuers
	people     People
	clients    Clients
	checker    Checker
	cfg        Config
	unexpected apperror.UnexpectedFunc
	now        func() time.Time
	log        *slog.Logger

	mu     sync.Mutex
	manual map[uuid.UUID]time.Time

	onAdmissionChange func(projectID uuid.UUID)
}

// Deps are the Service's collaborators.
type Deps struct {
	Store      Store
	Transactor db.Transactor
	Issuers    Issuers
	People     People
	Clients    Clients
	Checker    Checker
	Unexpected apperror.UnexpectedFunc
}

// NewService builds the Service.
func NewService(d Deps, cfg Config) *Service {
	return &Service{
		store: d.Store, uow: d.Transactor, issuers: d.Issuers, people: d.People, clients: d.Clients, checker: d.Checker, cfg: cfg,
		unexpected: d.Unexpected, now: time.Now, log: slog.New(slog.DiscardHandler), manual: map[uuid.UUID]time.Time{},
	}
}

// WithLogger sets where trust and verification changes are logged.
func (s *Service) WithLogger(log *slog.Logger) *Service {
	s.log = log
	return s
}

// TrustAvailable reports whether this deployment allows trust without verification.
func (s *Service) TrustAvailable() bool { return s.cfg.TrustAvailable }

func scope(ctx context.Context, orgID, projectID uuid.UUID) context.Context {
	return tenant.WithProject(tenant.WithOrg(ctx, orgID), projectID)
}

// CreateRequest is the input to Create.
type CreateRequest struct {
	OrgID      uuid.UUID
	ProjectID  uuid.UUID
	Name       string
	Identifier string
	Scopes     []string
	// Trusted skips verification, as SetTrust does; an http loopback identifier needs it.
	Trusted bool
	// Person is the signed-in person, or uuid.Nil for a key; only a person may set Trusted.
	Person uuid.UUID
	// Descriptions are optional, one per scope.
	Descriptions map[string]string
}

// Create registers a resource server; the project's first one locks its issuer handle in the same unit of work.
func (s *Service) Create(ctx context.Context, req CreateRequest) (*ResourceServer, error) {
	ctx = scope(ctx, req.OrgID, req.ProjectID)
	if err := ValidateIdentifier(req.Identifier); err != nil {
		return nil, err
	}
	if err := ValidateScopes(req.Scopes); err != nil {
		return nil, err
	}
	if err := ValidateDescriptions(req.Scopes, req.Descriptions); err != nil {
		return nil, err
	}
	publicID, err := nanoid.New(publicIDLen)
	if err != nil {
		return nil, s.unexpected(ctx, "resourceserver: create", err)
	}
	now := s.now().UTC()
	r := &ResourceServer{
		ID: uuid.New(), PublicID: publicID, OrgID: req.OrgID, ProjectID: req.ProjectID, Name: strings.TrimSpace(req.Name),
		Identifier: req.Identifier, Scopes: req.Scopes, State: StatePending, CreatedAt: now, UpdatedAt: now,
		Descriptions: req.Descriptions,
	}
	switch {
	case req.Trusted && !s.cfg.TrustAvailable:
		return nil, &TrustSelfHostedOnlyError{}
	case req.Trusted && req.Person == uuid.Nil:
		return nil, &authz.ForbiddenError{Permission: authz.PersonOnly}
	case req.Trusted:
		r.Trust(req.Person, now)
	case IsLoopbackHTTP(req.Identifier):
		return nil, &InvalidIdentifierError{Identifier: req.Identifier, Reason: IdentifierNeedsTrust}
	}
	err = s.uow.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.issuers.LockIssuerHandle(ctx, req.ProjectID); err != nil {
			return err
		}
		return s.store.Insert(ctx, r)
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "resourceserver: create", err)
	}
	if r.Trusted {
		s.logTrust(ctx, r, true, req.Person)
	}
	return r, nil
}

// List returns the project's resource servers.
func (s *Service) List(ctx context.Context, orgID, projectID uuid.UUID) ([]*ResourceServer, error) {
	ctx = scope(ctx, orgID, projectID)
	list, err := s.store.List(ctx, projectID)
	if err != nil {
		return nil, s.errOrReport(ctx, "resourceserver: list", err)
	}
	return list, nil
}

// Get returns the project's resource server publicID.
func (s *Service) Get(ctx context.Context, orgID, projectID uuid.UUID, publicID string) (*ResourceServer, error) {
	ctx = scope(ctx, orgID, projectID)
	r, err := s.store.ByPublicID(ctx, projectID, publicID)
	if err != nil {
		return nil, s.errOrReport(ctx, "resourceserver: get", err)
	}
	return r, nil
}

// Update replaces a server's name, scopes and their descriptions; removing a scope a link or the admission allows is refused.
func (s *Service) Update(
	ctx context.Context, orgID, projectID uuid.UUID, publicID, name string, scopes []string, descriptions map[string]string,
) (*ResourceServer, error) {
	ctx = scope(ctx, orgID, projectID)
	if err := ValidateScopes(scopes); err != nil {
		return nil, err
	}
	if err := ValidateDescriptions(scopes, descriptions); err != nil {
		return nil, err
	}
	var out *ResourceServer
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		r, err := s.store.Lock(ctx, projectID, publicID)
		if err != nil {
			return err
		}
		if removed := r.Removed(scopes); len(removed) > 0 {
			if err := s.refuseRemoval(ctx, r, removed); err != nil {
				return err
			}
		}
		r.Name, r.Scopes, r.Descriptions, r.UpdatedAt = strings.TrimSpace(name), scopes, descriptions, s.now().UTC()
		out = r
		return s.store.Update(ctx, r)
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "resourceserver: update", err)
	}
	return out, nil
}

func (s *Service) refuseRemoval(ctx context.Context, r *ResourceServer, removed []string) error {
	for _, scope := range removed {
		if slices.Contains(r.Admission.Scopes, scope) {
			return &ScopePinnedError{Scope: scope, CIMD: true}
		}
	}
	links, err := s.links(ctx, r.OrgID, r.ProjectID, uuid.Nil, r.ID)
	if err != nil {
		return err
	}
	for _, scope := range removed {
		var clients []string
		for _, l := range links {
			for _, pinned := range l.Scopes {
				if pinned == scope && l.Client.MetadataURL == "" {
					clients = append(clients, l.Client.Name)
				}
			}
		}
		if len(clients) > 0 {
			return &ScopePinnedError{Scope: scope, Clients: clients}
		}
	}
	return nil
}

// Delete removes a server; it is refused while clients are linked to it.
func (s *Service) Delete(ctx context.Context, orgID, projectID uuid.UUID, publicID string) error {
	ctx = scope(ctx, orgID, projectID)
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		r, err := s.store.Lock(ctx, projectID, publicID)
		if err != nil {
			return err
		}
		links, err := s.links(ctx, orgID, projectID, uuid.Nil, r.ID)
		if err != nil {
			return err
		}
		var names []string
		for _, l := range links {
			if l.Client.MetadataURL == "" {
				names = append(names, l.Client.Name)
			}
		}
		if len(names) > 0 {
			return &LinkedError{Clients: names}
		}
		return s.store.Delete(ctx, projectID, r.ID)
	})
	if err != nil {
		return s.errOrReport(ctx, "resourceserver: delete", err)
	}
	s.admissionChanged(projectID)
	return nil
}

// Verify checks a server now: a pass verifies it; a failure is returned and changes nothing.
func (s *Service) Verify(ctx context.Context, orgID, projectID uuid.UUID, publicID string) (*ResourceServer, *Failure, error) {
	ctx = scope(ctx, orgID, projectID)
	r, err := s.store.ByPublicID(ctx, projectID, publicID)
	if err != nil {
		return nil, nil, s.errOrReport(ctx, "resourceserver: verify", err)
	}
	if IsLoopbackHTTP(r.Identifier) {
		return nil, nil, &InvalidIdentifierError{Identifier: r.Identifier, Reason: IdentifierHTTPSRequired}
	}
	if err := s.throttle(r.ID); err != nil {
		return nil, nil, err
	}
	project, err := s.issuers.Project(ctx, projectID)
	if err != nil {
		return nil, nil, s.errOrReport(ctx, "resourceserver: verify", err)
	}
	if f := s.checker.Check(ctx, r.Identifier, project.Issuer); f != nil {
		return r, f, nil
	}
	var out *ResourceServer
	var before State
	err = s.uow.RunInTx(ctx, func(ctx context.Context) error {
		locked, err := s.store.Lock(ctx, projectID, publicID)
		if err != nil {
			return err
		}
		before = locked.State
		locked.Pass(s.now().UTC())
		locked.UpdatedAt = *locked.LastCheckedAt
		out = locked
		return s.store.Update(ctx, locked)
	})
	if err != nil {
		return nil, nil, s.errOrReport(ctx, "resourceserver: verify", err)
	}
	if before != out.State {
		s.admissionChanged(projectID)
	}
	s.logState(ctx, project.PublicID, out, before, "manual")
	return out, nil, nil
}

// SECURITY: a manual check fetches a URL the admin chose, so one server is checked at most once per VerifyCooldown.
func (s *Service) throttle(id uuid.UUID) error {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if next, ok := s.manual[id]; ok && now.Before(next) {
		return &VerifyTooSoonError{RetryAt: next}
	}
	for k, next := range s.manual {
		if !now.Before(next) {
			delete(s.manual, k)
		}
	}
	s.manual[id] = now.Add(VerifyCooldown)
	return nil
}

// SetTrust turns trust without verification on (self-hosted only) or off; person is who turns it on.
func (s *Service) SetTrust(
	ctx context.Context, orgID, projectID uuid.UUID, publicID string, trusted bool, person uuid.UUID,
) (*ResourceServer, error) {
	ctx = scope(ctx, orgID, projectID)
	if trusted && !s.cfg.TrustAvailable {
		return nil, &TrustSelfHostedOnlyError{}
	}
	var out *ResourceServer
	changed := false
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		r, err := s.store.Lock(ctx, projectID, publicID)
		if err != nil {
			return err
		}
		out = r
		if r.Trusted == trusted {
			return nil
		}
		now := s.now().UTC()
		if trusted {
			r.Trust(person, now)
		} else if err := r.Untrust(); err != nil {
			return err
		}
		r.UpdatedAt, changed = now, true
		return s.store.Update(ctx, r)
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "resourceserver: trust", err)
	}
	if changed {
		s.logTrust(ctx, out, trusted, person)
		s.admissionChanged(projectID)
	}
	return out, nil
}

// Recheck re-verifies the project's untrusted servers not checked for StaleAfter and returns how many changed state.
func (s *Service) Recheck(ctx context.Context, orgID, projectID uuid.UUID) (int, error) {
	ctx = scope(ctx, orgID, projectID)
	due, err := s.store.Due(ctx, projectID, s.now().UTC().Add(-s.cfg.StaleAfter), recheckBatch)
	if err != nil || len(due) == 0 {
		return 0, s.errOrNil(ctx, "resourceserver: recheck", err)
	}
	project, err := s.issuers.Project(ctx, projectID)
	if err != nil {
		return 0, s.errOrNil(ctx, "resourceserver: recheck", err)
	}
	changed := 0
	for _, r := range due {
		if ctx.Err() != nil {
			return changed, ctx.Err()
		}
		f := s.checker.Check(ctx, r.Identifier, project.Issuer)
		var locked *ResourceServer
		var before State
		err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
			var err error
			locked, err = s.store.Lock(ctx, projectID, r.PublicID)
			if err != nil || locked.Trusted {
				return err
			}
			before = locked.State
			if f == nil {
				locked.Pass(s.now().UTC())
			} else {
				locked.Fail(*f)
			}
			locked.UpdatedAt = *locked.LastCheckedAt
			return s.store.Update(ctx, locked)
		})
		switch {
		case IsNotFoundError(err):
			continue
		case err != nil:
			return changed, s.errOrReport(ctx, "resourceserver: recheck", err)
		case locked.State != before && !locked.Trusted:
			changed++
			s.logState(ctx, project.PublicID, locked, before, "scheduled")
			s.admissionChanged(projectID)
		}
	}
	return changed, nil
}

// ListLinks returns the links of the client clientPublicID, or of the server serverPublicID when clientPublicID is empty.
func (s *Service) ListLinks(ctx context.Context, orgID, projectID uuid.UUID, clientPublicID, serverPublicID string) ([]*Link, error) {
	ctx = scope(ctx, orgID, projectID)
	var clientID, serverID uuid.UUID
	if clientPublicID != "" {
		c, err := s.client(ctx, orgID, projectID, clientPublicID)
		if err != nil {
			return nil, s.errOrReport(ctx, "resourceserver: list links", err)
		}
		clientID = c.ID
	} else {
		r, err := s.store.ByPublicID(ctx, projectID, serverPublicID)
		if err != nil {
			return nil, s.errOrReport(ctx, "resourceserver: list links", err)
		}
		serverID = r.ID
	}
	links, err := s.links(ctx, orgID, projectID, clientID, serverID)
	if err != nil {
		return nil, s.errOrReport(ctx, "resourceserver: list links", err)
	}
	return links, nil
}

// LinkClient lets a client ask for some of a server's scopes; linking again replaces the scopes.
func (s *Service) LinkClient(
	ctx context.Context, orgID, projectID uuid.UUID, clientPublicID, serverPublicID string, scopes []string,
) (*Link, error) {
	return s.writeLink(ctx, orgID, projectID, clientPublicID, serverPublicID, scopes, true)
}

// UpdateLink replaces the scopes an existing link allows.
func (s *Service) UpdateLink(
	ctx context.Context, orgID, projectID uuid.UUID, clientPublicID, serverPublicID string, scopes []string,
) (*Link, error) {
	return s.writeLink(ctx, orgID, projectID, clientPublicID, serverPublicID, scopes, false)
}

func (s *Service) writeLink(
	ctx context.Context, orgID, projectID uuid.UUID, clientPublicID, serverPublicID string, scopes []string, create bool,
) (*Link, error) {
	ctx = scope(ctx, orgID, projectID)
	c, err := s.client(ctx, orgID, projectID, clientPublicID)
	if err == nil && c.MetadataURL != "" {
		err = &ClientManagedError{}
	}
	if err != nil {
		return nil, s.errOrReport(ctx, "resourceserver: link", err)
	}
	var out *Link
	err = s.uow.RunInTx(ctx, func(ctx context.Context) error {
		r, err := s.store.Lock(ctx, projectID, serverPublicID)
		if err != nil {
			return err
		}
		pins, err := r.LinkScopes(scopes)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		out = &Link{
			OrgID: orgID, ProjectID: projectID, ClientID: c.ID, Client: c, ResourceServer: r, Scopes: pins,
			CreatedAt: now, UpdatedAt: now,
		}
		if create {
			return s.store.UpsertLink(ctx, out)
		}
		ok, err := s.store.UpdateLink(ctx, out)
		if err == nil && !ok {
			return &NotFoundError{}
		}
		return err
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "resourceserver: link", err)
	}
	return s.reloadLink(ctx, out)
}

func (s *Service) reloadLink(ctx context.Context, l *Link) (*Link, error) {
	links, err := s.store.Links(ctx, l.ProjectID, l.ClientID, uuid.Nil)
	if err != nil {
		return nil, s.errOrReport(ctx, "resourceserver: link", err)
	}
	for _, got := range links {
		if got.ResourceServer.ID == l.ResourceServer.ID {
			got.Client = l.Client
			return got, nil
		}
	}
	return l, nil
}

// UnlinkClient removes the link between a client and a server; a missing link is no error.
func (s *Service) UnlinkClient(ctx context.Context, orgID, projectID uuid.UUID, clientPublicID, serverPublicID string) error {
	ctx = scope(ctx, orgID, projectID)
	c, err := s.client(ctx, orgID, projectID, clientPublicID)
	if err == nil && c.MetadataURL != "" {
		err = &ClientManagedError{}
	}
	if err != nil {
		return s.errOrReport(ctx, "resourceserver: unlink", err)
	}
	r, err := s.store.ByPublicID(ctx, projectID, serverPublicID)
	if err != nil {
		return s.errOrReport(ctx, "resourceserver: unlink", err)
	}
	if err := s.store.DeleteLink(ctx, projectID, c.ID, r.ID); err != nil {
		return s.errOrReport(ctx, "resourceserver: unlink", err)
	}
	return nil
}

func (s *Service) client(ctx context.Context, orgID, projectID uuid.UUID, publicID string) (ClientRef, error) {
	c, ok, err := s.clients.Lookup(ctx, orgID, projectID, publicID)
	switch {
	case err != nil:
		return ClientRef{}, err
	case !ok:
		return ClientRef{}, &ClientNotFoundError{}
	}
	return c, nil
}

func (s *Service) links(ctx context.Context, orgID, projectID, clientID, serverID uuid.UUID) ([]*Link, error) {
	links, err := s.store.Links(ctx, projectID, clientID, serverID)
	if err != nil || len(links) == 0 {
		return links, err
	}
	ids := make([]uuid.UUID, 0, len(links))
	for _, l := range links {
		ids = append(ids, l.ClientID)
	}
	refs, err := s.clients.Refs(ctx, orgID, projectID, ids)
	if err != nil {
		return nil, err
	}
	for _, l := range links {
		l.Client = refs[l.ClientID]
	}
	return links, nil
}

func (s *Service) logState(ctx context.Context, projectPublicID string, r *ResourceServer, before State, check string) {
	if r.State == before {
		return
	}
	attrs := []slog.Attr{
		slog.String("project", projectPublicID), slog.String("resource_server", r.PublicID),
		slog.String("identifier", r.Identifier), slog.String("from", string(before)), slog.String("to", string(r.State)),
		slog.String("check", check), slog.Int("consecutive_failures", int(r.ConsecutiveFailures)),
	}
	if r.LastError != nil {
		attrs = append(attrs, slog.String("reason", string(r.LastError.Reason)))
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "resourceserver: state changed", attrs...)
}

func (s *Service) logTrust(ctx context.Context, r *ResourceServer, trusted bool, by uuid.UUID) {
	attrs := []slog.Attr{
		slog.String("resource_server", r.PublicID), slog.String("identifier", r.Identifier), slog.Bool("trusted", trusted),
	}
	if project, err := s.issuers.Project(ctx, r.ProjectID); err == nil {
		attrs = append(attrs, slog.String("project", project.PublicID))
	}
	if person, err := s.people.UserPublicID(ctx, by); err == nil {
		attrs = append(attrs, slog.String("by", person))
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "resourceserver: trust changed", attrs...)
}

func (s *Service) errOrReport(ctx context.Context, message string, err error) error {
	if IsExpectedError(err) || authz.IsForbiddenError(err) {
		return err
	}
	return s.unexpected(ctx, message, err)
}

func (s *Service) errOrNil(ctx context.Context, message string, err error) error {
	if err == nil {
		return nil
	}
	return s.errOrReport(ctx, message, err)
}

// WithOnAdmissionChange sets what runs when a project's CIMD admission may have changed, so its discovery is published again.
func (s *Service) WithOnAdmissionChange(fn func(projectID uuid.UUID)) *Service {
	s.onAdmissionChange = fn
	return s
}

func (s *Service) admissionChanged(projectID uuid.UUID) {
	if s.onAdmissionChange != nil {
		s.onAdmissionChange(projectID)
	}
}

// SetAdmission sets whether a server admits metadata document clients (E10); turning it on needs an admissible server.
func (s *Service) SetAdmission(ctx context.Context, orgID, projectID uuid.UUID, publicID string, a Admission) (*ResourceServer, error) {
	ctx = scope(ctx, orgID, projectID)
	if err := ValidateClientHosts(a.Hosts); err != nil {
		return nil, err
	}
	var out *ResourceServer
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		r, err := s.store.Lock(ctx, projectID, publicID)
		if err != nil {
			return err
		}
		if a.Scopes != nil {
			if a.Scopes, err = r.LinkScopes(a.Scopes); err != nil {
				return err
			}
		}
		if a.Accept && !r.Admissible(s.cfg.TrustAvailable) {
			return &AdmissionUnverifiedError{}
		}
		r.Admission, r.UpdatedAt = a, s.now().UTC()
		if err := s.store.Update(ctx, r); err != nil {
			return err
		}
		out = r
		return s.repinMetadataClients(ctx, r)
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "resourceserver: admission", err)
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "resourceserver: admission", slog.String("resource_server", out.PublicID),
		slog.Bool("accept", a.Accept), slog.Int("hosts", len(a.Hosts)), slog.Bool("all_scopes", a.Scopes == nil))
	s.admissionChanged(projectID)
	return out, nil
}

func (s *Service) repinMetadataClients(ctx context.Context, r *ResourceServer) error {
	links, err := s.links(ctx, r.OrgID, r.ProjectID, uuid.Nil, r.ID)
	if err != nil {
		return err
	}
	for _, l := range links {
		if l.Client.MetadataURL == "" {
			continue
		}
		l.Scopes, l.UpdatedAt = r.MetadataClientScopes(), r.UpdatedAt
		if _, err := s.store.UpdateLink(ctx, l); err != nil {
			return err
		}
	}
	return nil
}

// Resolve returns the server identifier names exactly and the scopes clientID may ask of it, if admissible and linked (RFC 8707 R4).
func (s *Service) Resolve(
	ctx context.Context, orgID, projectID, clientID uuid.UUID, identifier string,
) (r *ResourceServer, pin []string, ok bool, err error) {
	ctx = scope(ctx, orgID, projectID)
	r, err = s.store.ByIdentifier(ctx, projectID, identifier)
	switch {
	case IsNotFoundError(err):
		return nil, nil, false, nil
	case err != nil:
		return nil, nil, false, s.errOrReport(ctx, "resourceserver: resolve", err)
	case !r.Admissible(s.cfg.TrustAvailable):
		return nil, nil, false, nil
	}
	links, err := s.store.Links(ctx, projectID, clientID, r.ID)
	switch {
	case err != nil:
		return nil, nil, false, s.errOrReport(ctx, "resourceserver: resolve", err)
	case len(links) == 0:
		return nil, nil, false, nil
	}
	return r, links[0].Scopes, true, nil
}

// ForMetadataClient returns the server identifier names and whether it admits metadata document clients from host.
func (s *Service) ForMetadataClient(
	ctx context.Context, orgID, projectID uuid.UUID, identifier, host string,
) (r *ResourceServer, ok bool, err error) {
	ctx = scope(ctx, orgID, projectID)
	r, err = s.store.ByIdentifier(ctx, projectID, identifier)
	switch {
	case IsNotFoundError(err):
		return nil, false, nil
	case err != nil:
		return nil, false, s.errOrReport(ctx, "resourceserver: admit", err)
	}
	ok = r.AdmitsMetadataClient(host, s.cfg.TrustAvailable)
	return r, ok, nil
}

// LinkMetadataClient links an admitted metadata document client to serverID with scopes; it joins the caller's unit of work.
func (s *Service) LinkMetadataClient(ctx context.Context, orgID, projectID, clientID, serverID uuid.UUID, scopes []string) error {
	ctx = scope(ctx, orgID, projectID)
	now := s.now().UTC()
	return s.store.UpsertLink(ctx, &Link{
		OrgID: orgID, ProjectID: projectID, ClientID: clientID, ResourceServer: &ResourceServer{ID: serverID}, Scopes: scopes,
		CreatedAt: now, UpdatedAt: now,
	})
}

// SECURITY: R5, authl gap 5: the server must still be admissible and allow every scope; cimdHost is a CIMD client_id host, else "".
func (s *Service) Admits(
	ctx context.Context, orgID, projectID, serverID, clientID uuid.UUID, cimdHost string, scopes []string,
) (identifier string, ok bool, err error) {
	ctx = scope(ctx, orgID, projectID)
	r, err := s.store.ByID(ctx, projectID, serverID)
	switch {
	case IsNotFoundError(err):
		return "", false, nil
	case err != nil:
		return "", false, s.errOrReport(ctx, "resourceserver: admits", err)
	case !r.Admissible(s.cfg.TrustAvailable):
		return "", false, nil
	}
	allowed := r.MetadataClientScopes()
	if cimdHost != "" {
		if !r.AdmitsMetadataClient(cimdHost, s.cfg.TrustAvailable) {
			return "", false, nil
		}
	} else {
		links, err := s.store.Links(ctx, projectID, clientID, r.ID)
		switch {
		case err != nil:
			return "", false, s.errOrReport(ctx, "resourceserver: admits", err)
		case len(links) == 0:
			return "", false, nil
		}
		allowed = links[0].Scopes
	}
	for _, sc := range scopes {
		if !slices.Contains(allowed, sc) {
			return "", false, nil
		}
	}
	return r.Identifier, true, nil
}

// AcceptsCIMD reports whether any admissible server of the project admits metadata document clients (CIMD §6, pm P3).
func (s *Service) AcceptsCIMD(ctx context.Context, orgID, projectID uuid.UUID) (bool, error) {
	ctx = scope(ctx, orgID, projectID)
	list, err := s.store.List(ctx, projectID)
	if err != nil {
		return false, s.errOrReport(ctx, "resourceserver: accepts cimd", err)
	}
	for _, r := range list {
		if r.Admission.Accept && r.Admissible(s.cfg.TrustAvailable) {
			return true, nil
		}
	}
	return false, nil
}
