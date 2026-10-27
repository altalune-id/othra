package consent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConsent_Covers(t *testing.T) {
	c := &Consent{Scopes: []string{"openid", "sample:read", "offline_access"}}
	assert.True(t, c.Covers([]string{"sample:read", "openid"}))
	assert.True(t, c.Covers(nil), "nothing to approve")
	assert.False(t, c.Covers([]string{"sample:read", "sample:write"}))
	var none *Consent
	assert.False(t, none.Covers([]string{"openid"}))
}

// approving again adds to what was approved, never narrows it.
func TestUnion(t *testing.T) {
	assert.Equal(t, []string{"openid", "sample:read", "sample:write"}, Union([]string{"openid", "sample:read"}, []string{"sample:write", "openid"}))
	assert.Equal(t, []string{"a"}, Union(nil, []string{"a", "a"}))
}

// the page lists the new scopes first, then the ones approved before.
func TestSplit(t *testing.T) {
	c := &Consent{Scopes: []string{"openid", "sample:read"}}
	fresh, before := c.Split([]string{"openid", "sample:write", "sample:read", "offline_access"})
	assert.Equal(t, []string{"sample:write", "offline_access"}, fresh)
	assert.Equal(t, []string{"openid", "sample:read"}, before)
	var none *Consent
	fresh, before = none.Split([]string{"openid"})
	assert.Equal(t, []string{"openid"}, fresh)
	assert.Empty(t, before)
}

// who sees which page.
func TestDecide(t *testing.T) {
	t0 := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	stored := &Consent{Scopes: []string{"openid", "sample:read"}, UpdatedAt: t0}
	approvedAfter := &Consent{Scopes: []string{"openid", "sample:read"}, UpdatedAt: t0.Add(time.Second)}
	asked := []string{"openid", "sample:read"}
	more := []string{"openid", "sample:read", "sample:write"}
	for name, tc := range map[string]struct {
		c    *Consent
		r    Request
		want Page
	}{
		"implicit":                        {nil, Request{Implicit: true, Scopes: asked}, PageNone},
		"implicit, prompt=consent":        {nil, Request{Implicit: true, PromptConsent: true, Scopes: asked}, PageFull},
		"first time":                      {nil, Request{Confidential: true, Scopes: asked}, PageFull},
		"new scope":                       {stored, Request{Confidential: true, Scopes: more}, PageFull},
		"remembered, confidential":        {stored, Request{Confidential: true, Scopes: asked}, PageNone},
		"remembered, public":              {stored, Request{Scopes: asked}, PageRemembered},
		"remembered, prompt=consent":      {stored, Request{Confidential: true, PromptConsent: true, Scopes: asked}, PageFull},
		"re-entry after the full page":    {approvedAfter, Request{PromptConsent: true, Scopes: asked, Marked: true, Since: t0}, PageNone},
		"re-entry after continue":         {approvedAfter, Request{Scopes: asked, Marked: true, Since: t0}, PageNone},
		"re-entry, consent older":         {stored, Request{PromptConsent: true, Scopes: asked, Marked: true, Since: t0}, PageFull},
		"re-entry, same millisecond":      {&Consent{Scopes: asked, UpdatedAt: t0.Add(time.Microsecond)}, Request{Scopes: asked, Marked: true, Since: t0}, PageRemembered},
		"re-entry, scope added since":     {approvedAfter, Request{PromptConsent: true, Scopes: more, Marked: true, Since: t0}, PageFull},
		"marker without a fresh approval": {stored, Request{Scopes: asked, Marked: true, Since: t0.Add(time.Hour)}, PageRemembered},
	} {
		assert.Equal(t, tc.want, Decide(tc.c, tc.r), name)
	}
}
