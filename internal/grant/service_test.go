package grant

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/testutil/fakes"
	"altalune.id/othra/scheduler"
)

var errBoom = errors.New("boom")

type failingStore struct {
	err    error
	pruned int
}

func (s *failingStore) InsertCode(context.Context, uuid.UUID, NewCode) error { return s.err }
func (s *failingStore) CodeByHash(context.Context, uuid.UUID, string) (*Code, error) {
	return nil, s.err
}
func (s *failingStore) UseCode(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) error {
	return s.err
}
func (s *failingStore) InsertFamily(context.Context, uuid.UUID, uuid.UUID, *Family) error {
	return s.err
}
func (s *failingStore) FamilyBySID(context.Context, uuid.UUID, string) (*Family, error) {
	return nil, s.err
}
func (s *failingStore) TouchFamily(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
	return s.err
}
func (s *failingStore) RevokeFamily(context.Context, uuid.UUID, uuid.UUID, string, time.Time) error {
	return s.err
}
func (s *failingStore) RevokeUserSession(context.Context, uuid.UUID, string, string, time.Time) error {
	return s.err
}
func (s *failingStore) InsertAccessToken(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID, *uuid.UUID, string, []string, time.Time, time.Time,
) error {
	return s.err
}
func (s *failingStore) RevokeAccessToken(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
	return s.err
}
func (s *failingStore) Subject(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", s.err
}
func (s *failingStore) AccessByHash(context.Context, uuid.UUID, string) (*Access, error) {
	return nil, s.err
}
func (s *failingStore) InsertRefreshToken(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, time.Time, time.Time) error {
	return s.err
}
func (s *failingStore) RefreshByHash(context.Context, uuid.UUID, string) (*RefreshToken, *Family, error) {
	return nil, nil, s.err
}
func (s *failingStore) UseRefreshToken(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
	return s.err
}
func (s *failingStore) Prune(context.Context, uuid.UUID, Cutoffs) (int, error) {
	return s.pruned, s.err
}

func newFailing(t *testing.T, err error) (*Service, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	reporter := apperror.NewReporter(slog.New(slog.NewJSONHandler(&logs, nil)), false)
	return NewService(&failingStore{err: err}, fakes.Transactor{}, Config{}, reporter.Unexpected), &logs
}

func TestVerifyPKCE(t *testing.T) {
	v := strings.Repeat("A", 43)
	sum := sha256.Sum256([]byte(v))
	c := base64.RawURLEncoding.EncodeToString(sum[:])
	for name, tc := range map[string]struct {
		challenge, verifier string
		want                bool
	}{
		"matches":                  {c, v, true},
		"no pkce, no verifier":     {"", "", true},
		"no pkce, a verifier":      {"", v, false},
		"pkce, no verifier":        {c, "", false},
		"wrong verifier":           {c, strings.Repeat("B", 43), false},
		"too short":                {c, v[:42], false},
		"too long":                 {c, strings.Repeat("A", 129), false},
		"outside the unreserved":   {c, strings.Repeat("A", 42) + "+", false},
		"every unreserved allowed": {challengeOf("aZ09-._~" + strings.Repeat("x", 35)), "aZ09-._~" + strings.Repeat("x", 35), true},
	} {
		assert.Equal(t, tc.want, verifyPKCE(tc.challenge, tc.verifier), name)
	}
}

func challengeOf(v string) string {
	sum := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func TestFamilyLive(t *testing.T) {
	now := time.Now()
	f := &Family{AbsoluteExpiresAt: now.Add(time.Hour)}
	assert.True(t, f.Live(now))
	assert.False(t, f.Live(now.Add(time.Hour)))
	f.RevokedAt = &now
	assert.False(t, f.Live(now))
	assert.True(t, subset([]string{"openid"}, []string{"openid", "email"}))
	assert.False(t, subset([]string{"profile"}, []string{"openid"}))
}

func TestErrors(t *testing.T) {
	for _, tc := range []struct {
		err error
		is  func(error) bool
	}{
		{&InvalidGrantError{}, IsInvalidGrantError},
		{&InvalidScopeError{}, IsInvalidScopeError},
		{&InvalidTokenError{}, IsInvalidTokenError},
		{&SessionGoneError{}, IsSessionGoneError},
		{&InvalidRequestError{}, IsInvalidRequestError},
	} {
		assert.True(t, tc.is(tc.err), "%T", tc.err)
		assert.True(t, IsExpectedError(tc.err), "%T", tc.err)
		assert.NotEmpty(t, tc.err.Error())
	}
	assert.False(t, IsExpectedError(errBoom))
}

// SECURITY: a store failure is reported once and answered as unexpected, never as a token or a grant.
func TestService_ReportsUnexpectedFailures(t *testing.T) {
	svc, logs := newFailing(t, errBoom)
	ctx, p := context.Background(), Project{OrgID: uuid.New(), ID: uuid.New()}
	_, err := svc.IssueCode(ctx, p, CodeRequest{})
	assertUnexpected(t, err)
	_, err = svc.Exchange(ctx, p, ExchangeRequest{Code: "c"})
	assertUnexpected(t, err)
	_, err = svc.Refresh(ctx, p, RefreshRequest{RefreshToken: "r"})
	assertUnexpected(t, err)
	_, err = svc.Access(ctx, p, "a")
	assertUnexpected(t, err)
	_, err = svc.Family(ctx, p, "sid")
	assertUnexpected(t, err)
	_, err = svc.AnyFamily(ctx, p, "sid")
	assertUnexpected(t, err)
	_, err = svc.RevokeFamily(ctx, p, "sid", ReasonEndSession)
	assertUnexpected(t, err)
	assertUnexpected(t, svc.RevokeUserSession(ctx, p, "us", ReasonSignOut))
	assertUnexpected(t, svc.Revoke(ctx, p, Client{}, "t", ""))
	_, err = svc.Prune(ctx, p.OrgID, p.ID)
	assertUnexpected(t, err)
	assert.Contains(t, logs.String(), "boom")

	svc.newSecret = func() (string, error) { return "", errBoom }
	_, err = svc.IssueCode(ctx, p, CodeRequest{})
	assertUnexpected(t, err)
}

func assertUnexpected(t *testing.T, err error) {
	t.Helper()
	ae, ok := apperror.AsAppError(err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, apperror.CodeUnexpected, ae.Code())
}

func TestService_ReturnsTypedErrorsAsIs(t *testing.T) {
	svc, logs := newFailing(t, &SessionGoneError{})
	_, err := svc.IssueCode(context.Background(), Project{}, CodeRequest{})
	assert.True(t, IsSessionGoneError(err))
	svc, _ = newFailing(t, nil)
	_, err = svc.Exchange(context.Background(), Project{}, ExchangeRequest{Code: "c"})
	assert.True(t, IsInvalidGrantError(err), "an unknown code")
	_, err = svc.Refresh(context.Background(), Project{}, RefreshRequest{RefreshToken: "r"})
	assert.True(t, IsInvalidGrantError(err), "an unknown refresh token")
	_, err = svc.Access(context.Background(), Project{}, "a")
	assert.True(t, IsInvalidTokenError(err), "an unknown access token")
	f, err := svc.RevokeFamily(context.Background(), Project{}, "sid", ReasonEndSession)
	require.NoError(t, err)
	assert.Nil(t, f, "an unknown sid is no error")
	assert.Empty(t, logs.String())
}

type projectList []Project

func (p projectList) Each(ctx context.Context, fn func(ctx context.Context, orgID, projectID uuid.UUID) error) error {
	for _, pr := range p {
		if err := fn(ctx, pr.OrgID, pr.ID); err != nil {
			return err
		}
	}
	return nil
}

func TestScheduler_PrunesEveryProject(t *testing.T) {
	store := &failingStore{pruned: 2}
	svc := NewService(store, fakes.Transactor{}, Config{}, apperror.NewReporter(nil, false).Unexpected)
	var logs bytes.Buffer
	projects := projectList{{OrgID: uuid.New(), ID: uuid.New()}, {OrgID: uuid.New(), ID: uuid.New()}}
	jobs := NewScheduler(svc, projects, slog.New(slog.NewJSONHandler(&logs, nil))).SchedulerJobs()
	require.Len(t, jobs, 1)
	job := jobs[0]
	assert.Equal(t, "grant-prune", job.Name)
	assert.Equal(t, scheduler.ScopeSystem, job.Scope)
	assert.True(t, job.Singleton)

	require.NoError(t, job.Run(context.Background()))
	assert.Contains(t, logs.String(), `"deleted":4`)

	store.err = errBoom
	assert.Error(t, job.Run(context.Background()))
}
