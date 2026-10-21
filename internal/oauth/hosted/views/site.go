// Package views holds the templ pages and mail of the hosted end-user sign-in.
package views

import (
	"net/url"
	"strings"

	"altalune.id/othra/internal/web/layout"
)

// Site names the project a hosted view belongs to: its issuer handle for paths and its display name for copy.
type Site struct {
	Handle  string
	Project string
}

func (s Site) href(d layout.LayoutData, sub string) string {
	return d.Href("/p/" + s.Handle + sub)
}

func (s Site) login(d layout.LayoutData, query url.Values) string {
	for k, v := range query {
		if len(v) == 0 || v[0] == "" {
			delete(query, k)
		}
	}
	if len(query) == 0 {
		return s.href(d, "/login")
	}
	return s.href(d, "/login") + "?" + query.Encode()
}

func (s Site) addAccount(d layout.LayoutData, returnTo string) string {
	return s.login(d, url.Values{"add": {"1"}, "return_to": {returnTo}})
}

func withoutLegalFooter(d layout.LayoutData) layout.LayoutData {
	if d.Brand != nil {
		b := *d.Brand
		b.TermsURL, b.PrivacyURL = "", ""
		d.Brand = &b
	}
	return d
}

func titled(d layout.LayoutData, title string) layout.LayoutData {
	d.Title = title
	return d
}

func initial(email string) string {
	for _, r := range email {
		return strings.ToUpper(string(r))
	}
	return ""
}

func splitAround(msg, placeholder string) []string {
	before, after, found := strings.Cut(msg, placeholder)
	if !found {
		return []string{msg}
	}
	return []string{before, placeholder, after}
}
