package i18n

import (
	"context"
	"encoding/json"
	"io/fs"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
)

func TestPick(t *testing.T) {
	for header, want := range map[string]string{
		"":                               English,
		"id-ID":                          Indonesian,
		"id":                             Indonesian,
		"ID-id":                          Indonesian,
		"en-GB,en;q=0.9":                 English,
		"fr-FR,id;q=0.5,en;q=0.4":        Indonesian,
		"en;q=0.4,id;q=0.8":              Indonesian,
		"id;q=0,en":                      English,
		"ja-JP":                          English,
		"*":                              English,
		"id;q=bogus,en":                  English,
		" id-ID ; q=0.9 , en-US ; q=0.8": Indonesian,
	} {
		assert.Equal(t, want, Pick(header), "%q", header)
	}
}

func TestMiddleware(t *testing.T) {
	var got string
	h := Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = Locale(r.Context()) }))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, Indonesian, got)
	assert.Equal(t, "Accept-Language", rec.Header().Get("Vary"))
}

func TestLocale(t *testing.T) {
	assert.Equal(t, English, Locale(context.Background()))
	assert.Equal(t, Indonesian, Locale(WithLocale(context.Background(), Indonesian)))
	assert.Equal(t, English, Locale(WithLocale(context.Background(), "fr-FR")), "an unsupported locale falls back")
}

func TestT(t *testing.T) {
	en, id := WithLocale(context.Background(), English), WithLocale(context.Background(), Indonesian)
	assert.Equal(t, "Sign in to Openwa", T(en, "verify.title", "project", "Openwa"))
	assert.Equal(t, "Masuk ke Openwa", T(id, "verify.title", "project", "Openwa"))
	assert.Equal(t, "We sent a sign-in link to a@example.com. It works once, for 15 minutes.",
		T(en, "sent.body", "email", "a@example.com", "minutes", "15"))
	assert.Equal(t, "Masuk ke {project}", T(id, "verify.title"), "no args leaves the placeholder")
	assert.Equal(t, "Masuk ke {email}", T(id, "verify.title", "project", "{email}", "email", "x"), "a value is never expanded again")
	assert.Equal(t, "nope.missing", T(en, "nope.missing"))
	assert.Equal(t, "Welcome", T(en, "login.title", "dangling"))
}

func TestError(t *testing.T) {
	en, id := WithLocale(context.Background(), English), WithLocale(context.Background(), Indonesian)
	assert.Equal(t, T(en, "apperror.GEN900"), Error(en, apperror.CodeUnexpected))
	assert.Equal(t, T(id, "apperror.SIG005"), Error(id, apperror.CodeAuthorizeClientUnknown))
	assert.NotEqual(t, Error(en, apperror.CodeAuthorizeClientUnknown), Error(id, apperror.CodeAuthorizeClientUnknown))
	assert.Equal(t, T(en, "apperror.GEN900", "project", "Acme"), Error(en, apperror.CodeUnexpected, "project", "Acme"))
}

var placeholder = regexp.MustCompile(`\{[a-z_]+\}`)

// every locale has the same keys, no empty message, and the same placeholders per key.
func TestCatalogs_Parity(t *testing.T) {
	c := catalogs()
	base := slices.Sorted(maps.Keys(c[Default]))
	require.NotEmpty(t, base)
	for _, locale := range Locales() {
		keys := slices.Sorted(maps.Keys(c[locale]))
		assert.Equal(t, base, keys, "%s keys", locale)
		for k, msg := range c[locale] {
			assert.NotEmpty(t, strings.TrimSpace(msg), "%s %s", locale, k)
			assert.ElementsMatch(t, placeholder.FindAllString(c[Default][k], -1), placeholder.FindAllString(msg, -1), "%s %s placeholders", locale, k)
		}
	}
}

var tCall = regexp.MustCompile(`i18n\.T\(\s*[A-Za-z_.()]+\s*,\s*"([^"]+)"`)

var errorCall = regexp.MustCompile(`i18n\.Error\(\s*[A-Za-z_.()]+\s*,\s*apperror\.Code([A-Za-z0-9]+)\)`)

func codesByName(t *testing.T, root string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "apperror", "codes.json"))
	require.NoError(t, err)
	var entries []struct{ Code, Name string }
	require.NoError(t, json.Unmarshal(raw, &entries))
	out := map[string]string{}
	for _, e := range entries {
		out[e.Name] = e.Code
	}
	return out
}

// every key a Go or templ file asks for exists in the catalogs.
func TestCatalogs_HaveEveryUsedKey(t *testing.T) {
	root, err := filepath.Abs("../../..")
	require.NoError(t, err)
	codes := codesByName(t, root)
	var missing []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == ".git" || d.Name() == "gen") {
			return filepath.SkipDir
		}
		if d.IsDir() || (!strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".templ")) || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range tCall.FindAllSubmatch(body, -1) {
			if _, ok := catalogs()[Default][string(m[1])]; !ok {
				missing = append(missing, string(m[1])+" in "+path)
			}
		}
		for _, m := range errorCall.FindAllSubmatch(body, -1) {
			key := errorKeyPrefix + codes[string(m[1])]
			if _, ok := catalogs()[Default][key]; !ok {
				missing = append(missing, key+" (Code"+string(m[1])+") in "+path)
			}
		}
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, missing)
}

// NOTE: the hosted pages name the project where the console's shared catalog (ui/base) says "this project"; every other error text is the same.
func TestCatalogs_ErrorsMatchTheSharedUICatalog(t *testing.T) {
	for _, locale := range Locales() {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "ui", "base", "i18n", "locales", locale+".json"))
		require.NoError(t, err)
		var shared map[string]string
		require.NoError(t, json.Unmarshal(raw, &shared))
		for key, msg := range catalogs()[locale] {
			if !strings.HasPrefix(key, "apperror.") {
				continue
			}
			want, ok := shared[key]
			if !assert.True(t, ok, "%s %s is not in ui/base", locale, key) || strings.Contains(msg, "{project}") {
				continue
			}
			assert.Equal(t, want, msg, "%s %s differs from ui/base", locale, key)
		}
	}
}
