package session

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// EndUser is the end user an account.v1 call runs for: the browser's active account in one project (M8 G2).
type EndUser struct {
	OrgID           uuid.UUID
	ProjectID       uuid.UUID
	ProjectPublicID string
	ProjectName     string
	EndUserID       uuid.UUID
	Subject         string
	UserSession     string
	AuthTime        time.Time
	// SignOutToken and LinkToken are this browser's CSRF tokens for the hosted sign-out and connect-an-upstream forms the portal posts.
	SignOutToken string
	LinkToken    string
}

type endUserKey struct{}

// EndUserInto returns ctx carrying eu.
func EndUserInto(ctx context.Context, eu EndUser) context.Context {
	return context.WithValue(ctx, endUserKey{}, eu)
}

// EndUserFrom returns the EndUser in ctx; ok is false outside the account plane.
func EndUserFrom(ctx context.Context) (eu EndUser, ok bool) {
	eu, ok = ctx.Value(endUserKey{}).(EndUser)
	return eu, ok
}
