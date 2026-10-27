package fakes

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/client"
)

// ClientStore is an in-memory client.Store; FailOn names a method that returns Err instead.
type ClientStore struct {
	mu      sync.Mutex
	clients map[uuid.UUID]*client.Client
	secrets map[uuid.UUID]*clientSecret
	// Hashes holds every stored secret hash by secret id.
	Hashes map[uuid.UUID]string
	FailOn string
	Err    error
}

type clientSecret struct {
	clientID uuid.UUID
	secret   client.Secret
}

var _ client.Store = (*ClientStore)(nil)

var errOneCurrentSecret = errors.New("fakes: a client already has a current secret")

// NewClientStore returns an empty store.
func NewClientStore() *ClientStore {
	return &ClientStore{
		clients: map[uuid.UUID]*client.Client{}, secrets: map[uuid.UUID]*clientSecret{}, Hashes: map[uuid.UUID]string{},
	}
}

func (f *ClientStore) fail(op string) error {
	if f.FailOn == op {
		return f.Err
	}
	return nil
}

// Insert implements client.Store.
func (f *ClientStore) Insert(_ context.Context, c *client.Client) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("Insert"); err != nil {
		return err
	}
	cp := *c
	cp.Secrets = nil
	f.clients[c.ID] = &cp
	return nil
}

// Update implements client.Store.
func (f *ClientStore) Update(_ context.Context, c *client.Client) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("Update"); err != nil {
		return err
	}
	cp := *c
	cp.Secrets = nil
	f.clients[c.ID] = &cp
	return nil
}

// Delete implements client.Store.
func (f *ClientStore) Delete(_ context.Context, projectID, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("Delete"); err != nil {
		return err
	}
	c, ok := f.clients[id]
	if !ok || c.ProjectID != projectID {
		return &client.NotFoundError{}
	}
	delete(f.clients, id)
	for sid, s := range f.secrets {
		if s.clientID == id {
			delete(f.secrets, sid)
		}
	}
	return nil
}

// List implements client.Store.
func (f *ClientStore) List(_ context.Context, projectID uuid.UUID, now time.Time) ([]*client.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("List"); err != nil {
		return nil, err
	}
	var out []*client.Client
	for _, c := range f.clients {
		if c.ProjectID == projectID && !c.Managed() {
			out = append(out, f.withSecrets(c, now))
		}
	}
	slices.SortFunc(out, func(a, b *client.Client) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out, nil
}

// ByIDs implements client.Store.
func (f *ClientStore) ByIDs(_ context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]*client.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("ByIDs"); err != nil {
		return nil, err
	}
	var out []*client.Client
	for _, id := range ids {
		if c, ok := f.clients[id]; ok && c.ProjectID == projectID {
			cp := *c
			out = append(out, &cp)
		}
	}
	return out, nil
}

// ByPublicID implements client.Store.
func (f *ClientStore) ByPublicID(_ context.Context, projectID uuid.UUID, publicID string, now time.Time) (*client.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.find("ByPublicID", projectID, publicID, now)
}

// ForAuthentication implements client.Store.
func (f *ClientStore) ForAuthentication(_ context.Context, projectID uuid.UUID, publicID string, now time.Time) (*client.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, err := f.find("ForAuthentication", projectID, publicID, now)
	if err != nil {
		return nil, err
	}
	for i, s := range c.Secrets {
		c.Secrets[i] = s.WithHash(f.Hashes[s.ID])
	}
	return c, nil
}

// Lock implements client.Store.
func (f *ClientStore) Lock(_ context.Context, projectID uuid.UUID, publicID string, now time.Time) (*client.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.find("Lock", projectID, publicID, now)
}

// LockSystem returns the project's console client.
func (f *ClientStore) LockSystem(_ context.Context, projectID uuid.UUID, now time.Time) (*client.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("LockSystem"); err != nil {
		return nil, err
	}
	for _, c := range f.clients {
		if c.ProjectID == projectID && c.System {
			return f.withSecrets(c, now), nil
		}
	}
	return nil, &client.NotFoundError{}
}

func (f *ClientStore) find(op string, projectID uuid.UUID, publicID string, now time.Time) (*client.Client, error) {
	if err := f.fail(op); err != nil {
		return nil, err
	}
	for _, c := range f.clients {
		if c.ProjectID == projectID && (c.PublicID == publicID || (c.Managed() && c.MetadataURL == publicID)) {
			return f.withSecrets(c, now), nil
		}
	}
	return nil, &client.NotFoundError{}
}

func (f *ClientStore) withSecrets(c *client.Client, now time.Time) *client.Client {
	cp := *c
	cp.Secrets = []client.Secret{}
	for _, s := range f.secrets {
		if s.clientID == c.ID && (s.secret.ExpiresAt == nil || s.secret.ExpiresAt.After(now)) {
			cp.Secrets = append(cp.Secrets, s.secret)
		}
	}
	slices.SortFunc(cp.Secrets, func(a, b client.Secret) int {
		if a.Current() != b.Current() {
			if a.Current() {
				return -1
			}
			return 1
		}
		return b.CreatedAt.Compare(a.CreatedAt)
	})
	return &cp
}

// PruneShadows implements client.Store; the fake knows no consents or grants, so every expired shadow goes.
func (f *ClientStore) PruneShadows(_ context.Context, projectID uuid.UUID, cutoff time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("PruneShadows"); err != nil {
		return 0, err
	}
	n := 0
	for id, c := range f.clients {
		if c.ProjectID == projectID && c.Managed() && c.MetadataExpiresAt != nil && c.MetadataExpiresAt.Before(cutoff) {
			delete(f.clients, id)
			n++
		}
	}
	return n, nil
}

// SaveShadow implements client.Store.
func (f *ClientStore) SaveShadow(_ context.Context, c *client.Client) (*client.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("SaveShadow"); err != nil {
		return nil, err
	}
	cp := *c
	cp.Secrets = nil
	for id, have := range f.clients {
		if have.ProjectID == c.ProjectID && have.MetadataURL == c.MetadataURL {
			cp.ID, cp.PublicID, cp.CreatedAt = id, have.PublicID, have.CreatedAt
		}
	}
	f.clients[cp.ID] = &cp
	out := cp
	out.Secrets = []client.Secret{}
	return &out, nil
}

// InsertSecret implements client.Store.
func (f *ClientStore) InsertSecret(_ context.Context, c *client.Client, s client.Secret, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("InsertSecret"); err != nil {
		return err
	}
	for _, other := range f.secrets {
		if other.clientID == c.ID && other.secret.ExpiresAt == nil {
			return errOneCurrentSecret
		}
	}
	f.secrets[s.ID] = &clientSecret{clientID: c.ID, secret: s}
	f.Hashes[s.ID] = hash
	return nil
}

// ExpireSecret implements client.Store.
func (f *ClientStore) ExpireSecret(_ context.Context, projectID, clientID, id uuid.UUID, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("ExpireSecret"); err != nil {
		return err
	}
	if s, ok := f.secrets[id]; ok && s.clientID == clientID && f.clients[clientID] != nil && f.clients[clientID].ProjectID == projectID {
		s.secret.ExpiresAt = &at
	}
	return nil
}

// DeleteSecrets implements client.Store.
func (f *ClientStore) DeleteSecrets(_ context.Context, projectID, clientID uuid.UUID, ids []uuid.UUID, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("DeleteSecrets"); err != nil {
		return err
	}
	if c := f.clients[clientID]; c == nil || c.ProjectID != projectID {
		return nil
	}
	for id, s := range f.secrets {
		if s.clientID != clientID {
			continue
		}
		if slices.Contains(ids, id) || (s.secret.ExpiresAt != nil && !s.secret.ExpiresAt.After(now)) {
			delete(f.secrets, id)
			delete(f.Hashes, id)
		}
	}
	return nil
}

// SecretCount returns how many secret rows the store holds, live or not.
func (f *ClientStore) SecretCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.secrets)
}
