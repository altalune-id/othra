// Package project models projects within an organization.
package project

import (
	"math/rand/v2"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"altalune.id/othra/apperror"
	projectv1 "altalune.id/othra/gen/go/project/v1"
	"altalune.id/othra/nanoid"
)

const (
	slugMinLen = 3
	slugMaxLen = 64
	nameMinLen = 1
	nameMaxLen = 200

	issuerHandleMinLen = 3
	issuerHandleMaxLen = 40
	logoMaxBytes       = 256 << 10
)

// DefaultSlug and DefaultName seed the project every newly created org gets.
const (
	DefaultSlug = "default"
	DefaultName = "Default"
)

// SystemHandle is the console project's issuer handle (/p/system); it is reserved, so no other project can take it.
const SystemHandle = "system"

// SystemSlug is the console project's slug in the system org.
const SystemSlug = "console"

const (
	systemName        = "Console"
	systemDisplayName = "Othra"
)

//nolint:gochecknoglobals // immutable compiled pattern.
var slugRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

// Project is an aggregate scoped to an organization.
type Project struct {
	ID                   uuid.UUID
	PublicID             string
	OrgID                uuid.UUID
	Slug                 string
	Name                 string
	IssuerHandle         string
	IssuerHandleLockedAt *time.Time
	DisplayName          string
	PrimaryColor         string
	SupportEmail         string
	LogoFileID           *uuid.UUID
	LogoKey              string
	TermsURL             string
	PrivacyURL           string
	TermsVersion         int32
	TermsPublishedAt     *time.Time
	SignInEmailLink      bool
	CreatedBy            uuid.NullUUID
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Issuer is the public face of a project resolved by its handle before sign-in; it is never saved.
type Issuer struct {
	ProjectID        uuid.UUID
	OrgID            uuid.UUID
	PublicID         string
	Handle           string
	Name             string
	DisplayName      string
	PrimaryColor     string
	SupportEmail     string
	LogoKey          string
	TermsURL         string
	PrivacyURL       string
	TermsVersion     int32
	TermsPublishedAt *time.Time
	SignInEmailLink  bool
}

// NewProject constructs a Project after validating slug and name.
func NewProject(orgID uuid.UUID, slug, name string, createdBy uuid.UUID) (*Project, error) {
	s := strings.TrimSpace(slug)
	if err := ValidateSlug(s); err != nil {
		return nil, err
	}
	n := strings.TrimSpace(name)
	if err := ValidateName(n); err != nil {
		return nil, err
	}
	publicID, err := nanoid.New(21)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	p := &Project{
		ID: uuid.New(), PublicID: publicID, OrgID: orgID, Slug: s, Name: n,
		IssuerHandle: withIssuerSuffix(s), SignInEmailLink: true, CreatedAt: now, UpdatedAt: now,
	}
	if createdBy != uuid.Nil {
		p.CreatedBy = uuid.NullUUID{UUID: createdBy, Valid: true}
	}
	return p, nil
}

// NewSystemProject constructs the console project of the system org: issuer handle SystemHandle, branded "Othra".
func NewSystemProject(orgID uuid.UUID) (*Project, error) {
	p, err := NewProject(orgID, SystemSlug, systemName, uuid.Nil)
	if err != nil {
		return nil, err
	}
	p.IssuerHandle, p.DisplayName = SystemHandle, systemDisplayName
	return p, nil
}

// IsSystem reports whether p is the console project, which Othra manages.
func (p *Project) IsSystem() bool { return p.IssuerHandle == SystemHandle }

// Update applies a new display name and slug.
func (p *Project) Update(name, slug string) error {
	n := strings.TrimSpace(name)
	if err := ValidateName(n); err != nil {
		return err
	}
	s := strings.TrimSpace(slug)
	if err := ValidateSlug(s); err != nil {
		return err
	}
	p.Name = n
	p.Slug = s
	return nil
}

// ValidateSlug reports whether s is usable as a project slug.
func ValidateSlug(s string) error {
	if len(s) < slugMinLen || len(s) > slugMaxLen {
		return &InvalidSlugError{Slug: s, Reason: SlugLength}
	}
	if !slugRe.MatchString(s) {
		return &InvalidSlugError{Slug: s, Reason: SlugFormat}
	}
	return nil
}

// ValidateName reports whether n is usable as a project name.
func ValidateName(n string) error {
	if len(n) < nameMinLen || len(n) > nameMaxLen {
		return &InvalidNameError{Reason: NameLength}
	}
	return nil
}

//nolint:gochecknoglobals // a fixed list, the one place reserved issuer handles live.
var reservedIssuerHandles = []string{
	SystemHandle, "account", "admin", "api", "auth", "console", "default", "healthz", "install", "login", "mcp",
	"oauth", "oauth2", "readyz", "static", "well-known", "www", "othra", "altalune",
}

var (
	colorRe        = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	supportEmailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+$`)
)

const (
	displayNameMaxLen  = 60
	supportEmailMaxLen = 254
	linkMaxLen         = 2048
	issuerSuffixLen    = 4
	issuerSuffixChars  = "abcdefghijklmnopqrstuvwxyz0123456789"
)

// ValidateIssuerHandle reports whether h is usable as an issuer handle: 3 to 40 lowercase letters, digits and inner dashes, not reserved.
func ValidateIssuerHandle(h string) error {
	if len(h) < issuerHandleMinLen || len(h) > issuerHandleMaxLen {
		return &InvalidIssuerHandleError{Handle: h, Reason: IssuerHandleLength}
	}
	if !slugRe.MatchString(h) {
		return &InvalidIssuerHandleError{Handle: h, Reason: IssuerHandleFormat}
	}
	if slices.Contains(reservedIssuerHandles, h) {
		return &InvalidIssuerHandleError{Handle: h, Reason: IssuerHandleReserved}
	}
	return nil
}

// DefaultIssuerHandle is the handle a new project gets: its org slug and its slug, cut to the length limit.
func DefaultIssuerHandle(orgSlug, projectSlug string) string {
	if orgSlug == "" {
		return cutHandle(projectSlug, issuerHandleMaxLen)
	}
	return cutHandle(orgSlug+"-"+projectSlug, issuerHandleMaxLen)
}

// IssuerURL is the project's exact issuer: base, then /p/ and the handle, with no trailing slash.
func IssuerURL(base, handle string) string {
	return strings.TrimSuffix(base, "/") + "/p/" + handle
}

func withIssuerSuffix(base string) string {
	suffix := make([]byte, issuerSuffixLen)
	for i := range suffix {
		suffix[i] = issuerSuffixChars[rand.IntN(len(issuerSuffixChars))]
	}
	return cutHandle(base, issuerHandleMaxLen-issuerSuffixLen-1) + "-" + string(suffix)
}

func cutHandle(h string, limit int) string {
	if len(h) > limit {
		h = h[:limit]
	}
	return strings.TrimRight(h, "-")
}

// SetIssuerHandle changes the handle; it is refused once the handle is locked.
func (p *Project) SetIssuerHandle(h string) error {
	if h == p.IssuerHandle {
		return nil
	}
	if p.IssuerHandleLockedAt != nil {
		return &IssuerHandleLockedError{}
	}
	if err := ValidateIssuerHandle(h); err != nil {
		return err
	}
	p.IssuerHandle = h
	return nil
}

// LockIssuerHandle freezes the handle at now; a locked handle keeps its first lock time.
func (p *Project) LockIssuerHandle(now time.Time) {
	if p.IssuerHandleLockedAt == nil {
		at := now.UTC()
		p.IssuerHandleLockedAt = &at
	}
}

// UpdateBranding sets the display name, primary colour and support email; each may be empty.
func (p *Project) UpdateBranding(displayName, primaryColor, supportEmail string) error {
	d, c, e := strings.TrimSpace(displayName), strings.TrimSpace(primaryColor), strings.TrimSpace(supportEmail)
	switch {
	case utf8.RuneCountInString(d) > displayNameMaxLen:
		return invalidField("display_name", apperror.FieldTooLong, strconv.Itoa(displayNameMaxLen))
	case c != "" && !colorRe.MatchString(c):
		return invalidField("primary_color", apperror.FieldInvalidFormat, "")
	case len(e) > supportEmailMaxLen:
		return invalidField("support_email", apperror.FieldTooLong, strconv.Itoa(supportEmailMaxLen))
	case e != "" && !supportEmailRe.MatchString(e):
		return invalidField("support_email", apperror.FieldInvalidFormat, "")
	}
	p.DisplayName, p.PrimaryColor, p.SupportEmail = d, c, e
	return nil
}

// UpdateTerms sets the terms and privacy links; setting a terms link where there was none publishes a new terms version at now.
func (p *Project) UpdateTerms(termsURL, privacyURL string, now time.Time) error {
	t, pr := strings.TrimSpace(termsURL), strings.TrimSpace(privacyURL)
	if err := validateLink("terms_url", t); err != nil {
		return err
	}
	if err := validateLink("privacy_url", pr); err != nil {
		return err
	}
	if p.TermsURL == "" && t != "" {
		p.publishTerms(now)
	}
	p.TermsURL, p.PrivacyURL = t, pr
	return nil
}

// BumpTermsVersion publishes a new terms version at now, so every end user accepts again; it needs a terms link.
func (p *Project) BumpTermsVersion(now time.Time) (int32, error) {
	if p.TermsURL == "" {
		return 0, &TermsNotSetError{}
	}
	p.publishTerms(now)
	return p.TermsVersion, nil
}

func (p *Project) publishTerms(now time.Time) {
	at := now.UTC()
	p.TermsVersion++
	p.TermsPublishedAt = &at
}

func validateLink(field, link string) error {
	if link == "" {
		return nil
	}
	if len(link) > linkMaxLen {
		return invalidField(field, apperror.FieldTooLong, strconv.Itoa(linkMaxLen))
	}
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return invalidField(field, apperror.FieldInvalidFormat, "")
	}
	return nil
}

func invalidField(field string, reason apperror.FieldReason, param string) error {
	return apperror.Validation([]apperror.Violation{{Field: field, Reason: reason, Param: param}})
}

// Ref is a project's ids and slug with its org's slug, as a console route names them.
type Ref struct {
	ID, OrgID     uuid.UUID
	Slug, OrgSlug string
}

type projectRow struct {
	ID                   uuid.UUID     `alias:"projects.id"`
	PublicID             string        `alias:"projects.public_id"`
	OrgID                uuid.UUID     `alias:"projects.org_id"`
	Slug                 string        `alias:"projects.slug"`
	Name                 string        `alias:"projects.name"`
	IssuerHandle         string        `alias:"projects.issuer_handle"`
	IssuerHandleLockedAt *time.Time    `alias:"projects.issuer_handle_locked_at"`
	DisplayName          string        `alias:"projects.display_name"`
	PrimaryColor         string        `alias:"projects.primary_color"`
	SupportEmail         string        `alias:"projects.support_email"`
	LogoFileID           *uuid.UUID    `alias:"projects.logo_file_id"`
	LogoKey              *string       `alias:"projects.logo_object_key"`
	TermsURL             string        `alias:"projects.terms_url"`
	PrivacyURL           string        `alias:"projects.privacy_url"`
	TermsVersion         int32         `alias:"projects.terms_version"`
	TermsPublishedAt     *time.Time    `alias:"projects.terms_published_at"`
	SignInEmailLink      bool          `alias:"projects.signin_email_link"`
	CreatedBy            uuid.NullUUID `alias:"projects.created_by"`
	CreatedAt            time.Time     `alias:"projects.created_at"`
	UpdatedAt            time.Time     `alias:"projects.updated_at"`
}

func (r *projectRow) toProject() *Project {
	return &Project{
		ID: r.ID, PublicID: r.PublicID, OrgID: r.OrgID, Slug: r.Slug, Name: r.Name,
		IssuerHandle: r.IssuerHandle, IssuerHandleLockedAt: utcPtr(r.IssuerHandleLockedAt),
		DisplayName: r.DisplayName, PrimaryColor: r.PrimaryColor, SupportEmail: r.SupportEmail,
		LogoFileID: r.LogoFileID, LogoKey: derefString(r.LogoKey),
		TermsURL: r.TermsURL, PrivacyURL: r.PrivacyURL, TermsVersion: r.TermsVersion, TermsPublishedAt: utcPtr(r.TermsPublishedAt),
		SignInEmailLink: r.SignInEmailLink, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func projectToProto(p *Project, orgPublicID, issuerURL, logoURL string) *projectv1.Project {
	return &projectv1.Project{
		Id:                   p.PublicID,
		OrgId:                orgPublicID,
		Slug:                 p.Slug,
		Name:                 p.Name,
		CreatedAt:            timestamppb.New(p.CreatedAt),
		UpdatedAt:            timestamppb.New(p.UpdatedAt),
		IssuerHandle:         p.IssuerHandle,
		IssuerUrl:            issuerURL,
		IssuerHandleLocked:   p.IssuerHandleLockedAt != nil,
		System:               p.IsSystem(),
		IssuerHandleLockedAt: timestampOrNil(p.IssuerHandleLockedAt),
		DisplayName:          p.DisplayName,
		PrimaryColor:         p.PrimaryColor,
		SupportEmail:         p.SupportEmail,
		TermsUrl:             p.TermsURL,
		PrivacyUrl:           p.PrivacyURL,
		TermsVersion:         p.TermsVersion,
		TermsPublishedAt:     timestampOrNil(p.TermsPublishedAt),
		SigninEmailLink:      p.SignInEmailLink,
		LogoUrl:              logoURL,
	}
}

func timestampOrNil(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
