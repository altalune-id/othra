// Package redirecturi holds the redirect URI rules OAuth clients register under and the matcher /authorize checks them with.
package redirecturi

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ApplicationType is the OIDC application type a redirect URI is registered for; each one has its rules in Validate.
type ApplicationType string

// The application types.
const (
	Web    ApplicationType = "web"
	Native ApplicationType = "native"
)

// The limits on a list of redirect URIs.
const (
	MaxURIs   = 20
	MaxLength = 2000
)

// Validate checks one redirect URI against the rules for its application type (OAuth 2.1 §2.3.1, RFC 8252 §7.3).
func Validate(appType ApplicationType, uri string) error {
	fail := func(r Reason) error { return &Error{URI: uri, Reason: r} }
	switch {
	case uri == "":
		return fail(Required)
	case len(uri) > MaxLength:
		return fail(TooLong)
	case strings.ContainsRune(uri, '*'):
		return fail(Wildcard)
	case strings.ContainsRune(uri, '#'):
		return fail(Fragment)
	}
	u, err := url.Parse(uri)
	if err != nil || !u.IsAbs() || u.Opaque != "" || u.Hostname() == "" || strings.ContainsAny(uri, " \t\r\n\\") {
		return fail(Unparsable)
	}
	if u.User != nil {
		return fail(Userinfo)
	}
	if hasDotSegment(u.EscapedPath()) {
		return fail(DotSegments)
	}
	scheme, host, port := strings.ToLower(u.Scheme), strings.ToLower(u.Hostname()), u.Port()
	if port != "" && !validPort(port) {
		return fail(Unparsable)
	}
	switch appType {
	case Native:
		if scheme != "http" || !IsLoopbackHost(host) {
			return fail(LoopbackOnly)
		}
		if port != "" {
			return fail(PortIgnored)
		}
	case Web:
		if scheme == "http" && (!IsLoopbackHost(host) || port == "" || port == "80") {
			return fail(HTTPSRequired)
		}
		if scheme != "https" && scheme != "http" {
			return fail(HTTPSRequired)
		}
	default:
		return fmt.Errorf("redirecturi: unknown application type %q", appType)
	}
	if c := Canonical(u); c != uri {
		return &Error{URI: uri, Reason: NotCanonical, Canonical: c}
	}
	return nil
}

// ValidateList checks every URI of a list, its size and its duplicates; an empty list is refused only when required.
func ValidateList(appType ApplicationType, uris []string, required bool) error {
	if len(uris) == 0 {
		if required {
			return &Error{Index: -1, Reason: Required}
		}
		return nil
	}
	if len(uris) > MaxURIs {
		return &Error{Index: -1, Reason: TooMany}
	}
	seen := make(map[string]bool, len(uris))
	for i, uri := range uris {
		if err := Validate(appType, uri); err != nil {
			if e, ok := errors.AsType[*Error](err); ok {
				e.Index = i
			}
			return err
		}
		if seen[uri] {
			return &Error{URI: uri, Index: i, Reason: Duplicate}
		}
		seen[uri] = true
	}
	return nil
}

// Match reports whether requested equals registered byte for byte, ignoring a native client's loopback port (RFC 8252 §7.3).
func Match(appType ApplicationType, registered, requested string) bool {
	if registered == "" || requested == "" {
		return false
	}
	if registered == requested {
		return true
	}
	if appType != Native {
		return false
	}
	stripped, ok := withoutLoopbackPort(requested)
	return ok && stripped == registered
}

func withoutLoopbackPort(uri string) (string, bool) {
	const scheme = "http://"
	if !strings.HasPrefix(uri, scheme) {
		return "", false
	}
	rest := uri[len(scheme):]
	end := strings.IndexAny(rest, "/?#")
	if end < 0 {
		end = len(rest)
	}
	authority, tail := rest[:end], rest[end:]
	colon := strings.LastIndexByte(authority, ':')
	if colon < 0 || strings.HasSuffix(authority, "]") {
		return "", false
	}
	host, port := authority[:colon], authority[colon+1:]
	if !validPort(port) || !IsLoopbackHost(strings.Trim(host, "[]")) {
		return "", false
	}
	return scheme + host + tail, true
}

// IsLoopbackHost reports whether host is 127.0.0.1, ::1 or localhost exactly; other 127.x addresses, 0.0.0.0 and *.localhost are not.
func IsLoopbackHost(host string) bool {
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

// Canonical writes u with a lower-case scheme and host, no default port and a / path on a bare host.
func Canonical(u *url.URL) string {
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port := u.Port(); port != "" && (scheme != "https" || port != "443") && (scheme != "http" || port != "80") {
		host = net.JoinHostPort(strings.Trim(host, "[]"), port)
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	out := scheme + "://" + host + path
	if u.RawQuery != "" || u.ForceQuery {
		out += "?" + u.RawQuery
	}
	return out
}

func hasDotSegment(path string) bool {
	for seg := range strings.SplitSeq(path, "/") {
		switch strings.ToLower(seg) {
		case ".", "..", "%2e", "%2e%2e", ".%2e", "%2e.":
			return true
		}
	}
	return false
}

func validPort(port string) bool {
	if port == "" || len(port) > 5 || port[0] == '0' {
		return false
	}
	n := 0
	for _, c := range port {
		if c < '0' || c > '9' {
			return false
		}
		n = n*10 + int(c-'0')
	}
	return n > 0 && n <= 65535
}
