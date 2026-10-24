package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"altalune.id/othra/httpclient"
)

// HTTPMetadataFetcher fetches client metadata documents over the SSRF-filtered client, with no redirects and no retry.
type HTTPMetadataFetcher struct {
	client *http.Client
}

var _ MetadataFetcher = (*HTTPMetadataFetcher)(nil)

// NewMetadataFetcher builds a fetcher that gives up after timeout; allowPrivateHosts turns the SSRF filter off for development.
func NewMetadataFetcher(timeout time.Duration, allowPrivateHosts bool) *HTTPMetadataFetcher {
	return &HTTPMetadataFetcher{client: httpclient.New(metadataClientOptions(timeout, allowPrivateHosts)...)}
}

func metadataClientOptions(timeout time.Duration, allowPrivateHosts bool) []httpclient.Option {
	return []httpclient.Option{
		httpclient.WithTimeout(timeout), httpclient.WithoutRedirects(), httpclient.WithAllowPrivateHosts(allowPrivateHosts),
	}
}

// FetchMetadata GETs url and returns at most MaxMetadataBytes+1 bytes of a 200 response (CIMD §5, §8.7).
func (f *HTTPMetadataFetcher) FetchMetadata(ctx context.Context, url string) (*FetchedMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("client: metadata status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxMetadataBytes+1))
	if err != nil {
		return nil, err
	}
	return &FetchedMetadata{Body: body, ContentType: resp.Header.Get("Content-Type"), CacheControl: resp.Header.Get("Cache-Control")}, nil
}
