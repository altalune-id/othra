package authz_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/authz"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/scope"
)

func TestAllowed(t *testing.T) {
	person := session.Principal{Source: session.SourceLocal, UserID: uuid.New(), Scopes: []string{"webhook:*", scope.OrgRead}}
	assert.True(t, authz.Allowed(person, scope.WebhookWrite))
	assert.False(t, authz.Allowed(person, scope.MemberWrite))
	assert.True(t, authz.Allowed(person, authz.AnyAuthenticated))

	op := session.Principal{Source: session.SourceBasic, Operator: true}
	assert.True(t, authz.Allowed(op, scope.MemberWrite))

	assert.False(t, authz.Allowed(session.Principal{}, authz.AnyAuthenticated), "nobody is not authenticated")
	assert.False(t, authz.Allowed(person, ""), "an unmapped procedure is denied")
}

func TestAllowed_KeyNeverHoldsPersonOnly(t *testing.T) {
	key := session.Principal{Source: session.SourceAPIKey, Scopes: []string{"member:*"}}
	assert.False(t, authz.Allowed(key, scope.MemberWrite))
	assert.True(t, authz.Allowed(key, scope.MemberRead))

	opKey := session.Principal{Source: session.SourceAPIKey, Operator: true}
	assert.False(t, authz.Allowed(opKey, scope.MemberWrite))
	assert.True(t, authz.Allowed(opKey, authz.AnyAuthenticated))
}

func TestRequire(t *testing.T) {
	ctx := session.PrincipalInto(context.Background(), session.Principal{Source: session.SourceLocal, UserID: uuid.New(), Scopes: []string{scope.WebhookRead}})
	assert.NoError(t, authz.Require(ctx, scope.WebhookRead))
	assert.True(t, authz.IsForbiddenError(authz.Require(ctx, scope.WebhookWrite)))
	assert.True(t, authz.IsForbiddenError(authz.Require(context.Background(), scope.WebhookRead)))
}

func TestForbiddenError(t *testing.T) {
	err := &authz.ForbiddenError{Permission: scope.WebhookWrite}
	assert.Contains(t, err.Error(), scope.WebhookWrite)
	ae := err.ToAppError()
	assert.Equal(t, apperror.CodeForbidden, ae.Code())
	assert.Equal(t, "You do not have permission to do this", ae.Message())
	assert.False(t, authz.IsForbiddenError(assert.AnError))
}

func TestAllowed_PersonOnlyNeedsAPerson(t *testing.T) {
	assert.True(t, authz.Allowed(session.Principal{Source: session.SourceLocal, UserID: uuid.New()}, authz.PersonOnly))
	assert.True(t, authz.Allowed(session.Principal{Source: session.SourceOIDC, UserID: uuid.New()}, authz.PersonOnly))
	for name, p := range map[string]session.Principal{
		"key":            {Source: session.SourceAPIKey, UserID: uuid.New(), Operator: true, Scopes: []string{"*"}},
		"basic operator": {Source: session.SourceBasic, Operator: true},
		"system":         session.System(),
		"nobody":         {},
		"local, no user": {Source: session.SourceLocal},
	} {
		assert.False(t, authz.Allowed(p, authz.PersonOnly), name)
	}
}
