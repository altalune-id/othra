package resourceserver

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/altalune-id/othra/httpclient"
	"github.com/altalune-id/othra/oauthrs"
)

// MaxMetadataBytes bounds the RFC 9728 metadata document a check reads.
const MaxMetadataBytes = 64 << 10

const maxEchoedResource = MaxIdentifierLength

// Verifier fetches a resource server's RFC 9728 metadata over the SSRF-filtered client, with no redirects and no retry.
type Verifier struct {
	client *http.Client
	now    func() time.Time
}

var _ Checker = (*Verifier)(nil)

// NewVerifier builds a Verifier whose fetch gives up after timeout; allowPrivateHosts turns the SSRF filter off for development.
func NewVerifier(timeout time.Duration, allowPrivateHosts bool) *Verifier {
	return &Verifier{client: httpclient.New(clientOptions(timeout, allowPrivateHosts)...), now: time.Now}
}

func clientOptions(timeout time.Duration, allowPrivateHosts bool) []httpclient.Option {
	return []httpclient.Option{
		httpclient.WithTimeout(timeout), httpclient.WithoutRedirects(), httpclient.WithAllowPrivateHosts(allowPrivateHosts),
	}
}

// Check passes when the metadata's resource is identical to identifier (RFC 9728 §3.3) and authorization_servers lists issuer.
func (v *Verifier) Check(ctx context.Context, identifier, issuer string) *Failure {
	fail := func(reason FailureReason) *Failure {
		return &Failure{Reason: reason, CheckedAt: v.now().UTC()}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, MetadataURL(identifier), http.NoBody)
	if err != nil {
		return fail(FailureNetwork)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := v.client.Do(req)
	if err != nil {
		return fail(transportReason(err))
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		f := fail(FailureRedirect)
		f.Status = status(resp.StatusCode)
		return f
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		f := fail(FailureNotFound)
		f.Status = status(resp.StatusCode)
		return f
	case resp.StatusCode != http.StatusOK:
		f := fail(FailureStatus)
		f.Status = status(resp.StatusCode)
		return f
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxMetadataBytes+1))
	switch {
	case err != nil:
		return fail(transportReason(err))
	case len(body) > MaxMetadataBytes:
		return fail(FailureTooLarge)
	}
	doc, err := oauthrs.ParseProtectedResource(body)
	if err != nil {
		return fail(FailureNotJSON)
	}
	if doc.Resource != identifier {
		f := fail(FailureResourceMismatch)
		f.Resource = truncate(doc.Resource, maxEchoedResource)
		return f
	}
	if !slices.Contains(doc.AuthorizationServers, issuer) {
		return fail(FailureIssuerMissing)
	}
	return nil
}

func status(code int) int32 {
	if code < 0 || code > 999 {
		return 0
	}
	return int32(code)
}

func transportReason(err error) FailureReason {
	var netErr net.Error
	switch {
	case httpclient.IsPrivateAddressError(err):
		return FailureBlocked
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return FailureTimeout
	}
	return FailureNetwork
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// MetadataURL is where the RFC 9728 metadata of identifier lives, or "" when it can't have any.
func MetadataURL(identifier string) string {
	u, err := oauthrs.MetadataURL(identifier)
	if err != nil {
		return ""
	}
	return u
}
