// Package i18n holds the message catalogs of the hosted pages and mail, and picks a locale from Accept-Language.
package i18n

import (
	"cmp"
	"context"
	"embed"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	"altalune.id/othra/apperror"
)

// The supported locales; Default is the fallback.
const (
	English    = "en-US"
	Indonesian = "id-ID"
	Default    = English
)

//go:embed locales/*.json
var files embed.FS

//nolint:gochecknoglobals // embedded catalogs, parsed once.
var catalogs = sync.OnceValue(func() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, locale := range Locales() {
		raw, err := files.ReadFile("locales/" + locale + ".json")
		if err != nil {
			panic(err)
		}
		msgs := map[string]string{}
		if err := json.Unmarshal(raw, &msgs); err != nil {
			panic("i18n: " + locale + ": " + err.Error())
		}
		out[locale] = msgs
	}
	return out
})

// Locales returns the supported locales, the default first.
func Locales() []string { return []string{English, Indonesian} }

type ctxKey struct{}

// WithLocale returns ctx carrying locale, or the default when locale is not supported.
func WithLocale(ctx context.Context, locale string) context.Context {
	if !slices.Contains(Locales(), locale) {
		locale = Default
	}
	return context.WithValue(ctx, ctxKey{}, locale)
}

// Locale returns the locale on ctx, or the default.
func Locale(ctx context.Context) string {
	if l, ok := ctx.Value(ctxKey{}).(string); ok {
		return l
	}
	return Default
}

// Error returns the message for an error code in the locale on ctx.
func Error(ctx context.Context, code apperror.Code, args ...string) string {
	return T(ctx, errorKeyPrefix+string(code), args...)
}

const errorKeyPrefix = "apperror."

// Middleware puts the locale picked from Accept-Language on the request context.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Language")
		next.ServeHTTP(w, r.WithContext(WithLocale(r.Context(), Pick(r.Header.Get("Accept-Language")))))
	})
}

// Pick returns the supported locale that best matches an Accept-Language header (RFC 9110 §12.5.4), or the default.
func Pick(header string) string {
	type choice struct {
		tag string
		q   float64
	}
	var choices []choice
	for part := range strings.SplitSeq(header, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			parsed, err := strconv.ParseFloat(v, 64)
			if err != nil {
				continue
			}
			q = parsed
		}
		if tag != "" && q > 0 {
			choices = append(choices, choice{tag: strings.ToLower(tag), q: q})
		}
	}
	slices.SortStableFunc(choices, func(a, b choice) int { return cmp.Compare(b.q, a.q) })
	for _, c := range choices {
		if l := match(c.tag); l != "" {
			return l
		}
	}
	return Default
}

func match(tag string) string {
	if tag == "*" {
		return Default
	}
	primary, _, _ := strings.Cut(tag, "-")
	for _, l := range Locales() {
		lp, _, _ := strings.Cut(strings.ToLower(l), "-")
		if strings.EqualFold(tag, l) || primary == lp {
			return l
		}
	}
	return ""
}

// T returns the message key in ctx's locale with each {name} replaced; args are name, value pairs. A missing key returns the key.
func T(ctx context.Context, key string, args ...string) string {
	msg, ok := catalogs()[Locale(ctx)][key]
	if !ok {
		if msg, ok = catalogs()[Default][key]; !ok {
			return key
		}
	}
	if len(args) < 2 {
		return msg
	}
	pairs := make([]string, 0, len(args))
	for i := 0; i+1 < len(args); i += 2 {
		pairs = append(pairs, "{"+args[i]+"}", args[i+1])
	}
	return strings.NewReplacer(pairs...).Replace(msg)
}
