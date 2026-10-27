package hosted

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMarker(now *time.Time) *Marker {
	m := NewMarker([]byte("marker-key-marker-key-marker-key"), 30*time.Minute)
	m.now = func() time.Time { return *now }
	return m
}

// a signed marker says when Othra sent the person to sign in or choose; only a later sign-in or choice satisfies it.
func TestMarker_RoundTrip(t *testing.T) {
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	m := newTestMarker(&now)
	q := url.Values{"client_id": {"web"}, "prompt": {"login"}}
	marked := m.Mark(q, "web")
	assert.Equal(t, "login", marked.Get("prompt"), "the rest of the query is kept")
	require.NotEmpty(t, marked.Get(MarkerParam))
	assert.Empty(t, q.Get(MarkerParam), "Mark copies, it never changes its input")

	now = now.Add(5 * time.Minute)
	at, ok := m.Since(marked, "web")
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC), at)

	stripped := Unmarked(marked)
	assert.Empty(t, stripped.Get(MarkerParam))
	assert.Equal(t, "login", stripped.Get("prompt"))
}

func TestMarker_Refusals(t *testing.T) {
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	m := newTestMarker(&now)
	marked := m.Mark(url.Values{}, "web")
	other := NewMarker([]byte("another key, another deployment."), 30*time.Minute)
	for name, tc := range map[string]struct {
		q      url.Values
		client string
		at     time.Time
	}{
		"missing":         {url.Values{}, "web", now},
		"garbage":         {url.Values{MarkerParam: {"nope!"}}, "web", now},
		"truncated":       {url.Values{MarkerParam: {marked.Get(MarkerParam)[:10]}}, "web", now},
		"another client":  {marked, "cli", now},
		"another key":     {other.Mark(url.Values{}, "web"), "web", now},
		"too old":         {marked, "web", now.Add(31 * time.Minute)},
		"from the future": {marked, "web", now.Add(-2 * time.Minute)},
		"repeated":        {url.Values{MarkerParam: {marked.Get(MarkerParam), marked.Get(MarkerParam)}}, "web", now},
	} {
		check := now
		now = tc.at
		_, ok := m.Since(tc.q, tc.client)
		assert.False(t, ok, name)
		now = check
	}
}

// the subject of an id_token_hint crosses sign-in signed, never as the token itself.
func TestMarker_Hint(t *testing.T) {
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	m := newTestMarker(&now)
	q := m.WithHint(url.Values{"prompt": {"login"}}, "web", "sub-1")
	sub, ok := m.Hinted(q, "web")
	require.True(t, ok)
	assert.Equal(t, "sub-1", sub)

	_, ok = m.Hinted(q, "cli")
	assert.False(t, ok, "bound to the client")
	_, ok = m.Since(url.Values{MarkerParam: q[HintParam]}, "web")
	assert.False(t, ok, "a hint is not a marker")
	_, ok = m.Hinted(url.Values{HintParam: m.Mark(url.Values{}, "web")[MarkerParam]}, "web")
	assert.False(t, ok, "a marker is not a hint")
	forged := []byte(q.Get(HintParam))
	forged[len(forged)-1] ^= 1
	_, ok = m.Hinted(url.Values{HintParam: {string(forged)}}, "web")
	assert.False(t, ok, "the subject is signed")
	now = now.Add(31 * time.Minute)
	_, ok = m.Hinted(q, "web")
	assert.False(t, ok, "too old")
	assert.Empty(t, Unmarked(q).Get(HintParam))
}
