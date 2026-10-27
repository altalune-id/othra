package layout

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func render(t *testing.T, ctx context.Context, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, c.Render(ctx, &buf))
	return strings.Join(strings.Fields(buf.String()), " ")
}

func page(t *testing.T, ctx context.Context, d LayoutData) string {
	t.Helper()
	body := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, "<p>body</p>")
		return err
	})
	card := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return Card(d.Title).Render(templ.WithChildren(ctx, body), w)
	})
	return render(t, templ.WithChildren(ctx, card), Layout(d))
}

func TestReadableOn_EveryColourReachesAA(t *testing.T) {
	colours := []string{"#777777", "#808080", "#ffff00", "#0000ff", "#facc15", "#1e3a8a", OthraPrimary}
	digits := "0123456789abcdef"
	for _, r := range digits {
		for _, g := range digits {
			for _, b := range digits {
				colours = append(colours, fmt.Sprintf("#%c%c%c%c%c%c", r, r, g, g, b, b))
			}
		}
	}
	for _, c := range colours {
		fg := ReadableOn(c)
		assert.GreaterOrEqual(t, contrast(c, fg), 4.5, "%s on %s", fg, c)
	}
}

func TestReadableOn_PicksWhiteOnDarkAndBlackOnLight(t *testing.T) {
	assert.Equal(t, "#ffffff", ReadableOn("#1e3a8a"))
	assert.Equal(t, "#ffffff", ReadableOn(OthraPrimary))
	assert.Equal(t, "#000000", ReadableOn("#facc15"))
	assert.Equal(t, "#000000", ReadableOn("#FFFFFF"))
	assert.Equal(t, "#ffffff", ReadableOn("not a colour"))
}

func TestNewBrand_KeepsOnlyValidHex(t *testing.T) {
	assert.Equal(t, &Brand{Name: "Acme", LogoURL: "/l.png", Primary: "#facc15", OnPrimary: "#000000"}, NewBrand("Acme", "/l.png", "#facc15"))
	for _, bad := range []string{"", "red", "#fff", "#facc15;}", "#gggggg", "facc15"} {
		assert.Equal(t, &Brand{Name: "Acme"}, NewBrand("Acme", "", bad), bad)
	}
}

func TestBrandStyle_WritesOnlyCheckedHexUnderTheNonce(t *testing.T) {
	ctx := templ.WithNonce(t.Context(), "n0nce")
	assert.Equal(t, `<style nonce="n0nce">:root{--color-primary:#1e3a8a;--color-primary-content:#ffffff}</style>`, render(t, ctx, brandStyle(&Brand{Primary: "#1e3a8a", OnPrimary: "#ffffff"})))
	assert.Equal(t, `<style nonce="n0nce">:root{--color-primary:#facc15;--color-primary-content:#000000}</style>`, render(t, ctx, brandStyle(&Brand{Primary: "#facc15", OnPrimary: "#ffffff"})))
	for _, b := range []*Brand{nil, {}, {Primary: "#000000;}*{color:red"}, {Primary: "</style><script>"}} {
		assert.Empty(t, render(t, ctx, brandStyle(b)))
	}
}

func TestLayout_WithoutBrandLooksAsToday(t *testing.T) {
	html := page(t, t.Context(), LayoutData{BasePath: "/x", Title: "Sign in"})
	assert.Contains(t, html, `<html lang="en" data-theme="othra-light">`)
	assert.Contains(t, html, "<title>Sign in · Othra</title>")
	assert.NotContains(t, html, "<style")
	assert.NotContains(t, html, "data-brand")
	assert.NotContains(t, html, "brand-header")
	assert.Contains(t, html, `<h1 class="card-title text-2xl break-words max-sm:text-xl">Sign in</h1><p>body</p>`)
}

func TestLayout_WithBrand(t *testing.T) {
	b := NewBrand("Acme <Store>", "", "#1e3a8a")
	b.TermsURL = "https://acme.test/terms"
	ctx := templ.WithNonce(t.Context(), "n0nce")
	html := page(t, ctx, LayoutData{BasePath: "/x", Title: "Sign in", Lang: "id-ID", Brand: b})

	assert.Contains(t, html, `<html lang="id-ID" data-theme="othra-light" data-brand>`)
	assert.Contains(t, html, "<title>Sign in · Acme &lt;Store&gt;</title>")
	assert.Contains(t, html, `<style nonce="n0nce">:root{--color-primary:#1e3a8a;--color-primary-content:#ffffff}</style></head>`)
	assert.Contains(t, html, `data-testid="brand-header"`)
	assert.Less(t, strings.Index(html, "brand-header"), strings.Index(html, "<h1"), "the brand header tops the card")
	assert.Contains(t, html, `<a class="link" href="https://acme.test/terms" target="_blank" rel="noopener noreferrer">Terms of Service<span class="sr-only"> (opens in a new tab)</span></a>`)
	assert.NotContains(t, html, "Privacy Policy")
	assert.NotContains(t, html, "style=")
}

func TestLayout_BrandWithoutColourOrLinks(t *testing.T) {
	html := page(t, t.Context(), LayoutData{Title: "Sign in", Brand: NewBrand("Acme", "", "")})
	assert.Contains(t, html, "data-brand")
	assert.NotContains(t, html, "<style")
	assert.NotContains(t, html, "<nav")
}

func TestBrandHeader(t *testing.T) {
	logo := render(t, t.Context(), BrandHeader(&Brand{Name: "Acme Store", LogoURL: "/p/acme/logo.png"}))
	assert.Contains(t, logo, `<img src="/p/acme/logo.png" alt="" class="h-10 w-auto max-w-40 object-contain" width="160" height="40">`)
	assert.Contains(t, logo, `<span class="truncate text-sm font-semibold" title="Acme Store">Acme Store</span>`)

	letter := render(t, t.Context(), BrandHeader(&Brand{Name: "émile"}))
	assert.NotContains(t, letter, "<img")
	assert.Contains(t, letter, `aria-hidden="true">É</span>`)

	assert.NotContains(t, render(t, t.Context(), BrandHeader(&Brand{})), "aria-hidden=\"true\">?")
}

func TestCSRFInput(t *testing.T) {
	assert.Equal(t, `<input type="hidden" name="csrf" value="tok&#34;en">`, render(t, t.Context(), CSRFInput(`tok"en`)))
	assert.Empty(t, render(t, t.Context(), CSRFInput("")))
}

func TestMailButton(t *testing.T) {
	html := render(t, t.Context(), MailButton("https://x.test/p/acme/login/verify?token=a&b", "Sign in to Acme", "#facc15"))
	assert.Equal(t, `<a href="https://x.test/p/acme/login/verify?token=a&amp;b" style="display:inline-block;padding:10px 16px;background:#facc15;color:#000000;text-decoration:none;border-radius:6px;font-weight:600;">Sign in to Acme</a>`, html)

	for _, bad := range []string{"", "red;background:url(x)"} {
		assert.Contains(t, render(t, t.Context(), MailButton("https://x.test", "Go", bad)), "background:"+OthraPrimary+";color:#ffffff;", bad)
	}
}

func TestBrandComponentsReturnWriteErrors(t *testing.T) {
	b := NewBrand("Acme", "/logo.png", "#1e3a8a")
	b.TermsURL, b.PrivacyURL = "https://acme.test/t", "https://acme.test/p"
	body := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, "<p>body</p>")
		return err
	})
	card := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return Card("Sign in").Render(templ.WithChildren(ctx, body), w)
	})
	ctx := templ.WithNonce(t.Context(), "n0nce")
	cases := map[string]struct {
		c   templ.Component
		ctx context.Context
	}{
		"branded layout": {Layout(LayoutData{Title: "T", Brand: b}), templ.WithChildren(ctx, card)},
		"logo header":    {BrandHeader(b), ctx},
		"letter header":  {BrandHeader(&Brand{Name: "Acme"}), ctx},
		"footer":         {brandFooter(b), ctx},
		"consent":        {Consent(LayoutData{}, ConsentData{Required: true}), ctx},
		"consent note":   {Consent(LayoutData{}, ConsentData{}), ctx},
		"mail":           {Mail("en-US", "Hi"), templ.WithChildren(ctx, body)},
		"mail button":    {MailButton("https://x.test", "Go", ""), ctx},
		"csrf":           {CSRFInput("tok"), ctx},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, tc.c.Render(tc.ctx, &buf))
			full := buf.Len()
			for limit := range full {
				w := unbufferedTo(&limitWriter{left: limit})
				err := tc.c.Render(tc.ctx, w)
				if err == nil {
					err = w.Flush()
				}
				require.ErrorIs(t, err, errWriteLimit, "limit %d", limit)
			}
			w := unbufferedTo(&limitWriter{left: full})
			require.NoError(t, tc.c.Render(tc.ctx, w))
			require.NoError(t, w.Flush())
			cancelled, cancel := context.WithCancel(tc.ctx)
			cancel()
			assert.Error(t, tc.c.Render(cancelled, &buf))
		})
	}
}

func TestLayout_Refresh(t *testing.T) {
	html := page(t, t.Context(), LayoutData{Title: "T", Refresh: `/p/acme/oauth2/authorize?a=1&b="x"`})
	assert.Contains(t, html, `<meta http-equiv="refresh" content="0;url=/p/acme/oauth2/authorize?a=1&amp;b=&#34;x&#34;">`)
	assert.Less(t, strings.Index(html, "http-equiv"), strings.Index(html, "</head>"))
	assert.NotContains(t, page(t, t.Context(), LayoutData{Title: "T"}), "http-equiv", "no refresh unless asked")
}

func TestCardWith_PutsTheLeadInTheTitleRow(t *testing.T) {
	lead := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, `<span class="mark">C</span>`)
		return err
	})
	html := render(t, t.Context(), CardWith("Allow it?", lead))
	assert.Contains(t, html, `<div class="flex items-center gap-3"><span class="mark">C</span><h1 class="card-title text-2xl break-words max-sm:text-xl">Allow it?</h1></div>`)
	assert.Equal(t, render(t, t.Context(), Card("Plain")), render(t, t.Context(), CardWith("Plain", nil)), "Card is CardWith without a lead")
}

func TestBrand_CSS(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ":root{--color-primary:#1e3a8a;--color-primary-content:#ffffff}", NewBrand("Acme", "", "#1e3a8a").CSS())
	assert.Empty(t, NewBrand("Acme", "", "red").CSS())
	assert.Empty(t, (*Brand)(nil).CSS())
}
