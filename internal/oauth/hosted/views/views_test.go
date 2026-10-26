package views

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
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
	assert.Contains(t, out, "<title>Allow Claude Code to access Openwa MCP? · Acme Store</title>")
	assert.Contains(t, out, `<span class="font-medium">Openwa MCP</span>`)
	assert.Contains(t, out, `https://openwa.example.com/mcp</span>`)
	assert.Contains(t, out, `aria-hidden="true">C</span>`, "a letter avatar, never a hot-linked logo")
	assert.NotContains(t, out, "<img")

	assert.Contains(t, out, `role="note" class="alert alert-warning alert-soft`)
	assert.Contains(t, out, "Unverified app")
	assert.Contains(t, out, "Acme Store hasn&#39;t reviewed this app. Its details come from claude.ai.")

	assert.Contains(t, out, `<span class="block text-sm font-medium">Read your messages</span><span class="block font-mono text-xs text-base-content/70">messages:read</span>`)
	assert.Contains(t, out, `<span class="block text-sm font-medium">Stay connected when you&#39;re away</span>`, "offline_access is its own line")
	assert.Contains(t, out, `<span class="block text-sm font-medium">devices:write</span>`, "no description: the raw scope")
	assert.Contains(t, out, `<span class="badge badge-ghost badge-sm">Allowed before</span>`)
	order := []int{strings.Index(out, "messages:read"), strings.Index(out, ">offline_access<"), strings.Index(out, "devices:write"), strings.Index(out, ">openid<")}
	assert.IsIncreasing(t, order, "new scopes first, in request order; allowed-before last")

	assert.Contains(t, out, "After you allow, you&#39;ll go back to <span class=\"font-medium\">localhost</span>.")
	assert.Contains(t, out, `<p class="font-medium">That&#39;s a program on this device. Continue only if you started this sign-in.</p>`)

	form := out[strings.Index(out, `<form method="post" action="/id/p/acme/consent"`):]
	assert.Contains(t, form, `<input type="hidden" name="csrf" value="tok-consent"><input type="hidden" name="user_session" value="us1"><input type="hidden" name="return_to" value="/id/p/acme/oauth2/authorize?client_id=x">`)
	assert.Contains(t, form, `<button class="btn btn-primary btn-block" type="submit">Allow</button><button class="btn btn-ghost btn-block" type="submit" name="deny" value="1" formnovalidate>Deny</button>`)
	assert.Contains(t, out, `href="/id/p/acme/login?add=1&amp;return_to=%2Fid%2Fp%2Facme%2Foauth2%2Fauthorize%3Fclient_id%3Dx">Use another account</a>`)
	assert.Contains(t, out, `<span class="block truncate font-medium" title="ana@acme.test">ana@acme.test</span>`)
}

func TestConsent_RegisteredAccountTitleNoLoopback(t *testing.T) {
	v := consentData()
	v.ClientHost, v.RS, v.RSIdentifier, v.Loopback, v.RedirectHost = "", "", "", false, "app.example"
	out := html(t, ctxIn(t, i18n.Indonesian), Consent(page(), v))
	assert.Contains(t, out, "Izinkan Claude Code mengakses akun Acme Store Anda?")
	assert.NotContains(t, out, "alert-warning", "no CIMD note for a registered client")
	assert.NotContains(t, out, "perangkat ini", "no loopback warning")
	assert.Contains(t, out, `Setelah Anda mengizinkan, Anda akan kembali ke <span class="font-medium">app.example</span>.`)
	assert.Contains(t, out, "Mengetahui siapa Anda")
	assert.Contains(t, out, ">Izinkan</button>")
}

func TestConsent_Remembered(t *testing.T) {
	v := consentData()
	v.Remembered = true
	out := html(t, ctxIn(t, i18n.English), Consent(page(), v))
	assert.Contains(t, out, "<title>Continue to Claude Code? · Acme Store</title>")
	assert.Contains(t, out, "You allowed Claude Code to access Openwa MCP before.")
	assert.Contains(t, out, `<p class="text-xs text-base-content/70">From claude.ai · unverified app</p>`)
	assert.NotContains(t, out, "alert-warning", "the remembered page has one muted line, not the box")
	assert.NotContains(t, out, "messages:read", "no scope list")
	assert.Contains(t, out, "You&#39;ll go back to <span class=\"font-medium\">localhost</span>.")
	assert.Contains(t, out, "That&#39;s a program on this device.")
	assert.Contains(t, out, `<button class="btn btn-primary btn-block" type="submit">Continue to Claude Code</button>`)
	assert.Contains(t, out, `name="deny" value="1" formnovalidate>Deny</button>`)
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
	assert.Contains(t, login, "Sign in to continue to Claude Code.")
	assert.Contains(t, login, line, "R-10: a self-asserted name always comes with its host")
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

func TestConsent_AppMarkSitsInTheTitleRow(t *testing.T) {
	en := ctxIn(t, i18n.English)
	for name, v := range map[string]ConsentData{
		"first":      consentData(),
		"remembered": func() ConsentData { v := consentData(); v.Remembered = true; return v }(),
		"account":    func() ConsentData { v := consentData(); v.RS, v.RSIdentifier = "", ""; return v }(),
	} {
		out := strings.ReplaceAll(html(t, en, Consent(page(), v)), "> <", "><")
		assert.Regexp(t, `<div class="flex items-center gap-3"><span class="[^"]*rounded-full[^"]*" aria-hidden="true">C</span><h1 `, out, "%s: MF3 the app mark is the title's", name)
	}
	first := strings.ReplaceAll(html(t, en, Consent(page(), consentData())), "> <", "><")
	assert.Contains(t, first, `</h1></div><p class="text-sm"><span class="font-medium">Openwa MCP</span><span class="block font-mono text-xs break-all text-base-content/70">https://openwa.example.com/mcp</span></p>`, "the resource line is text only, under the title")
}

func TestLogin_Google(t *testing.T) {
	both := html(t, ctxIn(t, i18n.English), Login(page(), LoginData{Site: site, ReturnTo: "/id/p/acme/account/", LoginToken: "tok-login", Google: true, GoogleToken: "tok-google"}))
	assert.Contains(t, both, `<form method="post" action="/id/p/acme/federation/google/start"><input type="hidden" name="csrf" value="tok-google"><input type="hidden" name="return_to" value="/id/p/acme/account/">`)
	assert.Contains(t, both, `<button class="btn-google" type="submit"><svg`)
	assert.Contains(t, both, `<span>Continue with Google</span></button>`)
	assert.Contains(t, both, "or continue with email")
	assert.Less(t, strings.Index(both, "btn-google"), strings.Index(both, `name="email"`), "Google first, then the email form")

	only := html(t, ctxIn(t, i18n.Indonesian), Login(page(), LoginData{Site: site, Google: true, EmailOff: true}))
	assert.Contains(t, only, "Lanjutkan dengan Google", "MF5: Google's own Indonesian wording")
	assert.NotContains(t, only, `name="email"`)
	assert.NotContains(t, only, "atau lanjut dengan email")

	plain := html(t, ctxIn(t, i18n.English), Login(page(), LoginData{Site: site, EmailOff: true}))
	assert.Contains(t, plain, `name="email"`, "no Google, so the email form stays")
	assert.NotContains(t, plain, "btn-google")
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

func TestAccount_SignInMethods(t *testing.T) {
	en := ctxIn(t, i18n.English)
	none := html(t, en, Account(page(), AccountData{Site: site, Email: "ana@acme.test"}))
	assert.NotContains(t, none, "Sign-in methods", "no Google anywhere, no card")

	linked := html(t, en, Account(page(), AccountData{Site: site, Email: "ana@acme.test", Google: &GoogleMethod{Identity: "idn1", Email: "ana.s@gmail.com", Token: "tok-unlink"}, Linked: true}))
	assert.Contains(t, linked, `<h2 id="methods-title" class="text-sm font-semibold">Sign-in methods</h2>`)
	assert.Contains(t, linked, "Email link")
	assert.Contains(t, linked, `<form method="post" action="/id/p/acme/account/unlink"><input type="hidden" name="csrf" value="tok-unlink"><input type="hidden" name="identity" value="idn1">`)
	assert.Contains(t, linked, `aria-label="Disconnect Google">Disconnect</button>`)
	assert.Contains(t, linked, `role="status"`)
	assert.Contains(t, linked, "Google is connected.")
	assert.NotContains(t, linked, "Connect Google")

	open := html(t, ctxIn(t, i18n.Indonesian), Account(page(), AccountData{Site: site, Email: "ana@acme.test", Google: &GoogleMethod{Token: "tok-link"}, Unlinked: true}))
	assert.Contains(t, open, `<form method="post" action="/id/p/acme/federation/google/start"><input type="hidden" name="csrf" value="tok-link"><input type="hidden" name="intent" value="link">`)
	assert.Contains(t, open, "Hubungkan Google")
	assert.Contains(t, open, "Belum terhubung")
	assert.Contains(t, open, "Anda tetap bisa masuk dengan tautan email.")

	googleOnly := html(t, en, Account(page(), AccountData{Site: site, Email: "ana@acme.test", EmailLinkOff: true, Google: &GoogleMethod{Identity: "idn1", Email: "ana@acme.test"}, Unlinked: true, Error: "You can't remove your only way to sign in."}))
	assert.NotContains(t, googleOnly, "Email link")
	assert.NotContains(t, googleOnly, "You can still sign in with an email link.")
	assert.Contains(t, googleOnly, `role="alert"`)
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

// A18, N2: the collision link's email says it adds Google, in both locales.
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
			assert.Contains(t, out, "Acme Store", "MF6 %s %s", code, locale)
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
	assert.Contains(t, login, "atau lanjutkan dengan email")
}

// MF7: a failed Connect Google is titled as a connect and leads back to the account, never to sign in.
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
	assert.Contains(t, out, "Menghubungkan Google dibatalkan. Tidak ada yang berubah.", "BL-3: the connect sentence")
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

// P8 = A: the Google sign-up link's email and page, in both locales.
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

// O11, P8: a sign-up link's confirm page says it finishes the Google sign-up, in both locales.
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
