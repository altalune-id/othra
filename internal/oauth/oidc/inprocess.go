package oidc

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
)

type inProcessKey struct{}

// InProcess is a transport that serves each request on h in memory, marked so the token endpoint knows it never left this process.
func InProcess(h http.Handler) http.RoundTripper { return inProcess{h: h} }

type inProcess struct{ h http.Handler }

// RoundTrip implements http.RoundTripper.
func (t inProcess) RoundTrip(r *http.Request) (*http.Response, error) {
	req := r.Clone(context.WithValue(r.Context(), inProcessKey{}, true))
	req.RemoteAddr = "127.0.0.1:0"
	req.RequestURI = req.URL.RequestURI()
	if req.Body == nil {
		req.Body = http.NoBody
	}
	rec := &recorder{header: http.Header{}}
	t.h.ServeHTTP(rec, req)
	status := rec.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		Status: strconv.Itoa(status) + " " + http.StatusText(status), StatusCode: status,
		Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header: rec.header, Body: io.NopCloser(bytes.NewReader(rec.body.Bytes())), ContentLength: int64(rec.body.Len()),
		Request: r,
	}, nil
}

// SECURITY: only InProcess sets the key, and its type is unexported, so no network request can claim to be in process.
func isInProcess(ctx context.Context) bool {
	on, _ := ctx.Value(inProcessKey{}).(bool)
	return on
}

type recorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

// Header implements http.ResponseWriter.
func (r *recorder) Header() http.Header { return r.header }

// WriteHeader implements http.ResponseWriter.
func (r *recorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}

// Write implements http.ResponseWriter.
func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}
