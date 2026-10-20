package oidc

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/internal/project"
)

const publicCache = "public, max-age=300"

type document struct {
	body []byte
	etag string
}

func newDocument(body []byte) document {
	sum := sha256.Sum256(body)
	return document{body: body, etag: `"` + base64.RawURLEncoding.EncodeToString(sum[:]) + `"`}
}

func (d document) serve(w http.ResponseWriter, r *http.Request, contentType string) {
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", publicCache)
	h.Set("ETag", d.etag)
	if etagMatches(r.Header.Get("If-None-Match"), d.etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Length", strconv.Itoa(len(d.body)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(d.body)
	}
}

func etagMatches(ifNoneMatch, etag string) bool {
	for tag := range strings.SplitSeq(ifNoneMatch, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "*" || strings.TrimPrefix(tag, "W/") == etag {
			return true
		}
	}
	return false
}

type issuerEntry struct {
	issuer    *project.Issuer
	discovery document
	jwks      document
	expires   time.Time
}

type issuerCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]*issuerEntry
}

func (c *issuerCache) get(handle string, now time.Time) *issuerEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[handle]
	if !ok || !now.Before(e.expires) {
		return nil
	}
	return e
}

func (c *issuerCache) put(handle string, e *issuerEntry, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for h, old := range c.entries {
		if !now.Before(old.expires) {
			delete(c.entries, h)
		}
	}
	e.expires = now.Add(c.ttl)
	c.entries[handle] = e
}

func (c *issuerCache) forget(projectID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for h, e := range c.entries {
		if e.issuer.ProjectID == projectID {
			delete(c.entries, h)
		}
	}
}
