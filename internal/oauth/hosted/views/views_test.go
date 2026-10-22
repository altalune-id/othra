package views

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/internal/web/i18n"
	"altalune.id/othra/internal/web/layout"
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
	assert.Contains(t, out, "<title>Welcome · Acme Store</title>")
	assert.Contains(t, out, `<form method="post" action="/id/p/acme/login"`)
	assert.Contains(t, out, `<input type="hidden" name="csrf" value="tok-login">`)
	assert.Contains(t, out, `<input type="hidden" name="return_to" value="/id/p/acme/account/">`)
	assert.Contains(t, out, `type="email" name="email" value="" placeholder="you@example.com" autocomplete="email" inputmode="email" spellcheck="false" required autofocus>`)
	assert.Contains(t, out, "Sign in to continue to Acme Store.")
	assert.Contains(t, out, `<button class="btn btn-primary btn-block" type="submit">Continue with email</button>`)
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
		assert.Contains(t, out, want, "B5 %s", locale)
	}
}

func TestLogin_ErrorAndBackToAccounts(t *testing.T) {
	out := html(t, ctxIn(t, i18n.Indonesian), Login(page(), LoginData{Site: site, Email: "nope", Error: "Masukkan alamat email yang valid.", FromPicker: true, ReturnTo: "/id/p/acme/account/"}))
	assert.Contains(t, out, `value="nope"`)
	assert.Contains(t, out, `aria-invalid="true" aria-describedby="email-error"`)
	assert.Contains(t, out, `<p id="email-error" class="flex items-start gap-1.5 text-sm text-base-content">`)
	assert.Contains(t, out, `href="/id/p/acme/login?return_to=%2Fid%2Fp%2Facme%2Faccount%2F">← Kembali ke daftar akun</a>`)
	assert.Contains(t, out, "Lanjut dengan email")
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
	out := html(t, ctxIn(t, i18n.English), Sent(page(), SentData{Site: site, Email: "ana@acme.test", Minutes: 15, ResendToken: "tok-resend", Resent: true}))
	assert.Contains(t, out, `We sent a sign-in link to <span class="break-all">ana@acme.test</span>. It works once, for 15 minutes.`)
	assert.Contains(t, out, `<form method="post" action="/id/p/acme/login/resend">`)
	assert.Contains(t, out, `<input type="hidden" name="csrf" value="tok-resend"><input type="hidden" name="email" value="ana@acme.test">`)
	assert.Contains(t, out, `role="status"`)
	assert.Contains(t, out, `href="/id/p/acme/login?add=1">Use a different email</a>`)
	assert.Contains(t, out, "<p>Only the newest link works. If you asked more than once, use the latest email.</p>", "B6")

	noCookie := html(t, ctxIn(t, i18n.Indonesian), Sent(page(), SentData{Site: site, Minutes: 15}))
	assert.Contains(t, noCookie, "Kami sudah mengirim tautan masuk. Tautan hanya bisa dipakai sekali, selama 15 menit.")
	assert.Contains(t, noCookie, "<p>Hanya tautan terbaru yang berlaku. Jika Anda meminta lebih dari sekali, pakai email yang terakhir.</p>", "B6")
	assert.NotContains(t, noCookie, "/login/resend")
	assert.NotContains(t, noCookie, `role="status"`)
}

func TestVerifyAndLinkUnusable(t *testing.T) {
	out := html(t, ctxIn(t, i18n.English), Verify(page(), VerifyData{Site: site, Email: "ana@acme.test", VerifyToken: "tok-verify"}))
	assert.Contains(t, out, "<title>Confirm sign-in · Acme Store</title>", "B2: the tab does not repeat the project")
	assert.Contains(t, out, ">Sign in to Acme Store</h1>")
	assert.Contains(t, out, `You&#39;re signing in as <span class="break-all">ana@acme.test</span> on this device.`)
	assert.Contains(t, out, `<form method="post" action="/id/p/acme/login/verify"><input type="hidden" name="csrf" value="tok-verify">`)
	assert.Contains(t, out, `type="submit" autofocus>Continue</button>`)
	assert.NotContains(t, out, "token=", "the link token never reaches the page")

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

func TestAccountAndSignedOut(t *testing.T) {
	out := html(t, ctxIn(t, i18n.English), Account(page(), AccountData{Site: site, Email: "ana@acme.test", UserSession: "us1", ReturnTo: "/id/p/acme/account/", SignOutToken: "tok-signout"}))
	assert.Equal(t, 1, strings.Count(out, "ana@acme.test"), "D8: the email shows once, in the tile")
	assert.Contains(t, out, `<span class="min-w-0 break-all font-medium">ana@acme.test</span>`)
	assert.Contains(t, out, `href="/id/p/acme/login?return_to=%2Fid%2Fp%2Facme%2Faccount%2F">Switch account</a>`)
	signOut := strings.ReplaceAll(out[strings.Index(out, `action="/id/p/acme/login/signout"`):], "> <", "><")
	assert.Contains(t, signOut, `<input type="hidden" name="csrf" value="tok-signout"><input type="hidden" name="user_session" value="us1"><input type="hidden" name="next" value="signed_out"><input type="hidden" name="return_to" value="/id/p/acme/account/">`)

	gone := html(t, ctxIn(t, i18n.Indonesian), SignedOut(page(), SignedOutData{Site: site}))
	assert.Contains(t, gone, "Anda sudah keluar dari Acme Store di browser ini.")
	assert.Contains(t, gone, `href="/id/p/acme/login">Masuk lagi</a>`)
}

func TestViews_EscapeEndUserText(t *testing.T) {
	evil := `"><script>x</script>`
	for name, c := range map[string]templ.Component{
		"login":   Login(page(), LoginData{Site: Site{Handle: "acme", Project: evil}, Email: evil, Error: evil}),
		"picker":  Picker(page(), PickerData{Site: site, Accounts: []AccountTile{{SessionID: evil, Email: evil}}, Notice: evil}),
		"sent":    Sent(page(), SentData{Site: site, Email: evil}),
		"terms":   Terms(page(), TermsData{Site: site, Email: evil, UserSession: evil}),
		"account": Account(page(), AccountData{Site: site, Email: evil}),
	} {
		assert.NotContains(t, html(t, ctxIn(t, i18n.English), c), "<script>", name)
	}
}

func TestMail(t *testing.T) {
	m := MailData{Project: "Acme Store", Link: "https://id.example.com/p/acme/login/verify?token=abc", Minutes: 15, HasReplyTo: true, Primary: "#facc15"}
	en := ctxIn(t, i18n.English)
	assert.Equal(t, "Sign in to Acme Store", MailSubject(en, m))
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
	assert.Equal(t, "P", initial("product-ana@acme.test"), "B4: the avatar letter is upper-case")
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
		assert.Contains(t, alert, `aria-hidden="true"`, "%s: MF2 status icon", name)
		assert.NotContains(t, alert, "text-error\">", name)
	}
}

func TestTerms_HidesTheFooterLinks(t *testing.T) {
	d := page()
	d.Brand.TermsURL, d.Brand.PrivacyURL = "https://acme.test/terms", "https://acme.test/privacy"
	out := html(t, ctxIn(t, i18n.English), Terms(d, TermsData{Site: site, TermsURL: "https://acme.test/terms", PrivacyURL: "https://acme.test/privacy"}))
	assert.NotContains(t, out, "<nav", "D9: the consent line already links both")
	assert.Equal(t, 1, strings.Count(out, `href="https://acme.test/terms"`))
	assert.Equal(t, "https://acme.test/terms", d.Brand.TermsURL, "the caller's brand is not changed")
	assert.Contains(t, html(t, ctxIn(t, i18n.English), Login(d, LoginData{Site: site})), "<nav", "other pages keep the footer")
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
	assert.Contains(t, id, ">Lanjut ke Acme Store</a>")
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
	assert.Contains(t, plain, "Sign in to continue to Acme Store.")

	withClient := html(t, en, Login(page(), LoginData{Site: site, Client: "Openwa Web", Email: "ana@acme.test"}))
	assert.Contains(t, withClient, "Sign in to continue to Openwa Web.")
	assert.Contains(t, withClient, `value="ana@acme.test"`, "login_hint prefills the email")
	assert.NotContains(t, withClient, "asks you to sign in again")

	reauth := html(t, ctxIn(t, i18n.Indonesian), Login(page(), LoginData{Site: site, Client: "Openwa Web", Reauth: true}))
	assert.Contains(t, reauth, `<p class="text-sm text-base-content/70">Openwa Web meminta Anda masuk lagi.</p>`)
	assert.Contains(t, reauth, "Masuk untuk melanjutkan ke Openwa Web.")
	assert.NotContains(t, html(t, en, Login(page(), LoginData{Site: site, Reauth: true})), "sign in again", "no client, no reauth line")

	assert.Contains(t, html(t, en, Picker(page(), PickerData{Site: site})), "to continue to Acme Store")
	assert.Contains(t, html(t, en, Picker(page(), PickerData{Site: site, Client: "Openwa Web"})), "to continue to Openwa Web")
}

func TestLogin_ReauthHasNoWayBackToThePicker(t *testing.T) {
	out := html(t, ctxIn(t, i18n.English), Login(page(), LoginData{Site: site, Client: "Openwa Web", Reauth: true, FromPicker: true}))
	assert.NotContains(t, out, "Back to accounts", "M1: the picker would send the user straight back here")
	assert.Contains(t, out, "Openwa Web asks you to sign in again.")
}

func TestError_HelpLineOnlyWithSomethingToShare(t *testing.T) {
	en := ctxIn(t, i18n.English)
	declined := html(t, en, Error(page(), TermsDeclined(en, site)))
	assert.NotContains(t, declined, "reference ID", "M2: nothing to share")
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
