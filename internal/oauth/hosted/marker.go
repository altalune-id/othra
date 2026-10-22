package hosted

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"net/url"
	"time"
)

// MarkerParam is the authorize query parameter that carries the re-entry marker.
const MarkerParam = "othra_t"

// HintParam is the authorize query parameter that carries the signed subject of an id_token_hint across sign-in.
const HintParam = "othra_h"

const markerSkew = time.Minute

const (
	purposeReentry = "othra-reentry"
	purposeHint    = "othra-hint"
)

// Marker signs the time Othra sent a person from /authorize to sign in or choose an account (G3), so re-entry does not loop.
type Marker struct {
	key    []byte
	maxAge time.Duration
	now    func() time.Time
}

// NewMarker builds a Marker signed with key whose markers live for maxAge.
func NewMarker(key []byte, maxAge time.Duration) *Marker {
	return &Marker{key: key, maxAge: maxAge, now: time.Now}
}

// Mark returns a copy of q with a marker for clientID, issued now.
func (m *Marker) Mark(q url.Values, clientID string) url.Values {
	out := copyValues(q)
	out.Set(MarkerParam, m.seal(purposeReentry, clientID, ""))
	return out
}

// Since returns when the marker in q was issued for clientID; ok is false when it is missing, forged, repeated or too old.
func (m *Marker) Since(q url.Values, clientID string) (time.Time, bool) {
	at, _, ok := m.open(q[MarkerParam], purposeReentry, clientID)
	return at, ok
}

// WithHint returns a copy of q carrying subject, signed for clientID, in place of an id_token_hint.
func (m *Marker) WithHint(q url.Values, clientID, subject string) url.Values {
	out := copyValues(q)
	out.Set(HintParam, m.seal(purposeHint, clientID, subject))
	return out
}

// Hinted returns the subject WithHint signed into q for clientID; ok is false when it is missing, forged, repeated or too old.
func (m *Marker) Hinted(q url.Values, clientID string) (string, bool) {
	_, subject, ok := m.open(q[HintParam], purposeHint, clientID)
	return subject, ok && subject != ""
}

// Unmarked returns a copy of q without the marker and the signed hint.
func Unmarked(q url.Values) url.Values {
	out := copyValues(q)
	out.Del(MarkerParam)
	out.Del(HintParam)
	return out
}

func copyValues(q url.Values) url.Values {
	out := make(url.Values, len(q))
	for k, v := range q {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func (m *Marker) seal(purpose, clientID, payload string) string {
	at := m.now().UnixMilli()
	buf := binary.BigEndian.AppendUint64(nil, uint64(at)) //nolint:gosec // unix milliseconds are positive.
	buf = append(buf, m.mac(purpose, clientID, at, payload)...)
	return encodeToken(append(buf, payload...))
}

func (m *Marker) open(values []string, purpose, clientID string) (time.Time, string, bool) {
	if len(values) != 1 {
		return time.Time{}, "", false
	}
	b, err := decodeToken(values[0])
	if err != nil || len(b) < 8+sha256.Size {
		return time.Time{}, "", false
	}
	issued := int64(binary.BigEndian.Uint64(b[:8])) //nolint:gosec // a forged value fails the MAC first.
	payload := string(b[8+sha256.Size:])
	if !hmac.Equal(b[8:8+sha256.Size], m.mac(purpose, clientID, issued, payload)) {
		return time.Time{}, "", false
	}
	at, now := time.UnixMilli(issued).UTC(), m.now()
	if at.After(now.Add(markerSkew)) || now.Sub(at) > m.maxAge {
		return time.Time{}, "", false
	}
	return at, payload, true
}

func (m *Marker) mac(purpose, clientID string, issued int64, payload string) []byte {
	h := hmac.New(sha256.New, m.key)
	h.Write([]byte(purpose + "\x00"))
	h.Write(binary.BigEndian.AppendUint32(nil, uint32(len(clientID)))) //nolint:gosec // a client id is short.
	h.Write([]byte(clientID))
	h.Write(binary.BigEndian.AppendUint64(nil, uint64(issued))) //nolint:gosec // the issued time as signed.
	h.Write([]byte(payload))
	return h.Sum(nil)
}
