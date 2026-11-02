package views

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/web/i18n"
	"github.com/altalune-id/othra/internal/web/layout"
)

var site = Site{Handle: "acme", Project: "Acme Store"}

func ctxIn(t *testing.T, locale string) context.Context {
	t.Helper()
	return templ.WithNonce(i18n.WithLocale(t.Context(), locale), "n0nce")
}

func page() layout.LayoutData {
	b := layout.NewBrand("Acme Store", "", "#1e3a8a")
	return layout.LayoutData{BasePath: "/id", Lang: i18n.English, Brand: b}
}

func html(t *testing.T, ctx context.Context, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, c.Render(ctx, &buf))
	return strings.Join(strings.Fields(buf.String()), " ")
}

func TestLogin(t *testing.T) {
	out := html(t, ctxIn(t, i18n.English), Login(page(), LoginData{Site: site, ReturnTo: "/id/p/acme/account/", LoginToken: "tok-login"}))
	assert.Contains(t, out, "<title>Sign in to Acme Store · Acme Store</title>")
	assert.Contains(t, out, `<form method="post" action="/id/p/acme/login"`)
	assert.Contains(t, out, `<input type="hidden" name="csrf" value="tok-login">`)
	assert.Contains(t, out, `<input type="hidden" name="return_to" value="/id/p/acme/account/">`)
	assert.Contains(t, out, `type="email" name="email" value="" placeholder="you@example.com" autocomplete="email" inputmode="email" spellcheck="false" required autofocus aria-describedby="email-hint">`)
	assert.Contains(t, out, `<p id="email-hint" class="validator-hint hidden">Enter an email address, for example name@example.com.</p>`, "a malformed email gets words, not only a red border")
	assert.Contains(t, out, ">Sign in to Acme Store</h1>")
	assert.Contains(t, out, `class="input validator w-full" type="email"`, "the browser marks a malformed email as the person types")
	assert.Contains(t, out, `<button class="btn btn-primary w-full" type="submit">Continue with email</button>`)
	assert.NotContains(t, out, "aria-invalid")
	assert.NotContains(t, out, "Back to accounts")
	assert.NotContains(t, out, `role="status"`)
}

func TestLogin_SignedOutNotice(t *testing.T) {
	for locale, want := range map[string]string{
		i18n.English:    `<span class="break-all">bob@example.com</span> is signed out on this browser.</span></div>`,
		i18n.Indonesian: `<span class="break-all">bob@example.com</span> sudah keluar di browser ini.</span></div>`,
	} {
		out := html(t, ctxIn(t, locale), Login(page(), LoginData{Site: site, Notice: "bob@example.com"}))
		assert.Contains(t, out, want, "the signed-out notice in %s", locale)
	}
}

func TestLogin_ErrorAndBackToAccounts(t *testing.T) {
	out := html(t, ctxIn(t, i18n.Indonesian), Login(page(), LoginData{Site: site, Email: "nope", Error: "Masukkan alamat email yang valid.", FromPicker: true, ReturnTo: "/id/p/acme/account/"}))
	assert.Contains(t, out, `value="nope"`)
	assert.Contains(t, out, `aria-invalid="true" aria-describedby="email-hint email-error"`)
	assert.Contains(t, out, ">Masukkan alamat email, misalnya nama@example.com.</p>")
	assert.Contains(t, out, `<p id="email-error" class="flex items-start gap-1.5 text-sm text-base-content">`)
	assert.Contains(t, out, `href="/id/p/acme/login?return_to=%2Fid%2Fp%2Facme%2Faccount%2F">← Kembali ke daftar akun</a>`)
	assert.Contains(t, out, "Lanjutkan dengan email")
	assert.NotContains(t, out, `name="csrf"`, "no token, no field")
}

func TestPicker(t *testing.T) {
	v := PickerData{
		Site:         site,
		Accounts:     []AccountTile{{SessionID: "us1", Email: "ana@acme.test"}, {SessionID: "us2", Email: "budi@example.com"}},
		ReturnTo:     "/id/p/acme/oauth2/authorize?x=1",
		SelectToken:  "tok-select",
		SignOutToken: "tok-signout",
		Notice:       "cici@acme.test",
	}
	out := html(t, ctxIn(t, i18n.English), Picker(page(), v))
	assert.Equal(t, 2, strings.Count(out, `action="/id/p/acme/login/select"`))
	assert.Equal(t, 2, strings.Count(out, `<input type="hidden" name="csrf" value="tok-select">`))
	assert.Equal(t, 2, strings.Count(out, `<input type="hidden" name="csrf" value="tok-signout">`))
	assert.Contains(t, out, `<input type="hidden" name="user_session" value="us2">`)
	assert.Contains(t, out, `aria-label="Continue as ana@acme.test"`)
	assert.Contains(t, out, `aria-label="Sign out budi@example.com" title="Sign out budi@example.com"`)
	assert.Contains(t, out, `<span class="break-all">cici@acme.test</span> is signed out on this browser.</span></div>`)
	assert.Contains(t, out, `href="/id/p/acme/login?add=1&amp;return_to=%2Fid%2Fp%2Facme%2Foauth2%2Fauthorize%3Fx%3D1"`)
	assert.Less(t, strings.Index(out, "us1"), strings.Index(out, "us2"), "the handler's order is kept")
	assert.NotContains(t, out, "autofocus")
	assert.NotContains(t, out, `name="next"`, "a picker sign-out returns to the picker")
}

func TestSent(t *testing.T) {
	out := strings.ReplaceAll(html(t, ctxIn(t, i18n.English), Sent(page(), SentData{Site: site, Email: "ana@acme.test", Minutes: 15, ResendToken: "tok-resend", Resent: true})), "> <", "><")
	assert.Contains(t, out, `<h1 class="text-2xl font-semibold text-balance break-words">Check your inbox</h1><p>We sent a sign-in link to <span class="font-medium break-all">ana@acme.test</span>.</p></div>`, "the address sits in the header, under the title")
	assert.Regexp(t, `<li class="flex gap-2"><svg[^>]*><circle cx="12" cy="12" r="10"></circle><path d="M12 6v6h4"></path></svg><span>The link works once, for 15 minutes.</span></li>`, out, "the expiry, from the server's link lifetime")
	assert.Contains(t, out, "<span>Not there after a minute? Check your spam folder.</span>")
	assert.Equal(t, 3, strings.Count(out, `<li class="flex gap-2">`), "the design's two hints plus the device hint")
	assert.Contains(t, out, "<span>Open it on the device where you want to be signed in.</span></li></ul>", "the device hint comes last")
	assert.Contains(t, out, `<form method="post" action="/id/p/acme/login/resend">`)
	assert.Contains(t, out, `<input type="hidden" name="csrf" value="tok-resend"><input type="hidden" name="email" value="ana@acme.test">`)
	assert.Contains(t, out, `<button class="btn btn-primary" type="submit">Send the link again</button>`, "no countdown: the server exposes no resend wait")
	assert.Contains(t, out, `role="status"`)
	assert.Contains(t, out, `href="/id/p/acme/login?add=1">Wrong email?</a>`)
	assert.NotContains(t, out, "It opens", "no app, no app sentence")

	noCookie := html(t, ctxIn(t, i18n.Indonesian), Sent(page(), SentData{Site: site, Minutes: 15}))
	assert.Contains(t, noCookie, "<p>Kami sudah mengirim tautan masuk.</p>")
	assert.Contains(t, noCookie, "Tautan hanya bisa dipakai sekali, selama 15 menit.")
	assert.Contains(t, noCookie, ">Salah email?</a>")
	assert.NotContains(t, noCookie, "/login/resend")
	assert.NotContains(t, noCookie, `role="status"`)

	app := html(t, ctxIn(t, i18n.English), Sent(page(), SentData{Site: site, Email: "ana@acme.test", Minutes: 15, Client: "OAgent"}))
	assert.Contains(t, app, `ana@acme.test</span>. It opens OAgent.</p>`, "the body names the app the link returns to")
	assert.NotContains(t, app, "Continue to OAgent", "no subtitle")
	id := html(t, ctxIn(t, i18n.Indonesian), Sent(page(), SentData{Site: site, Minutes: 15, Client: "OAgent"}))
	assert.Contains(t, id, "Kami sudah mengirim tautan masuk. Tautan ini membuka OAgent.</p>")
}

func TestVerifyAndLinkUnusable(t *testing.T) {
	out := html(t, ctxIn(t, i18n.English), Verify(page(), VerifyData{Site: site, Email: "ana@acme.test", VerifyToken: "tok-verify"}))
	assert.Contains(t, out, "<title>Confirm sign-in · Acme Store</title>", "the tab does not repeat the project")
	assert.Contains(t, out, ">Sign in to Acme Store</h1>")
	assert.Contains(t, out, `You&#39;re signing in as <span class="break-all">ana@acme.test</span> on this device.`)
	assert.Contains(t, out, `<form method="post" action="/id/p/acme/login/verify"><input type="hidden" name="csrf" value="tok-verify">`)
	assert.Contains(t, out, `type="submit" autofocus>Continue</button>`)
	assert.NotContains(t, out, "token=", "the link token never reaches the page")

	app := html(t, ctxIn(t, i18n.English), Verify(page(), VerifyData{Site: site, Email: "ana@acme.test", Client: "OAgent"}))
	assert.Contains(t, app, ">Sign in to OAgent</h1>", "the app, not the project, names the sign-in")

	bad := html(t, ctxIn(t, i18n.Indonesian), LinkUnusable(page(), LinkUnusableData{Site: site, ReturnTo: "/id/p/acme/account/"}))
	assert.Contains(t, bad, "Tautan ini tidak bisa dipakai")
	assert.Contains(t, bad, `href="/id/p/acme/login?return_to=%2Fid%2Fp%2Facme%2Faccount%2F">Minta tautan baru</a>`)
}

func TestTerms(t *testing.T) {
	v := TermsData{Site: site, Email: "ana@acme.test", UserSession: "us1", TermsToken: "tok-terms", Version: 3, TermsURL: "https://acme.test/terms", Error: "Tick the box to accept the terms."}
	fresh := html(t, ctxIn(t, i18n.English), Terms(page(), v))
	assert.Contains(t, fresh, "<title>Welcome · Acme Store</title>")
	assert.Contains(t, fresh, "One quick step before we create your Acme Store account.")
	assert.Contains(t, fresh, `<input type="hidden" name="user_session" value="us1">`)
	assert.Contains(t, fresh, `<input type="hidden" name="terms_version" value="3">`, "the version shown is the version accepted")
	assert.Contains(t, fresh, `<input type="hidden" name="csrf" value="tok-terms">`)
	assert.Contains(t, fresh, `name="accept_terms" value="1" required`)
	assert.Contains(t, fresh, `href="https://acme.test/terms" target="_blank" rel="noopener noreferrer">Terms of Service`)
	assert.NotContains(t, fresh, "Privacy Policy", "a project without a privacy URL shows only its terms")
	assert.Contains(t, fresh, `<span>Tick the box to accept the terms.</span></div>`)
	assert.Contains(t, fresh, ">Create account</button>")
	assert.Contains(t, fresh, `href="/id/p/acme/login?add=1">Use another account</a>`)

	v.Updated, v.Error, v.PrivacyURL = true, "", "https://acme.test/privacy"
	updated := html(t, ctxIn(t, i18n.Indonesian), Terms(page(), v))
	assert.Contains(t, updated, "Ketentuan yang diperbarui")
	assert.Contains(t, updated, "Acme Store telah memperbarui ketentuannya.")
	assert.Contains(t, updated, "Kebijakan Privasi")
	assert.Contains(t, updated, ">Setujui dan lanjutkan</button>")
	assert.NotContains(t, updated, `role="alert"`)
}

func TestSignedOut(t *testing.T) {
	gone := html(t, ctxIn(t, i18n.Indonesian), SignedOut(page(), SignedOutData{Site: site}))
	assert.Contains(t, gone, "Anda sudah keluar dari Acme Store di browser ini.")
	assert.Contains(t, gone, `href="/id/p/acme/login">Masuk lagi</a>`)
}

func TestViews_EscapeEndUserText(t *testing.T) {
	evil := `"><script>x</script>`
	for name, c := range map[string]templ.Component{
		"login":  Login(page(), LoginData{Site: Site{Handle: "acme", Project: evil}, Email: evil, Error: evil}),
		"picker": Picker(page(), PickerData{Site: site, Accounts: []AccountTile{{SessionID: evil, Email: evil}}, Notice: evil}),
		"sent":   Sent(page(), SentData{Site: site, Email: evil}),
		"terms":  Terms(page(), TermsData{Site: site, Email: evil, UserSession: evil}),
	} {
		assert.NotContains(t, html(t, ctxIn(t, i18n.English), c), "<script>", name)
	}
}

func TestMail(t *testing.T) {
	m := MailData{Project: "Acme Store", Link: "https://id.example.com/p/acme/login/verify?token=abc", Minutes: 15, HasReplyTo: true, Primary: "#facc15"}
	en := ctxIn(t, i18n.English)
	assert.Equal(t, "Sign in to Acme Store", MailSubject(en, m))
	app := m
	app.Client = "OAgent"
	assert.Equal(t, "Sign in to OAgent", MailSubject(en, app), "the subject names the app the person returns to")
	assert.Equal(t, "Someone asked to sign in to Acme Store with this email address.\n\n"+
		"Open this link to sign in. It works once, for 15 minutes:\n"+
		"https://id.example.com/p/acme/login/verify?token=abc\n\n"+
		"If you didn't ask for this, you can ignore this email.\n\n"+
		"Questions? Reply to this email.\n"+
		"Sent by Othra for Acme Store.\n", MailText(en, m))

	out := html(t, en, MailHTML(m))
	assert.Contains(t, out, `<html lang="en-US">`)
	assert.Contains(t, out, `style="display:inline-block;padding:10px 16px;background:#facc15;color:#000000;`)
	assert.Contains(t, out, ">Sign in</a>")
	assert.Contains(t, out, "Questions? Reply to this email.")

	signUp := MailData{Project: "Acme Store", Link: "https://x.test/v", Minutes: 15, SignUp: true}
	id := ctxIn(t, i18n.Indonesian)
	assert.Equal(t, "Selesaikan pendaftaran di Acme Store", MailSubject(id, signUp))
	assert.NotContains(t, MailText(id, signUp), "Balas email ini")
	idHTML := html(t, id, MailHTML(signUp))
	assert.Contains(t, idHTML, `<html lang="id-ID">`)
	assert.Contains(t, idHTML, "background:"+layout.OthraPrimary+";color:#ffffff;")
	assert.NotContains(t, idHTML, "Balas email ini")
}

func TestSiteHrefs(t *testing.T) {
	d := layout.LayoutData{BasePath: "/"}
	assert.Equal(t, "/p/acme/login", site.href(d, "/login"))
	assert.Equal(t, "/p/acme/login", site.addAccount(d, "")[:len("/p/acme/login")])
	assert.Equal(t, "/p/acme/login?add=1", site.addAccount(d, ""))
	assert.Empty(t, initial(""))
	assert.Equal(t, "É", initial("émile@x.test"))
	assert.Equal(t, "P", initial("product-ana@acme.test"), "the avatar letter is upper-case")
}

func TestError(t *testing.T) {
	v := ErrorData{Site: site, Title: "Too many attempts", Message: "Too many attempts. Try again in 60 seconds.", Code: "AUT003", Reference: "req-123"}
	out := html(t, ctxIn(t, i18n.English), Error(page(), v))
	assert.Contains(t, out, "<title>Too many attempts · Acme Store</title>")
	assert.Contains(t, out, `<p class="break-words">Too many attempts. Try again in 60 seconds.</p>`)
	assert.Contains(t, out, `<dt>Error code</dt><dd class="font-mono">AUT003</dd><dt>Reference ID</dt><dd class="font-mono break-all">req-123</dd>`)
	assert.Contains(t, out, "If it keeps happening, contact the app&#39;s support and share the reference ID.")
	assert.Contains(t, out, `<a class="btn btn-primary btn-block" href="/id/p/acme/login">Back to sign in</a>`)
	assert.NotContains(t, out, "console")

	v.SupportEmail, v.Title, v.Code, v.Reference = "help@acme.test", "", "", ""
	help := html(t, ctxIn(t, i18n.Indonesian), Error(page(), v))
	assert.Contains(t, help, "<title>Terjadi kesalahan · Acme Store</title>")
	assert.Contains(t, help, `Butuh bantuan? Kirim email ke <a class="link break-all" href="mailto:help@acme.test">help@acme.test</a>.`, "no code or reference: no reference ID to share")
	assert.NotContains(t, help, "<dl")
	assert.Contains(t, help, "Kembali ke halaman masuk")
}

func TestError_UnknownProject(t *testing.T) {
	out := html(t, ctxIn(t, i18n.English), Error(layout.LayoutData{BasePath: "/id"}, ErrorData{Title: "ignored", Message: "ignored"}))
	assert.Contains(t, out, "<title>We couldn&#39;t find this sign-in page · Othra</title>")
	assert.Contains(t, out, "Check the link, or start again from your app.")
	assert.NotContains(t, out, "ignored")
	assert.NotContains(t, out, "data-brand")
	assert.NotContains(t, out, "/login")
	assert.NotContains(t, out, "console")
}

func TestSplitAround(t *testing.T) {
	assert.Equal(t, []string{"Email ", "{x}", " now"}, splitAround("Email {x} now", "{x}"))
	assert.Equal(t, []string{"no slot"}, splitAround("no slot", "{x}"))
}

func TestPicker_NoticeIsTranslatedOnce(t *testing.T) {
	for locale, want := range map[string]string{
		i18n.English:    `<span class="break-all">bob@example.com</span> is signed out on this browser.</span></div>`,
		i18n.Indonesian: `<span class="break-all">bob@example.com</span> sudah keluar di browser ini.</span></div>`,
	} {
		out := html(t, ctxIn(t, locale), Picker(page(), PickerData{Site: site, Notice: "bob@example.com"}))
		assert.Contains(t, out, want, locale)
		assert.Equal(t, 1, strings.Count(out, "bob@example.com"), "%s: the email appears once", locale)
	}
}

func TestAlerts_CarryAnIconAndNeutralText(t *testing.T) {
	en := ctxIn(t, i18n.English)
	for name, c := range map[string]templ.Component{
		"picker error": Picker(page(), PickerData{Site: site, Error: "That account isn't signed in."}),
		"terms error":  Terms(page(), TermsData{Site: site, Error: "Tick the box."}),
		"resent":       Sent(page(), SentData{Site: site, Resent: true}),
		"notice":       Login(page(), LoginData{Site: site, Notice: "a@b.test"}),
	} {
		out := html(t, en, c)
		i := strings.Index(out, `class="alert `)
		require.GreaterOrEqual(t, i, 0, name)
		alert := out[i:]
		alert = alert[:strings.Index(alert, "</div>")]
		assert.Contains(t, alert, `aria-hidden="true"`, "%s: the status icon is decorative", name)
		assert.NotContains(t, alert, "text-error\">", name)
	}
}

func TestTerms_HidesTheFooterLinks(t *testing.T) {
	d := page()
	d.Brand.TermsURL, d.Brand.PrivacyURL = "https://acme.test/terms", "https://acme.test/privacy"
	out := html(t, ctxIn(t, i18n.English), Terms(d, TermsData{Site: site, TermsURL: "https://acme.test/terms", PrivacyURL: "https://acme.test/privacy"}))
	assert.NotContains(t, out, "<nav", "the consent line already links both")
	assert.Equal(t, 1, strings.Count(out, `href="https://acme.test/terms"`))
	assert.Equal(t, "https://acme.test/terms", d.Brand.TermsURL, "the caller's brand is not changed")
	assert.Contains(t, html(t, ctxIn(t, i18n.English), Picker(d, PickerData{Site: site})), "<nav", "other pages keep the footer")
	login := html(t, ctxIn(t, i18n.English), Login(d, LoginData{Site: site}))
	assert.NotContains(t, login, "<nav", "sign-in names both in its own sentence")
	assert.Contains(t, login, "By continuing you accept Acme Store&#39;s <a class=\"link\" href=\"https://acme.test/terms\"")
	assert.Contains(t, login, "Terms of Service<span class=\"sr-only\"> (opens in a new tab)</span></a> and <a class=\"link\" href=\"https://acme.test/privacy\"")
}

func TestContinue(t *testing.T) {
	target := "https://app.example/oauth/callback?code=abc&state=s1&iss=https%3A%2F%2Fid.example%2Fp%2Facme"
	out := html(t, ctxIn(t, i18n.English), Continue(page(), ContinueData{Site: site, Client: "Openwa Web", Target: target}))
	assert.Contains(t, out, `<meta http-equiv="refresh" content="0;url=https://app.example/oauth/callback?code=abc&amp;state=s1&amp;iss=https%3A%2F%2Fid.example%2Fp%2Facme">`)
	assert.Contains(t, out, "<title>Continuing to Openwa Web · Acme Store</title>")
	assert.Contains(t, out, `<p role="status" class="text-sm text-base-content/70">If nothing happens, use the button below.</p>`)
	assert.Contains(t, out, `<a class="btn btn-primary btn-block" href="https://app.example/oauth/callback?code=abc&amp;state=s1&amp;iss=https%3A%2F%2Fid.example%2Fp%2Facme">Continue to Openwa Web</a>`)
	assert.Contains(t, out, `loading loading-spinner loading-sm`)
	assert.NotContains(t, out, "<script")

	id := html(t, ctxIn(t, i18n.Indonesian), Continue(page(), ContinueData{Site: site, Target: "/id/p/acme/oauth2/authorize?client_id=x"}))
	assert.Contains(t, id, "Melanjutkan ke Acme Store", "no client name falls back to the project")
	assert.Contains(t, id, ">Lanjutkan ke Acme Store</a>")
}

func TestEndSession(t *testing.T) {
	v := EndSessionData{Site: site, Client: "Openwa Web", Token: "tok-end", Hint: "eyJ.hint", PostLogout: "https://app.example/", State: "s1"}
	out := html(t, ctxIn(t, i18n.English), EndSession(page(), v))
	assert.Contains(t, out, "<title>Sign out of Acme Store? · Acme Store</title>")
	assert.Contains(t, out, "Openwa Web asked to sign you out of Acme Store.")
	form := strings.ReplaceAll(out[strings.Index(out, `<form method="post" action="/id/p/acme/oauth2/end-session">`):], "> <", "><")
	assert.Contains(t, form, `<input type="hidden" name="csrf" value="tok-end"><input type="hidden" name="confirm" value="1"><input type="hidden" name="id_token_hint" value="eyJ.hint"><input type="hidden" name="post_logout_redirect_uri" value="https://app.example/"><input type="hidden" name="state" value="s1"><button class="btn btn-primary btn-block" type="submit">Sign out</button>`)
	assert.Contains(t, out, "If you didn&#39;t ask for this, close this page. You stay signed in.")

	bare := html(t, ctxIn(t, i18n.Indonesian), EndSession(page(), EndSessionData{Site: site, Token: "t", Hint: "h"}))
	assert.Contains(t, bare, "Keluar dari Acme Store?")
	assert.Contains(t, bare, "Acme Store meminta agar Anda keluar dari Acme Store.", "no client name falls back to the project")
	assert.NotContains(t, bare, "post_logout_redirect_uri")
	assert.NotContains(t, bare, `name="state"`)
}

func TestError_NoSignIn(t *testing.T) {
	out := html(t, ctxIn(t, i18n.English), Error(page(), ErrorData{Site: site, Title: "Can't sign in to this app", Message: "x", Code: "SIG006", NoSignIn: true}))
	assert.NotContains(t, out, "Back to sign in")
	assert.NotContains(t, out, "/login")
	assert.Contains(t, out, "If it keeps happening")
}

func TestTermsDeclined(t *testing.T) {
	ctx := ctxIn(t, i18n.English)
	v := TermsDeclined(ctx, site)
	assert.Equal(t, ErrorData{Site: site, Title: "You didn't accept the terms", Message: "You need to accept Acme Store's terms to use your account. You can come back and accept them any time."}, v)
	out := html(t, ctx, Error(page(), v))
	assert.Contains(t, out, "Back to sign in")
	assert.NotContains(t, out, "<dl")
	assert.Equal(t, "Anda tidak menyetujui ketentuan", TermsDeclined(ctxIn(t, i18n.Indonesian), site).Title)
}

func TestTerms_Decline(t *testing.T) {
	out := html(t, ctxIn(t, i18n.English), Terms(page(), TermsData{Site: site, TermsURL: "https://acme.test/terms"}))
	assert.Contains(t, out, `<button class="btn btn-ghost btn-block" type="submit" name="decline" value="1" formnovalidate>Decline</button>`)
	assert.Less(t, strings.Index(out, ">Create account</button>"), strings.Index(out, ">Decline</button>"), "Decline sits under the main action, in the same form")
	assert.Less(t, strings.Index(out, ">Decline</button>"), strings.Index(out, "</form>"))
}

func TestLoginAndPicker_ClientContext(t *testing.T) {
	en := ctxIn(t, i18n.English)
	plain := html(t, en, Login(page(), LoginData{Site: site}))
	assert.Contains(t, plain, ">Sign in to Acme Store</h1>")

	withClient := html(t, en, Login(page(), LoginData{Site: site, Client: "Openwa Web", Email: "ana@acme.test"}))
	assert.Contains(t, withClient, "Continue to Openwa Web<")
	assert.Contains(t, withClient, `value="ana@acme.test"`, "login_hint prefills the email")
	assert.NotContains(t, withClient, "asks you to sign in again")

	reauth := html(t, ctxIn(t, i18n.Indonesian), Login(page(), LoginData{Site: site, Client: "Openwa Web", Reauth: true}))
	assert.Contains(t, reauth, `<p class="text-sm text-base-content/70">Openwa Web meminta Anda masuk lagi.</p>`)
	assert.Contains(t, reauth, "Lanjutkan ke Openwa Web<")
	portal := html(t, en, Login(page(), LoginData{Site: site, Reauth: true}))
	assert.Contains(t, portal, "Sign in again to continue.", "the portal's step-up")
	assert.NotContains(t, portal, "asks you to sign in again")
	assert.Contains(t, portal, "<title>Confirm it&#39;s you", "the step-up heading matches ACC009")
	assert.Contains(t, portal, ">Confirm it&#39;s you</h", "the step-up page is titled as a confirmation")
	assert.NotContains(t, portal, "Welcome")
	assert.Contains(t, html(t, ctxIn(t, i18n.Indonesian), Login(page(), LoginData{Site: site, Reauth: true})), "Konfirmasi bahwa ini Anda")
	assert.Contains(t, reauth, ">Masuk ke Acme Store</h1>", "an app's prompt=login keeps the sign-in heading")
	assert.Contains(t, withClient, ">Sign in to Acme Store</h1>", "normal sign-in keeps its heading")

	assert.Contains(t, html(t, en, Picker(page(), PickerData{Site: site})), "to continue to Acme Store")
	assert.Contains(t, html(t, en, Picker(page(), PickerData{Site: site, Client: "Openwa Web"})), "to continue to Openwa Web")
}

func TestLogin_ReauthHasNoWayBackToThePicker(t *testing.T) {
	out := html(t, ctxIn(t, i18n.English), Login(page(), LoginData{Site: site, Client: "Openwa Web", Reauth: true, FromPicker: true}))
	assert.NotContains(t, out, "Back to accounts", "the picker would send the user straight back here")
	assert.Contains(t, out, "Openwa Web asks you to sign in again.")
}

func TestError_HelpLineOnlyWithSomethingToShare(t *testing.T) {
	en := ctxIn(t, i18n.English)
	declined := html(t, en, Error(page(), TermsDeclined(en, site)))
	assert.NotContains(t, declined, "reference ID", "nothing to share")
	assert.Contains(t, declined, "Back to sign in")

	support := TermsDeclined(en, site)
	support.SupportEmail = "help@acme.test"
	plain := html(t, en, Error(page(), support))
	assert.Contains(t, plain, `Need help? Email <a class="link break-all" href="mailto:help@acme.test">help@acme.test</a>.`, "the contact stays, without the reference ID")
	assert.NotContains(t, plain, "reference ID")

	for name, v := range map[string]ErrorData{
		"code":      {Site: site, Message: "x", Code: "SIG006"},
		"reference": {Site: site, Message: "x", Reference: "req-1"},
	} {
		assert.Contains(t, html(t, en, Error(page(), v)), "share the reference ID", name)
	}
}

func consentData() ConsentData {
	return ConsentData{
		Site:         site,
		Client:       "Claude Code",
		ClientHost:   "claude.ai",
		RS:           "Openwa MCP",
		RSIdentifier: "https://openwa.example.com/mcp",
		Email:        "ana@acme.test",
		UserSession:  "us1",
		ReturnTo:     "/id/p/acme/oauth2/authorize?client_id=x",
		ConsentToken: "tok-consent",
		Scopes: []ConsentScope{
			{Scope: "openid", AllowedBefore: true},
			{Scope: "messages:read", Description: "Read your messages"},
			{Scope: "offline_access"},
			{Scope: "devices:write"},
		},
		RedirectHost: "localhost",
		Loopback:     true,
	}
}

func TestConsent_FirstTime(t *testing.T) {
	out := strings.ReplaceAll(html(t, ctxIn(t, i18n.English), Consent(page(), consentData())), "> <", "><")
	assert.Contains(t, out, "<title>Claude Code wants to access Openwa MCP · Acme Store</title>")
	assert.NotContains(t, out, `<span class="font-medium">Openwa MCP</span>`, "the resource sits in the title and Technical details only")
	assert.NotContains(t, out, "<img")

	assert.Regexp(t, `<span class="badge badge-sm"><svg[^>]*>.*?</svg> MCP client</span><span class="badge badge-sm badge-soft badge-warning"><svg[^>]*>.*?</svg> Unverified app</span>`, out, "a metadata-document client is labelled MCP and unverified")
	assert.Regexp(t, `<div role="note" class="alert items-start text-ui"><svg[^>]*>.*?</svg><span class="min-w-0">That&#39;s a program on this device. Continue only if you started this sign-in. Acme Store hasn&#39;t reviewed this app. Its details come from <span class="font-mono font-medium text-warning wrap-anywhere">claude.ai</span>.</span></div>`, out, "a loopback MCP client gets one calm note naming its metadata host")

	assert.Contains(t, out, `<span class="min-w-0 flex-1">Read your messages</span>`)
	assert.Contains(t, out, `<span class="min-w-0 flex-1">Stay connected when you&#39;re away</span>`, "offline_access is its own line")
	assert.Contains(t, out, `<span class="min-w-0 flex-1">devices:write</span>`, "no description: the raw scope")
	assert.Regexp(t, `<li class="flex items-start gap-2"><svg[^>]*><circle cx="12" cy="8" r="5"></circle>.*?Know who you are`, out, "identity scopes carry the person icon")
	assert.Regexp(t, `<li class="flex items-start gap-2"><svg[^>]*><path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"></path>.*?Stay connected`, out, "offline_access carries the refresh icon")
	details := out[strings.Index(out, `<details class="text-xs text-base-content/70">`):]
	assert.Contains(t, details, `>Technical details</summary>`, "raw scope ids sit behind Technical details")
	assert.Contains(t, details, `<dt>scope</dt><dd class="break-all">messages:read offline_access devices:write openid</dd><dt>resource</dt><dd class="break-all">https://openwa.example.com/mcp</dd>`)
	assert.NotContains(t, details, "<dt>issuer</dt>", "no issuer row when the brand has none")
	assert.Contains(t, out, `<span class="badge badge-ghost badge-sm">Allowed before</span>`)
	order := []int{strings.Index(out, "Read your messages"), strings.Index(out, ">Stay connected"), strings.Index(out, ">devices:write<"), strings.Index(out, ">Know who you are<")}
	assert.IsIncreasing(t, order, "new scopes first, in request order; allowed-before last")

	assert.Contains(t, out, `<span class="font-mono text-base font-medium break-all">localhost</span>`, "the return host sits under the title, in mono")
	assert.Less(t, strings.Index(out, "break-all\">localhost<"), strings.Index(out, "Signed in as"), "the redirect host sits above the account and the scopes")
	assert.Contains(t, out, `<p class="text-ui break-words text-base-content/70">Lasts until you revoke it in your Acme Store account.</p>`, "the lifetime sits above the buttons")
	assert.NotContains(t, out, "you&#39;ll go back to", "the return host is already under the title")

	form := out[strings.Index(out, `<form method="post" action="/id/p/acme/consent"`):]
	assert.Contains(t, form, `<input type="hidden" name="csrf" value="tok-consent"><input type="hidden" name="user_session" value="us1"><input type="hidden" name="return_to" value="/id/p/acme/oauth2/authorize?client_id=x">`)
	assert.Contains(t, form, `<button class="btn flex-1" type="submit" name="deny" value="1" formnovalidate>Cancel</button><button class="btn btn-primary flex-1" type="submit">Allow access</button>`)
	assert.Contains(t, out, `href="/id/p/acme/login?add=1&amp;return_to=%2Fid%2Fp%2Facme%2Foauth2%2Fauthorize%3Fclient_id%3Dx">Switch</a>`)
	assert.Contains(t, out, `Signed in as <span class="font-medium break-all" title="ana@acme.test">ana@acme.test</span>`)
}

func TestConsent_RegisteredAccountTitleNoLoopback(t *testing.T) {
	v := consentData()
	v.ClientHost, v.RS, v.RSIdentifier, v.Loopback, v.RedirectHost = "", "", "", false, "app.example"
	out := html(t, ctxIn(t, i18n.Indonesian), Consent(page(), v))
	assert.Contains(t, out, "Claude Code ingin mengakses akun Acme Store Anda")
	assert.NotContains(t, out, "alert-warning", "no CIMD note for a registered client")
	assert.NotContains(t, out, "perangkat ini", "no loopback warning")
	assert.Contains(t, out, "Berlaku sampai Anda mencabutnya di akun Acme Store Anda.")
	assert.Contains(t, out, ">Detail teknis</summary>")
	assert.Contains(t, out, "Mengetahui siapa Anda")
	assert.Contains(t, out, ">Izinkan</button>")
}

func TestConsent_Remembered(t *testing.T) {
	v := consentData()
	v.Remembered = true
	out := html(t, ctxIn(t, i18n.English), Consent(page(), v))
	assert.Contains(t, out, "<title>Continue to Claude Code? · Acme Store</title>")
	assert.Contains(t, out, "You allowed Claude Code to access Openwa MCP before.")
	assert.Contains(t, out, `<span class="badge badge-sm badge-soft badge-warning">`, "the remembered page still marks an unverified app")
	assert.NotContains(t, out, "alert-warning", "the remembered page has one muted line, not the box")
	assert.NotContains(t, out, "messages:read", "no scope list")
	assert.Contains(t, out, "You&#39;ll go back to <span class=\"font-medium text-base-content\">localhost</span>.")
	assert.NotContains(t, out, "Technical details", "the one-click page has no scope list to explain")
	assert.Contains(t, out, "That&#39;s a program on this device.")
	assert.Contains(t, out, `<button class="btn btn-primary flex-1" type="submit">Continue to Claude Code</button>`)
	assert.Contains(t, out, `name="deny" value="1" formnovalidate>Cancel</button>`)
	assert.Contains(t, out, `name="csrf" value="tok-consent"`, "a real POST, never a link")

	v.ClientHost = ""
	assert.NotContains(t, html(t, ctxIn(t, i18n.English), Consent(page(), v)), "unverified app")
}

func TestConsent_EscapesTheAppsName(t *testing.T) {
	v := consentData()
	v.Client = `<script>x</script>`
	v.Scopes = []ConsentScope{{Scope: "a", Description: `<img src=x>`}}
	out := html(t, ctxIn(t, i18n.English), Consent(page(), v))
	assert.NotContains(t, out, "<script>")
	assert.NotContains(t, out, "<img")
}

func TestAuthorizeError(t *testing.T) {
	en := ctxIn(t, i18n.English)
	v := AuthorizeError(en, site, apperror.CodeAuthorizeRedirectNotRegistered, "req-9")
	assert.Equal(t, ErrorData{
		Site: site, Title: "Can't sign in to this app", Message: "This app asked to return to an address that isn't registered for it.",
		Code: "SIG006", Reference: "req-9", NoSignIn: true,
	}, v)
	out := html(t, en, Error(page(), v))
	assert.NotContains(t, out, "Back to sign in")
	assert.Contains(t, out, "SIG006")
}

func TestTerms_UsesTheSharedAccountTile(t *testing.T) {
	out := html(t, ctxIn(t, i18n.English), Terms(page(), TermsData{Site: site, Email: "ana@acme.test", TermsURL: "https://acme.test/t"}))
	assert.Contains(t, out, `<span class="block truncate font-medium" title="ana@acme.test">ana@acme.test</span>`)
	assert.Contains(t, out, ">Use another account</a>")
}

func TestLoginAndPicker_CIMDClientShowsItsHost(t *testing.T) {
	en := ctxIn(t, i18n.English)
	line := `<p class="text-xs text-base-content/70">From claude.ai · unverified app</p>`
	login := html(t, en, Login(page(), LoginData{Site: site, Client: "Claude Code", ClientHost: "claude.ai"}))
	assert.Contains(t, login, "Continue to Claude Code<")
	assert.Contains(t, login, line, "a self-asserted name always comes with its host")
	assert.Less(t, strings.Index(login, "Claude Code."), strings.Index(login, line))

	picker := html(t, en, Picker(page(), PickerData{Site: site, Client: "Claude Code", ClientHost: "claude.ai"}))
	assert.Contains(t, picker, line)

	id := html(t, ctxIn(t, i18n.Indonesian), Login(page(), LoginData{Site: site, Client: "Claude Code", ClientHost: "claude.ai"}))
	assert.Contains(t, id, "Dari claude.ai · aplikasi belum terverifikasi")

	for name, out := range map[string]string{
		"registered login":  html(t, en, Login(page(), LoginData{Site: site, Client: "Openwa Web"})),
		"registered picker": html(t, en, Picker(page(), PickerData{Site: site, Client: "Openwa Web"})),
		"no client":         html(t, en, Login(page(), LoginData{Site: site, ClientHost: "claude.ai"})),
	} {
		assert.NotContains(t, out, "unverified app", name)
	}
}

func TestConsent_AppMarkPointsAtTheProject(t *testing.T) {
	en := ctxIn(t, i18n.English)
	for name, v := range map[string]ConsentData{
		"first":      consentData(),
		"remembered": func() ConsentData { v := consentData(); v.Remembered = true; return v }(),
		"account":    func() ConsentData { v := consentData(); v.RS, v.RSIdentifier = "", ""; return v }(),
	} {
		out := strings.ReplaceAll(html(t, en, Consent(page(), v)), "> <", "><")
		assert.Regexp(t, `<div class="flex items-center gap-2"><span class="[^"]*rounded-field[^"]*" aria-hidden="true"><svg[^>]*><path d="M12 22v-5"></path>.*?</svg></span><svg[^>]*>.*?</svg><span class="[^"]*" role="img" aria-label="Acme Store" data-testid="brand-tile">A</span></div>`, out, "%s: the MCP client's plug points at the project tile", name)
	}
	registered := consentData()
	registered.ClientHost, registered.Loopback = "", false
	out := strings.ReplaceAll(html(t, en, Consent(page(), registered)), "> <", "><")
	assert.Regexp(t, `aria-hidden="true"><svg[^>]*><rect x="2" y="4" width="20" height="16" rx="2"></rect>`, out, "a registered app gets the window icon")
	assert.NotContains(t, out, "MCP client")
	assert.NotContains(t, out, `role="note"`)
}

func TestConsent_TechnicalDetailsNameTheIssuer(t *testing.T) {
	d := page()
	d.Brand.Issuer = "https://othra.example/p/acme"
	out := strings.ReplaceAll(html(t, ctxIn(t, i18n.English), Consent(d, consentData())), "> <", "><")
	assert.Contains(t, out, `<dt>issuer</dt><dd class="break-all">https://othra.example/p/acme</dd></dl>`)
	assert.Contains(t, out, `<footer class="flex w-full flex-col gap-2 text-xs text-base-content/70 max-w-112">`, "the footer lines up with the wider consent column")
}

func TestLogin_Google(t *testing.T) {
	both := html(t, ctxIn(t, i18n.English), Login(page(), LoginData{Site: site, ReturnTo: "/id/p/acme/account/", LoginToken: "tok-login", Google: true, GoogleToken: "tok-google"}))
	assert.Contains(t, both, `<form method="post" action="/id/p/acme/federation/google/start"><input type="hidden" name="csrf" value="tok-google"><input type="hidden" name="return_to" value="/id/p/acme/account/">`)
	assert.Contains(t, both, `<button class="btn w-full gap-2.5 font-['Google_Sans_Flex',Roboto,Arial,sans-serif] text-sm/5 font-medium [--btn-color:#f2f2f2] [--btn-fg:#1f1f1f]" type="submit" data-google><svg`, "Google's neutral button: its own fill, text colour and font")
	assert.Contains(t, both, `<span>Continue with Google</span></button>`)
	assert.Contains(t, both, `<div class="divider my-0 text-xs">or</div>`)
	assert.Less(t, strings.Index(both, `name="email"`), strings.Index(both, "data-google"), "the email form first, then Google under an or divider")

	only := html(t, ctxIn(t, i18n.Indonesian), Login(page(), LoginData{Site: site, Google: true, EmailOff: true}))
	assert.Contains(t, only, "Lanjutkan dengan Google", "Google's own Indonesian wording")
	assert.NotContains(t, only, `name="email"`)
	assert.NotContains(t, only, "atau lanjut dengan email")

	plain := html(t, ctxIn(t, i18n.English), Login(page(), LoginData{Site: site, EmailOff: true}))
	assert.Contains(t, plain, `name="email"`, "no Google, so the email form stays")
	assert.NotContains(t, plain, "data-google")
}

func TestCollision(t *testing.T) {
	out := html(t, ctxIn(t, i18n.English), Collision(page(), CollisionData{Site: site, Email: "ana@acme.test", Minutes: 15, ReturnTo: "/id/p/acme/account/"}))
	assert.Contains(t, out, "<title>This email already has an account · Acme Store</title>")
	assert.Contains(t, out, `<span class="break-all">ana@acme.test</span> already signs in to Acme Store with an email link.`)
	assert.Contains(t, out, "The link works once, for 15 minutes. Until then, nothing changes.")
	assert.Contains(t, out, `href="/id/p/acme/login?add=1&amp;return_to=%2Fid%2Fp%2Facme%2Faccount%2F">Use a different account</a>`)
	assert.NotContains(t, out, "<form", "no resend: a plain link would drop the pending Google identity")

	id := html(t, ctxIn(t, i18n.Indonesian), Collision(page(), CollisionData{Site: site, Minutes: 15}))
	assert.Contains(t, id, "Email akun Google Anda sudah dipakai untuk masuk ke Acme Store")
	assert.NotContains(t, id, "break-all\">")
}

func TestVerify_LinkMode(t *testing.T) {
	out := html(t, ctxIn(t, i18n.English), Verify(page(), VerifyData{Site: site, Email: "ana@acme.test", VerifyToken: "tok-verify", LinkEmail: "ana.s@gmail.com"}))
	assert.Contains(t, out, "<title>Add Google to your account? · Acme Store</title>")
	assert.Contains(t, out, `You&#39;re signing in to Acme Store as ana@acme.test. After this, you can also sign in with Google as <span class="break-all font-medium">ana.s@gmail.com</span>.`)
	assert.Contains(t, out, `<button class="btn btn-primary btn-block" type="submit" autofocus>Add Google and continue</button>`)
	assert.Contains(t, out, `<button class="btn btn-ghost btn-block" type="submit" name="skip" value="1">Continue without Google</button>`)
	assert.Contains(t, out, "Only continue if you asked for this link.")

	plain := html(t, ctxIn(t, i18n.English), Verify(page(), VerifyData{Site: site, Email: "ana@acme.test"}))
	assert.NotContains(t, plain, "Google")
	assert.NotContains(t, plain, `name="skip"`)
}

func TestPicker_SignInMethod(t *testing.T) {
	out := html(t, ctxIn(t, i18n.English), Picker(page(), PickerData{Site: site, Accounts: []AccountTile{{SessionID: "s1", Email: "ana@acme.test", Google: true}, {SessionID: "s2", Email: "budi@example.com"}}}))
	assert.Less(t, strings.Index(out, "ana@acme.test"), strings.Index(out, "Signed in with Google"))
	assert.Less(t, strings.Index(out, "Signed in with Google"), strings.Index(out, "budi@example.com"))
	assert.Less(t, strings.Index(out, "budi@example.com"), strings.Index(out, "Signed in with an email link"))
}

func TestUpstreamError_KeepsReturnTo(t *testing.T) {
	ctx := ctxIn(t, i18n.English)
	v := UpstreamError(ctx, site, apperror.CodeUnexpected, "ref-1", "/id/p/acme/account/")
	out := html(t, ctx, Error(page(), v))
	assert.Contains(t, out, "<title>Couldn&#39;t sign in with Google · Acme Store</title>")
	assert.Contains(t, out, `href="/id/p/acme/login?return_to=%2Fid%2Fp%2Facme%2Faccount%2F">Back to sign in</a>`)

	bare := html(t, ctx, Error(page(), ErrorData{Site: site, Message: "x"}))
	assert.Contains(t, bare, `href="/id/p/acme/login">Back to sign in</a>`)
}

func TestContinue_ToGoogle(t *testing.T) {
	out := html(t, ctxIn(t, i18n.English), Continue(page(), ContinueData{Site: site, Client: "Google", Target: "https://accounts.google.com/o/oauth2/v2/auth?client_id=c&state=s"}))
	assert.Contains(t, out, `<meta http-equiv="refresh" content="0;url=https://accounts.google.com/o/oauth2/v2/auth?client_id=c&amp;state=s">`)
	assert.Contains(t, out, "Continuing to Google")
	assert.Contains(t, out, `href="https://accounts.google.com/o/oauth2/v2/auth?client_id=c&amp;state=s">Continue to Google</a>`)
}

// the collision link's email says it adds Google, in both locales.
func TestMail_AddGoogle(t *testing.T) {
	m := MailData{Project: "Acme Store", Link: "https://x.test/v", Minutes: 15, GoogleEmail: "ana@gmail.com"}
	en := ctxIn(t, i18n.English)
	assert.Equal(t, "Add Google to your Acme Store account", MailSubject(en, m))
	assert.Equal(t, "Someone tried to sign in to Acme Store with Google as ana@gmail.com. This email already has an account.\n\n"+
		"Open this link to add Google to your account. It works once, for 15 minutes:\n"+
		"https://x.test/v\n\n"+
		"If this wasn't you, ignore this email. Nothing changes without the link.\n\n"+
		"Sent by Othra for Acme Store.\n", MailText(en, m))
	out := html(t, en, MailHTML(m))
	assert.Contains(t, out, ">Add Google</a>")
	assert.Contains(t, out, "with Google as ana@gmail.com")

	id := ctxIn(t, i18n.Indonesian)
	assert.Equal(t, "Tambahkan Google ke akun Acme Store Anda", MailSubject(id, m))
	assert.Contains(t, MailText(id, m), "dengan Google sebagai ana@gmail.com")
	assert.Contains(t, html(t, id, MailHTML(m)), ">Tambahkan Google</a>")
}

func TestUpstreamError_NamesTheProject(t *testing.T) {
	for _, code := range []apperror.Code{apperror.CodeSignInUpstreamDisabled, apperror.CodeSignInUpstreamLinkedElsewhere, apperror.CodeSignInUpstreamEmailTaken} {
		for _, locale := range []string{i18n.English, i18n.Indonesian} {
			ctx := ctxIn(t, locale)
			out := html(t, ctx, Error(page(), UpstreamError(ctx, site, code, "", "")))
			assert.Contains(t, out, "Acme Store", "the project name %s %s", code, locale)
			assert.NotContains(t, out, "{project}", "%s %s", code, locale)
			assert.NotContains(t, out, "this project", "%s %s", code, locale)
			assert.NotContains(t, out, "proyek ini", "%s %s", code, locale)
		}
	}
}

func TestVerify_LinkModeIndonesian(t *testing.T) {
	out := html(t, ctxIn(t, i18n.Indonesian), Verify(page(), VerifyData{Site: site, Email: "ana@acme.test", LinkEmail: "ana.s@gmail.com"}))
	assert.Contains(t, out, "Tambahkan Google dan lanjutkan")
	assert.Contains(t, out, "Lanjutkan tanpa Google")
	login := html(t, ctxIn(t, i18n.Indonesian), Login(page(), LoginData{Site: site, Google: true}))
	assert.Contains(t, login, `<div class="divider my-0 text-xs">atau</div>`)
}

// a failed Connect Google is titled as a connect and leads back to the account, never to sign in.
func TestLinkUpstreamError(t *testing.T) {
	ctx := ctxIn(t, i18n.English)
	out := html(t, ctx, Error(page(), LinkUpstreamError(ctx, site, apperror.CodeSignInUpstreamLinkedElsewhere, "ref-1")))
	assert.Contains(t, out, "<title>Couldn&#39;t connect Google · Acme Store</title>")
	assert.Contains(t, out, `href="/id/p/acme/account/">Back to your account</a>`)
	assert.NotContains(t, out, "Back to sign in")

	id := ctxIn(t, i18n.Indonesian)
	out = html(t, id, Error(page(), LinkUpstreamError(id, site, apperror.CodeSignInUpstreamCancelled, "")))
	assert.Contains(t, out, "Tidak bisa menghubungkan Google")
	assert.Contains(t, out, "Kembali ke akun Anda")
	assert.Contains(t, out, "Menghubungkan Google dibatalkan. Tidak ada yang berubah.", "the connect sentence")
	assert.NotContains(t, out, "cara masuk lain")

	for code, want := range map[apperror.Code]string{
		apperror.CodeSignInUpstreamCancelled:       "Connecting Google was cancelled. Nothing changed.",
		apperror.CodeSignInUpstreamResponseInvalid: "We couldn&#39;t verify the answer from Google. Nothing changed; try connecting again.",
		apperror.CodeSignInUpstreamLinkedElsewhere: "This Google account already signs in to a different Acme Store account.",
	} {
		got := html(t, ctx, Error(page(), LinkUpstreamError(ctx, site, code, "")))
		assert.Contains(t, got, want, code)
		assert.NotContains(t, got, "sign in.", "%s: no sign-in sentence on a connect", code)
	}
}

// the Google sign-up link's email and page, in both locales.
func TestMailAndPage_GoogleSignUp(t *testing.T) {
	m := MailData{Project: "Acme Store", Link: "https://x.test/v", Minutes: 15, GoogleEmail: "budi@acme.test", GoogleSignUp: true}
	en, id := ctxIn(t, i18n.English), ctxIn(t, i18n.Indonesian)
	assert.Equal(t, "Finish signing up to Acme Store with Google", MailSubject(en, m))
	assert.Contains(t, MailText(en, m), "creating your Acme Store account with Google as budi@acme.test")
	assert.Contains(t, html(t, en, MailHTML(m)), ">Finish signing up</a>")
	assert.Equal(t, "Selesaikan pendaftaran di Acme Store dengan Google", MailSubject(id, m))

	v := CollisionData{Site: site, Email: "budi@acme.test", Minutes: 15, SignUp: true}
	out := html(t, en, Collision(page(), v))
	assert.Contains(t, out, "Check your inbox to finish signing up with Google")
	assert.NotContains(t, out, "already has an account")
	assert.Contains(t, html(t, id, Collision(page(), v)), "Cek kotak masuk")
}

// a sign-up link's confirm page says it finishes the Google sign-up, in both locales.
func TestVerify_GoogleSignUp(t *testing.T) {
	v := VerifyData{Site: site, Email: "budi@acme.test", VerifyToken: "tok", SignUpEmail: "budi@acme.test"}
	out := html(t, ctxIn(t, i18n.English), Verify(page(), v))
	assert.Contains(t, out, "Finish signing up to Acme Store with Google")
	assert.Contains(t, out, "with Google as")
	assert.Contains(t, out, "budi@acme.test")
	assert.Contains(t, out, ">Finish signing up</button>")
	assert.NotContains(t, out, "Sign in to Acme Store")
	assert.NotContains(t, out, `name="skip"`)
	assert.Contains(t, html(t, ctxIn(t, i18n.Indonesian), Verify(page(), v)), "Selesaikan pendaftaran di Acme Store dengan Google")
}

func TestContinue_ConsentOutcomes(t *testing.T) {
	target := "https://app.example/cb?code=abc"
	allowed := strings.ReplaceAll(html(t, ctxIn(t, i18n.English), Continue(page(), ContinueData{Site: site, Client: "Openwa Web", Target: target, Outcome: OutcomeAllowed})), "> <", "><")
	assert.Contains(t, allowed, "<title>Access allowed · Acme Store</title>")
	assert.Contains(t, allowed, `<meta http-equiv="refresh" content="0;url=https://app.example/cb?code=abc">`)
	assert.Regexp(t, `<div class="flex w-full max-w-112 flex-col gap-3"><svg[^>]*class="shrink-0 size-6"[^>]*><circle cx="12" cy="12" r="10"></circle><path d="m16 9-5.5 5.5L8 12"></path></svg><h1 class="text-2xl font-semibold">Access allowed</h1>`, allowed)
	assert.Contains(t, allowed, "<p>Returning you to Openwa Web. Revoke access at any time under Connected apps in your Acme Store account.</p>")
	assert.Contains(t, allowed, `<a class="btn" href="https://app.example/cb?code=abc">Continue to Openwa Web</a>`, "a visible way on when the refresh is blocked")
	assert.NotContains(t, allowed, "<script", "a forwarding page runs no script")
	assert.Contains(t, allowed, `<footer class="flex w-full flex-col gap-2 text-xs text-base-content/70 max-w-112">`, "the consent column's width, footer included")

	denied := html(t, ctxIn(t, i18n.Indonesian), Continue(page(), ContinueData{Site: site, Client: "Openwa Web", Target: target, Outcome: OutcomeDenied}))
	assert.Contains(t, denied, `<h1 class="text-2xl font-semibold">Tidak ada yang dibagikan</h1>`)
	assert.Contains(t, denied, "Openwa Web tidak mendapat akses.")
	assert.Contains(t, denied, `<path d="m15 9-6 6"></path>`, "the circle-x icon")
}
