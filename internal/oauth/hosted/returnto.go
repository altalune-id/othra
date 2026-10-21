package hosted

import (
	"net/url"
	"strings"

	"altalune.id/othra/internal/web/layout"
)

// ReturnTo returns raw rebuilt from its path and query when it names this issuer's authorize or account page, else the account page.
func ReturnTo(issuer, raw string) string {
	base := issuerPath(issuer)
	fallback := base + "/account/"
	if raw == "" || strings.ContainsFunc(raw, func(r rune) bool { return r < 0x20 || r == 0x7f || r == '\\' }) {
		return fallback
	}
	clean := layout.SanitizeReturnTo(base, raw)
	if clean == "" {
		return fallback
	}
	u, err := url.Parse(clean)
	if err != nil {
		return fallback
	}
	escaped := strings.ToLower(u.EscapedPath())
	if strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") || dotSegment(u.Path) {
		return fallback
	}
	if u.Path != base+"/oauth2/authorize" && !strings.HasPrefix(u.Path, fallback) {
		return fallback
	}
	out := &url.URL{Path: u.Path, RawQuery: u.Query().Encode()}
	return out.String()
}

func issuerPath(issuer string) string {
	u, err := url.Parse(issuer)
	if err != nil {
		return ""
	}
	return strings.TrimRight(u.Path, "/")
}

func dotSegment(path string) bool {
	for seg := range strings.SplitSeq(path, "/") {
		if seg == "." || seg == ".." {
			return true
		}
	}
	return false
}
