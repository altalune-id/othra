package boot

import (
	"context"
	"slices"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/config"
	"github.com/altalune-id/othra/internal/apikey"
	"github.com/altalune-id/othra/internal/auth"
	"github.com/altalune-id/othra/internal/client"
	"github.com/altalune-id/othra/internal/consent"
	"github.com/altalune-id/othra/internal/controlplane"
	"github.com/altalune-id/othra/internal/enduser"
	"github.com/altalune-id/othra/internal/federation"
	"github.com/altalune-id/othra/internal/grant"
	"github.com/altalune-id/othra/internal/install"
	"github.com/altalune-id/othra/internal/kernel"
	"github.com/altalune-id/othra/internal/kernel/capabilities"
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/mailqueue"
	"github.com/altalune-id/othra/internal/kernel/outbox"
	"github.com/altalune-id/othra/internal/kernel/ratelimit"
	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/kernel/storage"
	"github.com/altalune-id/othra/internal/kernel/usage"
	"github.com/altalune-id/othra/internal/keys"
	"github.com/altalune-id/othra/internal/legal"
	"github.com/altalune-id/othra/internal/oauth/oidc"
	"github.com/altalune-id/othra/internal/org"
	"github.com/altalune-id/othra/internal/org/invite"
	"github.com/altalune-id/othra/internal/org/member"
	"github.com/altalune-id/othra/internal/pat"
	"github.com/altalune-id/othra/internal/project"
	"github.com/altalune-id/othra/internal/resourceserver"
	"github.com/altalune-id/othra/internal/upstream"
	"github.com/altalune-id/othra/internal/user"
	"github.com/altalune-id/othra/internal/webhook"
	"github.com/altalune-id/othra/mailer"
	"github.com/altalune-id/othra/sealer"
)

const keyUsageInterval = 30 * time.Second

// Services is every store and service the composition root wires.
type Services struct {
	OrgStore        org.Store
	Orgs            *org.Service
	ProjectStore    project.Store
	Projects        *project.Service
	APIKeyStore     apikey.Store
	APIKeys         *apikey.Service
	KeyScheme       apikey.Scheme
	KeyAuthn        *apikey.Authenticator
	KeyUsage        *usage.Worker
	Outbox          outbox.Store
	WebhookStore    webhook.Store
	Webhooks        *webhook.Service
	Sealer          sealer.Sealer
	Mailer          mailer.Mailer
	MailQueue       mailqueue.Store
	Users           *user.Service
	Members         *member.Service
	Invites         *invite.Service
	Sessions        session.Store
	Auth            *auth.Service
	Resets          *auth.Resets
	Install         *install.Service
	Legal           *legal.Service
	InstallToken    install.Token
	InstallRequired *atomic.Bool
	Caps            capabilities.Capabilities
	AuthLimiter     ratelimit.Limiter
	StateSecret     []byte
	Validator       *controlplane.Validator
	Storage         *storage.Service
	Keys            *keys.Service
	Clients         *client.Service
	ResourceServers *resourceserver.Service
	IssuerResolver  *oidc.Resolver
	EndUsers        *enduser.Service
	Grants          *grant.Service
	Consents        *consent.Service
	Upstreams       *upstream.Service
	Federation      *federation.Service
	PersonalTokens  *pat.Service
	TokenUsage      *usage.Worker
	ConsolePages    consolePages
}

func buildServices(k kernel.Kernel, cfg config.Config) *Services {
	orgStore := org.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix)
	projectStore := project.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix)
	apiKeyStore := apikey.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix)

	uow := db.NewTransactor(k.Pool)
	files := buildStorage(k, cfg)
	caps := capabilities.From(capabilitiesConfig(cfg))
	sl := buildSealer(cfg.Secrets)
	signingKeys := buildKeys(k, cfg, uow, sl)
	users := user.NewService(user.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix), k.Reporter.Unexpected)
	members := member.NewService(member.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix), uow, k.Reporter.Unexpected).
		WithOperatorSupport(caps.OperatorOrgSupport)
	projects := project.NewService(projectStore, uow, memberPermissions{members: members},
		func(ctx context.Context, p *project.Project) error {
			_, err := signingKeys.Generate(ctx, p.OrgID, p.ID)
			return err
		}, publicBase(cfg.HTTP), k.Reporter.Unexpected).WithStorage(files)
	orgs := org.NewService(orgStore, uow, memberPermissions{members: members},
		func(ctx context.Context, o *org.Org) error {
			if err := ownNewOrg(ctx, users, members, o); err != nil {
				return err
			}
			_, err := projects.Create(ctx, project.CreateRequest{OrgID: o.ID, Slug: project.DefaultSlug, Name: project.DefaultName})
			return err
		}, k.Reporter.Unexpected).WithCreation(caps.OrgCreation)
	projects.WithOrgSlugs(projectOrgSlugs{orgs: orgs})

	outboxStore := outbox.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix)
	webhookStore := webhook.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix)
	projectOwners := projectOrgs{projects: projects}
	webhooks := webhook.NewService(
		webhookStore, k.Log, k.Reporter.Unexpected, sl, outboxStore,
		refResolver{projects: projects, orgs: orgs}, append(webhookServiceOpts(cfg), webhook.WithProjects(projectOwners))...,
	)

	keyScheme := apikey.NewScheme("")
	owners := ownerRoles{members: members}
	apiKeys := apikey.NewService(apiKeyStore, keyScheme, projectIDs{projects: projects}, owners, k.Reporter.Unexpected).
		WithPersonalKeys(caps.PersonalKeys)
	keyUsage := usage.New("apikey-usage", apiKeyStore, keyUsageInterval, k.Log)
	keyAuthn := apikey.NewAuthenticator(apiKeyStore, keyUsage, keyScheme, owners, k.Reporter.Unexpected)
	sessions := buildSessionStore(k, cfg, sl, secretsLog(k.Log, caps))
	mailQueue := buildMailQueue(k.Pool, cfg, sl)
	svcs := &Services{
		ConsolePages: newConsolePages(cfg.HTTP),
		OrgStore:     orgStore,
		Orgs:         orgs,
		ProjectStore: projectStore,
		Projects:     projects,
		APIKeyStore:  apiKeyStore,
		APIKeys:      apiKeys,
		KeyScheme:    keyScheme,
		KeyAuthn:     keyAuthn,
		KeyUsage:     keyUsage,
		Outbox:       outboxStore,
		WebhookStore: webhookStore,
		Webhooks:     webhooks,
		Sealer:       sl,
		MailQueue:    mailQueue,
		Users:        users,
		Members:      members,
		Sessions:     sessions,
		Legal:        buildLegal(k, cfg, users),
		Caps:         caps,
		AuthLimiter:  ratelimit.Disabled(),
		Storage:      files,
		Keys:         signingKeys,
		Clients:      buildClients(k, cfg, uow, projects),
	}
	svcs.ResourceServers = buildResourceServers(k, cfg, uow, projects, users, svcs.Clients)
	svcs.Clients.WithLinks(rsLinks{servers: svcs.ResourceServers})
	svcs.IssuerResolver = oidc.NewResolver(projects, signingKeys, svcs.ResourceServers, publicBase(cfg.HTTP), k.Reporter.Unexpected)
	svcs.Grants = buildGrants(k, cfg, uow, signingKeys, grantAudiences{servers: svcs.ResourceServers})
	svcs.EndUsers = buildEndUsers(k, cfg, uow, mailQueue, endUserGrants{grants: svcs.Grants}, projects)
	svcs.Consents = buildConsents(k, cfg, uow, svcs)
	svcs.PersonalTokens, svcs.TokenUsage = buildPersonalTokens(k, cfg, uow, svcs.ResourceServers, svcs.EndUsers)
	svcs.Upstreams = buildUpstreams(k, cfg, uow, sl, projects)
	svcs.Federation = buildFederation(k, cfg, uow, svcs.Upstreams, svcs.EndUsers, projects)
	projects.WithSignInMethods(projectSignInMethods{upstreams: svcs.Upstreams, linked: svcs.Federation})
	svcs.Resets = auth.NewResets(auth.ResetDeps{
		Store:      auth.NewResetStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix),
		Transactor: uow,
		Users:      users,
		Passwords:  users,
		Sessions:   sessions,
		Mail:       svcs.MailQueue,
		From:       cfg.Mail.From,
		BaseURL:    publicBase(cfg.HTTP),
	})
	members.WithEffects(memberEffects(cfg, svcs)).WithDirectory(memberDirectory{users: users, orgs: orgs})
	svcs.Invites = invite.NewService(invite.Deps{
		Store:      invite.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix),
		Transactor: uow,
		Members:    members,
		Mail:       svcs.MailQueue,
		Users:      users,
		Terms:      svcs.Legal,
		Events:     webhooks,
		Orgs:       memberOrgs{orgs: orgs},
		People:     memberPeople{users: users},
		From:       cfg.Mail.From,
		BaseURL:    publicBase(cfg.HTTP),
		Unexpected: k.Reporter.Unexpected,
	})
	svcs.Auth = auth.NewService(auth.Deps{
		Users:      users,
		Members:    members,
		Orgs:       orgs,
		Projects:   projects,
		Sessions:   sessions,
		Passwords:  users,
		Basic:      basicCredentials(cfg, caps),
		Profiles:   users,
		Transactor: uow,
		Mail:       mailQueue,
		From:       cfg.Mail.From,
		BaseURL:    publicBase(cfg.HTTP),
		Identities: users,
		Invites:    consoleInvites{invites: svcs.Invites},
		OrgCreator: orgs,
		OpenSignup: cfg.OpenSignup(),
	}).WithOperatorOrgSupport(caps.OperatorOrgSupport)
	svcs.Install = buildInstall(cfg, svcs, uow)
	return svcs
}

func buildLegal(k kernel.Kernel, cfg config.Config, users *user.Service) *legal.Service {
	l := legal.NewService(users, legal.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix), db.NewTransactor(k.Pool))
	if cfg.Compliance.TermsURL != "" || cfg.Compliance.PrivacyURL != "" {
		l.WithExternalTerms(cfg.Compliance.TermsUpdated())
	}
	return l
}

func memberEffects(cfg config.Config, s *Services) member.Effects {
	return member.Effects{
		Sessions: s.Sessions,
		Keys:     s.APIKeys,
		People:   memberPeople{users: s.Users},
		Orgs:     memberOrgs{orgs: s.Orgs},
		Mail:     s.MailQueue,
		From:     cfg.Mail.From,
		BaseURL:  publicBase(cfg.HTTP),
	}
}

type memberPeople struct{ users *user.Service }

func (p memberPeople) Contact(ctx context.Context, userID uuid.UUID) (email, name string, err error) {
	u, err := p.users.ByID(ctx, userID)
	if err != nil {
		return "", "", err
	}
	return u.Email, u.Name, nil
}

func (p memberPeople) PublicID(ctx context.Context, userID uuid.UUID) (string, error) {
	u, err := p.users.ByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return u.PublicID, nil
}

type memberDirectory struct {
	users *user.Service
	orgs  *org.Service
}

func (d memberDirectory) UserIDByEmail(ctx context.Context, email string) (uuid.UUID, error) {
	u, err := d.users.FindByEmail(ctx, email)
	if err != nil {
		return uuid.Nil, err
	}
	return u.ID, nil
}

func (d memberDirectory) UserPublicID(ctx context.Context, id uuid.UUID) (string, error) {
	u, err := d.users.ByID(ctx, id)
	if err != nil {
		return "", err
	}
	return u.PublicID, nil
}

func (d memberDirectory) OrgIDByPublicID(ctx context.Context, publicID string) (uuid.UUID, error) {
	o, err := d.orgs.ByPublicID(ctx, publicID)
	if err != nil {
		return uuid.Nil, err
	}
	return o.ID, nil
}

type inviteSignIn struct{ *auth.Service }

func (s inviteSignIn) LoginLocal(ctx context.Context, email, password string) (session.Principal, error) {
	return s.Service.LoginLocal(ctx, auth.Credentials{Email: email, Password: password})
}

// SECURITY: cloud signup's one org per person: the creator's row is locked, so two concurrent creations can't both pass the check.
func ownNewOrg(ctx context.Context, users *user.Service, members *member.Service, o *org.Org) error {
	if !o.CreatedBy.Valid {
		return nil
	}
	owner := o.CreatedBy.UUID
	if err := users.Lock(ctx, owner); err != nil {
		return err
	}
	ms, err := members.ListByUser(ctx, owner)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(ms, func(m *member.Membership) bool { return m.Role == member.RoleOwner }) {
		return &org.AlreadyOwnedError{}
	}
	_, err = members.Add(ctx, o.ID, owner, member.RoleOwner)
	return err
}

type consoleInvites struct{ invites *invite.Service }

func (c consoleInvites) Seat(ctx context.Context, publicID string) (*auth.Seat, error) {
	inv, err := c.invites.Landing(ctx, publicID)
	if err != nil {
		return nil, noSeat(err)
	}
	if usable := c.invites.Usable(inv) == nil; !usable {
		return nil, nil
	}
	return &auth.Seat{ID: inv.ID, OrgID: inv.OrgID, Email: inv.Email}, nil
}

func (c consoleInvites) OwnerSeat(ctx context.Context, orgID uuid.UUID, email string) (*auth.Seat, error) {
	inv, err := c.invites.Live(ctx, orgID, email)
	if err != nil {
		return nil, noSeat(err)
	}
	if !inv.EmailOnlyOwner() {
		return nil, nil
	}
	return &auth.Seat{ID: inv.ID, OrgID: inv.OrgID, Email: inv.Email}, nil
}

func (c consoleInvites) Claim(ctx context.Context, seat *auth.Seat, userID uuid.UUID) (bool, error) {
	if _, err := c.invites.Accept(ctx, seat.OrgID, seat.ID, userID); err != nil {
		return false, noSeat(err)
	}
	return true, nil
}

// NOTE: an invite that is gone, used, expired or for another email is no seat to claim, not a failure.
func noSeat(err error) error {
	if invite.IsNotFoundError(err) || invite.IsNotUsableError(err) || invite.IsEmailMismatchError(err) {
		return nil
	}
	return err
}

type memberOrgs struct{ orgs *org.Service }

func (o memberOrgs) Name(ctx context.Context, orgID uuid.UUID) (string, error) {
	got, err := o.orgs.Get(ctx, orgID)
	if err != nil {
		return "", err
	}
	return got.Name, nil
}

func (o memberOrgs) Slug(ctx context.Context, orgID uuid.UUID) (string, error) {
	got, err := o.orgs.Get(ctx, orgID)
	if err != nil {
		return "", err
	}
	return got.Slug, nil
}

// webhookServiceOpts allows http:// endpoint URLs outside production (dev/self-hosted).
func webhookServiceOpts(cfg config.Config) []webhook.ServiceOption {
	if cfg.Env.IsProduction() {
		return nil
	}
	return []webhook.ServiceOption{webhook.WithAllowInsecureURL()}
}

type refResolver struct {
	projects *project.Service
	orgs     *org.Service
}

func (r refResolver) ProjectRef(ctx context.Context, projectID uuid.UUID) (slug, publicID string, err error) {
	pr, err := r.projects.Get(ctx, projectID)
	if err != nil {
		return "", "", err
	}
	return pr.Slug, pr.PublicID, nil
}

func (r refResolver) OrgRef(ctx context.Context, orgID uuid.UUID) (publicID string, err error) {
	o, err := r.orgs.Get(ctx, orgID)
	if err != nil {
		return "", err
	}
	return o.PublicID, nil
}

type projectIDs struct{ projects *project.Service }

func (p projectIDs) ProjectIDs(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error) {
	list, err := p.projects.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(list))
	for _, pr := range list {
		ids = append(ids, pr.ID)
	}
	return ids, nil
}

type ownerRoles struct{ members *member.Service }

func (o ownerRoles) Role(ctx context.Context, orgID, userID uuid.UUID) (member.Role, bool, error) {
	m, err := o.members.Find(ctx, orgID, userID)
	if member.IsNotFoundError(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return m.Role, true, nil
}

type memberPermissions struct{ members *member.Service }

func (m memberPermissions) Role(ctx context.Context, orgID, userID uuid.UUID) (role string, permissions []string, ok bool, err error) {
	r, ok, err := ownerRoles(m).Role(ctx, orgID, userID)
	if err != nil || !ok {
		return "", nil, false, err
	}
	return string(r), member.BuiltinPermissions(r), true, nil
}

func (m memberPermissions) IsMember(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	_, ok, err := ownerRoles(m).Role(ctx, orgID, userID)
	return ok, err
}

type projectOrgs struct{ projects *project.Service }

func (p projectOrgs) OrgOf(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	pr, err := p.projects.Get(ctx, projectID)
	if err != nil {
		return uuid.Nil, err
	}
	return pr.OrgID, nil
}

func capabilitiesConfig(cfg config.Config) capabilities.Config {
	return capabilities.Config{
		Selfhosted:  cfg.Mode == config.ModeSelfhosted,
		Cloud:       cfg.Mode == config.ModeCloud,
		BasicAuth:   cfg.Auth.Mode == config.AuthModeBasic,
		EnvGenesis:  cfg.Auth.Genesis.Email != "",
		FileStorage: cfg.Storage.Driver != "",
		Passwords:   cfg.PasswordsOn(),
	}
}

type projectOrgSlugs struct{ orgs *org.Service }

func (p projectOrgSlugs) Slug(ctx context.Context, orgID uuid.UUID) (string, error) {
	o, err := p.orgs.Get(ctx, orgID)
	if err != nil {
		return "", err
	}
	return o.Slug, nil
}
