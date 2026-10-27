// Package consent holds the scopes an end user approved for an OAuth client and a resource server, and decides when to ask again.
package consent

import (
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MaxScopes bounds the scopes one consent holds.
const MaxScopes = 72

// Consent is what an end user approved for one client and one resource server, or for the client alone when ResourceServerID is nil.
type Consent struct {
	ID        uuid.UUID
	PublicID  string
	OrgID     uuid.UUID
	ProjectID uuid.UUID
	Key       Key
	Scopes    []string
	GrantedAt time.Time
	UpdatedAt time.Time
	IP        string
	UserAgent string
}

// Key names one consent: an end user, a client and a resource server, or no resource server for an OIDC-only request.
type Key struct {
	EndUserID        uuid.UUID
	ClientID         uuid.UUID
	ResourceServerID *uuid.UUID
}

// Covers reports whether c already approved every scope; a nil consent covers nothing.
func (c *Consent) Covers(scopes []string) bool {
	if c == nil {
		return false
	}
	for _, s := range scopes {
		if !slices.Contains(c.Scopes, s) {
			return false
		}
	}
	return true
}

// Split returns the requested scopes c never approved, then those it did, each in request order.
func (c *Consent) Split(requested []string) (fresh, before []string) {
	for _, s := range requested {
		if c != nil && slices.Contains(c.Scopes, s) {
			before = append(before, s)
			continue
		}
		fresh = append(fresh, s)
	}
	return fresh, before
}

// Union returns have followed by the scopes of add it lacks, without repeats.
func Union(have, add []string) []string {
	out := make([]string, 0, len(have)+len(add))
	for _, s := range slices.Concat(have, add) {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// Page is what an authorization request shows before a code is issued.
type Page int

// The pages: none issues the code at once; full lists the scopes; remembered is the one-click "Continue to {client}" page.
const (
	PageNone Page = iota
	PageFull
	PageRemembered
)

// Request is the part of an authorization request the consent decision reads.
type Request struct {
	Implicit      bool
	Confidential  bool
	PromptConsent bool
	Scopes        []string
	// Marked and Since carry the re-entry marker: the request came back from a consent page shown at Since.
	Marked bool
	Since  time.Time
}

// Decide returns the page r shows given the stored consent c (OIDC Core §3.1.2.4).
func Decide(c *Consent, r Request) Page {
	switch {
	case r.Marked && c.Covers(r.Scopes) && c.UpdatedAt.Truncate(time.Millisecond).After(r.Since):
		return PageNone
	case r.PromptConsent:
		return PageFull
	case r.Implicit:
		return PageNone
	case !c.Covers(r.Scopes):
		return PageFull
	case r.Confidential:
		return PageNone
	default:
		return PageRemembered
	}
}

// Family is a live grant a client holds for an end user, as Grants reports it.
type Family struct {
	ClientID         uuid.UUID
	ResourceServerID *uuid.UUID
	Scopes           []string
	CreatedAt        time.Time
	LastUsedAt       time.Time
}

// App is one client an end user is connected to through a consent, a live grant, or both (profile §2.5).
type App struct {
	ClientID    uuid.UUID
	Resources   []Resource
	Consented   bool
	Offline     bool
	ConnectedAt time.Time
	LastUsedAt  time.Time
}

// Resource is what an App may use at one resource server, or the OIDC scopes alone when ResourceServerID is nil.
type Resource struct {
	ResourceServerID *uuid.UUID
	Scopes           []string
}

const scopeOfflineAccess = "offline_access"

// Apps groups the end user's consents and live families by client, newest use first; resourceScopes splits off the OIDC scopes.
func Apps(consents []*Consent, families []Family, resourceScopes func([]string) []string) []App {
	byClient := map[uuid.UUID]*App{}
	var order []uuid.UUID
	app := func(id uuid.UUID, at time.Time) *App {
		a, ok := byClient[id]
		if !ok {
			a = &App{ClientID: id, ConnectedAt: at}
			byClient[id] = a
			order = append(order, id)
		}
		if at.Before(a.ConnectedAt) {
			a.ConnectedAt = at
		}
		return a
	}
	for _, c := range consents {
		a := app(c.Key.ClientID, c.GrantedAt)
		a.Consented = true
		a.add(c.Key.ResourceServerID, c.Scopes, resourceScopes)
	}
	for _, f := range families {
		a := app(f.ClientID, f.CreatedAt)
		a.Offline = a.Offline || slices.Contains(f.Scopes, scopeOfflineAccess)
		if f.LastUsedAt.After(a.LastUsedAt) {
			a.LastUsedAt = f.LastUsedAt
		}
		a.add(f.ResourceServerID, f.Scopes, resourceScopes)
	}
	out := make([]App, 0, len(order))
	for _, id := range order {
		a := *byClient[id]
		slices.SortStableFunc(a.Resources, compareResources)
		out = append(out, a)
	}
	slices.SortStableFunc(out, func(x, y App) int { return y.lastActive().Compare(x.lastActive()) })
	return out
}

func (a *App) add(rs *uuid.UUID, scopes []string, resourceScopes func([]string) []string) {
	if rs == nil {
		a.addScopes(nil, scopes)
		return
	}
	at := resourceScopes(scopes)
	a.addScopes(rs, at)
	if signIn := slices.DeleteFunc(slices.Clone(scopes), func(s string) bool { return slices.Contains(at, s) }); len(signIn) > 0 {
		a.addScopes(nil, signIn)
	}
}

func (a *App) addScopes(rs *uuid.UUID, scopes []string) {
	for i := range a.Resources {
		if sameResource(a.Resources[i].ResourceServerID, rs) {
			a.Resources[i].Scopes = sortedUnion(a.Resources[i].Scopes, scopes)
			return
		}
	}
	a.Resources = append(a.Resources, Resource{ResourceServerID: rs, Scopes: sortedUnion(nil, scopes)})
}

// NOTE: sorted, so the order shown never depends on which of two equal-time consents was read first.
func sortedUnion(have, add []string) []string {
	out := Union(have, add)
	slices.Sort(out)
	return out
}

func (a App) lastActive() time.Time {
	if a.LastUsedAt.After(a.ConnectedAt) {
		return a.LastUsedAt
	}
	return a.ConnectedAt
}

// NOTE: resource servers first, by id, then the OIDC-only scopes, so the order never depends on equal timestamps.
func compareResources(x, y Resource) int {
	switch {
	case x.ResourceServerID == nil && y.ResourceServerID == nil:
		return 0
	case x.ResourceServerID == nil:
		return 1
	case y.ResourceServerID == nil:
		return -1
	}
	return strings.Compare(x.ResourceServerID.String(), y.ResourceServerID.String())
}

func sameResource(a, b *uuid.UUID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// ClientInfo is how a connected app is shown: its client_id, name, and the host of its metadata document when it has one.
type ClientInfo struct {
	OAuthID      string
	Name         string
	MetadataHost string
}

// ServerInfo is how a resource server is shown with what an app may do there.
type ServerInfo struct {
	PublicID     string
	Name         string
	Identifier   string
	Descriptions map[string]string
}
