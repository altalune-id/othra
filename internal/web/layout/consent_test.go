package layout

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/web/i18n"
)

func renderConsent(t *testing.T, c ConsentData) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, Consent(LayoutData{BasePath: "/x"}, c).Render(t.Context(), &buf))
	return strings.Join(strings.Fields(buf.String()), " ")
}

func TestConsent_Required(t *testing.T) {
	html := renderConsent(t, ConsentData{Required: true})
	assert.Contains(t, html, `type="checkbox" name="accept_terms" value="1" required`)
	assert.Contains(t, html, "I agree to the ")
	assert.Contains(t, html, `href="/x/terms" target="_blank" rel="noopener noreferrer">Terms of Service<span class="sr-only"> (opens in a new tab)</span></a> and the `)
	assert.Contains(t, html, `href="/x/privacy" target="_blank" rel="noopener noreferrer">Privacy Policy<span class="sr-only"> (opens in a new tab)</span></a>.`)
}

func TestConsent_NotRequired(t *testing.T) {
	html := renderConsent(t, ConsentData{TermsURL: "https://legal.example.com/tos", PrivacyURL: "https://legal.example.com/pp"})
	assert.NotContains(t, html, "checkbox")
	assert.NotContains(t, html, "accept_terms")
	assert.Contains(t, html, "By continuing, you agree to the ")
	assert.Contains(t, html, `href="https://legal.example.com/tos" target="_blank" rel="noopener noreferrer">Terms of Service`)
	assert.Contains(t, html, `href="https://legal.example.com/pp" target="_blank" rel="noopener noreferrer">Privacy Policy`)
}

func TestConsent_BrandedHidesAnEmptyLink(t *testing.T) {
	render := func(c ConsentData) string {
		var buf bytes.Buffer
		require.NoError(t, Consent(LayoutData{BasePath: "/x", Brand: &Brand{Name: "Acme"}}, c).Render(t.Context(), &buf))
		return strings.Join(strings.Fields(buf.String()), " ")
	}

	termsOnly := render(ConsentData{TermsURL: "https://acme.test/terms", Required: true})
	assert.Contains(t, termsOnly, `I&#39;ve read and agree to the <a class="link" href="https://acme.test/terms"`)
	assert.Contains(t, termsOnly, `(opens in a new tab)</span></a>.</span>`)
	assert.NotContains(t, termsOnly, "and the")
	assert.NotContains(t, termsOnly, "/x/privacy")

	privacyOnly := render(ConsentData{PrivacyURL: "https://acme.test/privacy"})
	assert.Contains(t, privacyOnly, `agree to the <a class="link" href="https://acme.test/privacy" target="_blank" rel="noopener noreferrer">Privacy Policy`)
	assert.NotContains(t, privacyOnly, "/x/terms")
}

func TestConsent_BrandedSpeaksThePageLanguage(t *testing.T) {
	var buf bytes.Buffer
	ctx := i18n.WithLocale(t.Context(), i18n.Indonesian)
	d := LayoutData{Brand: &Brand{Name: "Acme"}}
	require.NoError(t, Consent(d, ConsentData{TermsURL: "https://acme.test/t", PrivacyURL: "https://acme.test/p", Required: true}).Render(ctx, &buf))
	html := strings.Join(strings.Fields(buf.String()), " ")

	assert.Contains(t, html, `Saya telah membaca dan menyetujui <a class="link" href="https://acme.test/t" target="_blank" rel="noopener noreferrer">Ketentuan Layanan<span class="sr-only"> (dibuka di tab baru)</span></a> dan <a class="link" href="https://acme.test/p"`)
	assert.Contains(t, html, `Kebijakan Privasi<span class="sr-only"> (dibuka di tab baru)</span></a>.</span>`)
}

func TestSplitSentence(t *testing.T) {
	links := map[string]sentencePart{"{a}": {Text: "A", URL: "/a"}, "{b}": {Text: "B", URL: "/b"}}
	assert.Equal(t, []sentencePart{{Text: "see "}, {Text: "B", URL: "/b"}, {Text: " then "}, {Text: "A", URL: "/a"}}, splitSentence("see {b} then {a}", links))
	assert.Equal(t, []sentencePart{{Text: "A", URL: "/a"}}, splitSentence("{a}", links))
	assert.Equal(t, []sentencePart{{Text: "plain"}}, splitSentence("plain", links))
	assert.Empty(t, splitSentence("", links))
}
