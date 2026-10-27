// Package layout is the shared shell of every server-rendered view: the layout, rendering and the error view.
package layout

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/a-h/templ"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/web/i18n"
)

// LayoutData is what every page component receives.
type LayoutData struct {
	BasePath string
	Title    string
	Lang     string
	Brand    *Brand
	Home     string
	CSRF     string
	Refresh  string
}

// Href returns sub (which starts with "/") under the base path.
func (d LayoutData) Href(sub string) string {
	return strings.TrimRight(d.BasePath, "/") + sub
}

// Static returns the URL of an embedded static asset.
func (d LayoutData) Static(sub string) string {
	return d.Href("/static" + sub)
}

func (d LayoutData) siteName() string {
	if d.Brand != nil && d.Brand.Name != "" {
		return d.Brand.Name
	}
	return "Othra"
}

func (d LayoutData) lang() string {
	if d.Lang == "" {
		return "en"
	}
	return d.Lang
}

// Kit renders pages for one base path; a domain's web adapter holds one.
type Kit struct {
	BasePath   string
	Log        *slog.Logger
	Unexpected apperror.UnexpectedFunc
}

// Path returns sub under the kit's base path.
func (k Kit) Path(sub string) string { return Path(k.BasePath, sub) }

// Layout returns the LayoutData for an admin page titled title, whose error page links to the console.
func (k Kit) Layout(title string) LayoutData {
	return LayoutData{BasePath: k.BasePath, Title: title, Home: k.Path("/console") + "/"}
}

// Render writes c as an HTML response with status.
func (k Kit) Render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := c.Render(r.Context(), w); err != nil && !isClientGone(r.Context()) {
		k.log().ErrorContext(r.Context(), "web: render failed", slog.Any("err", err))
	}
}

// Fail reports err and renders the generic 500 page. SECURITY: err never reaches the response.
func (k Kit) Fail(w http.ResponseWriter, r *http.Request, msg string, err error) {
	if k.Unexpected != nil {
		_ = k.Unexpected(r.Context(), msg, err)
	}
	k.Render(w, r, http.StatusInternalServerError, Error(k.Layout("Something went wrong"), "Please try again in a moment."))
}

func (k Kit) log() *slog.Logger {
	if k.Log == nil {
		return slog.Default()
	}
	return k.Log
}

func isClientGone(ctx context.Context) bool { return ctx.Err() != nil }

type sentencePart struct {
	Text string
	URL  string
}

func consentSentence(ctx context.Context, d LayoutData, c ConsentData) []sentencePart {
	terms, privacy := consentURL(d, c.TermsURL, "/terms"), consentURL(d, c.PrivacyURL, "/privacy")
	links := map[string]sentencePart{
		"{terms}":   {Text: i18n.T(ctx, "terms.terms_link"), URL: terms},
		"{privacy}": {Text: i18n.T(ctx, "terms.privacy_link"), URL: privacy},
	}
	var msg string
	switch {
	case d.Brand == nil && c.Required:
		msg = "I agree to the {terms} and the {privacy}."
	case d.Brand == nil:
		msg = "By continuing, you agree to the {terms} and the {privacy}."
	case terms != "" && privacy != "":
		msg = i18n.T(ctx, "terms.agree_both")
	case terms != "":
		msg = i18n.T(ctx, "terms.agree_terms_only")
	default:
		msg = i18n.T(ctx, "terms.agree_terms_only")
		links["{terms}"] = links["{privacy}"]
	}
	return splitSentence(msg, links)
}

func splitSentence(msg string, links map[string]sentencePart) []sentencePart {
	var parts []sentencePart
	for msg != "" {
		at, name := len(msg), ""
		for k := range links {
			if i := strings.Index(msg, k); i >= 0 && i < at {
				at, name = i, k
			}
		}
		if at > 0 {
			parts = append(parts, sentencePart{Text: msg[:at]})
		}
		if name == "" {
			break
		}
		parts = append(parts, links[name])
		msg = msg[at+len(name):]
	}
	return parts
}

func consentURL(d LayoutData, configured, builtIn string) string {
	if configured != "" || d.Brand != nil {
		return configured
	}
	return d.Href(builtIn)
}
