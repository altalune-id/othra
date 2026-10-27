package hosted

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/internal/federation"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/oauth/hosted/views"
)

const (
	bindingCookie   = "__Secure-othra_link"
	collisionCookie = "__Secure-othra_collision"
	linkingPage     = page("/federation/linking")
)

// NOTE: the identity a magic link carries; SignUp marks a first Google sign-in whose inbox the link proves.
type pendingLink struct {
	federation.Identity
	SignUp bool `json:"sign_up,omitempty"`
}

const modeSignUp = "sign_up"

// SECURITY: a collision never links; it mails the existing account a link that carries the identity, bound to this browser.
// NOTE: the answer is a 303 to a GET page, so a reload never replays the spent upstream code.
func (h *Web) collision(w http.ResponseWriter, r *http.Request, s *site, cerr *federation.CollisionError, returnTo string) {
	if s.issuer.SignInEmailLink && !h.sendPending(w, r, s, cerr.Email, returnTo, &pendingLink{Identity: cerr.Identity}) {
		return
	}
	h.toLinking(w, r, s, cerr.Email, "", returnTo)
}

// SECURITY: an unproven first sign-in creates nobody; the link proves the inbox, even with the email link off, on this browser.
func (h *Web) unproven(w http.ResponseWriter, r *http.Request, s *site, uerr *federation.UnprovenEmailError, returnTo string) {
	if !h.sendPending(w, r, s, uerr.Email, returnTo, &pendingLink{Identity: uerr.Identity, SignUp: true}) {
		return
	}
	h.toLinking(w, r, s, uerr.Email, modeSignUp, returnTo)
}

func (h *Web) sendPending(w http.ResponseWriter, r *http.Request, s *site, email, returnTo string, pending *pendingLink) bool {
	l, ok := h.sendLink(w, r, s, email, returnTo, pending)
	if ok && l.ID != "" {
		h.cookies.setBinding(w, s.verifyPath(), l.ID, h.cfg.LinkTTL)
	}
	return ok
}

func (h *Web) toLinking(w http.ResponseWriter, r *http.Request, s *site, email, mode, returnTo string) {
	value := url.Values{"e": {email}, "m": {mode}}.Encode()
	h.cookies.flash(collisionCookie, s.federationPath()).Set(w, value, h.cookies.now().Add(h.cfg.LinkTTL))
	h.toPage(w, r, s, linkingPage, backTo(returnTo))
}

func (h *Web) linking(w http.ResponseWriter, r *http.Request, s *site) {
	returnTo := h.returnTo(s, r.URL.Query().Get("return_to"))
	raw, ok := h.cookies.flash(collisionCookie, s.federationPath()).Get(r)
	v, err := url.ParseQuery(raw)
	signUp := v.Get("m") == modeSignUp
	switch {
	case !ok || err != nil || v.Get("e") == "":
		h.toPage(w, r, s, pageLogin, backTo(returnTo))
	case !signUp && !s.issuer.SignInEmailLink:
		h.upstreamFailed(w, r, s, returnTo, &federation.CollisionError{Email: v.Get("e")})
	default:
		h.kit.Render(w, r, http.StatusOK, views.Collision(h.layout(r, s), views.CollisionData{
			Site: s.views(), Email: v.Get("e"), Minutes: int(h.cfg.LinkTTL / time.Minute), ReturnTo: returnTo, SignUp: signUp,
		}))
	}
}

func (s *site) verifyPath() string { return s.path + "/login/verify" }

// SECURITY: the binding names the link it was set for and is signed, so only the browser that hit the collision can link.
func (c cookies) setBinding(w http.ResponseWriter, path, linkID string, ttl time.Duration) {
	http.SetCookie(w, cookie(bindingCookie, path, session.Sign(c.key, bindingCookie+"="+linkID), int(ttl.Seconds())))
}

func (c cookies) bound(r *http.Request, linkID string) bool {
	raw, err := r.Cookie(bindingCookie)
	if err != nil {
		return false
	}
	signed, err := session.Verify(c.key, raw.Value)
	return err == nil && signed == bindingCookie+"="+linkID
}

func (c cookies) clearBinding(w http.ResponseWriter, path string) {
	http.SetCookie(w, cookie(bindingCookie, path, "", -1))
}

// NOTE: the pending identity of l when this browser holds its binding; another device redeems l as a plain sign-in.
func (h *Web) pendingIdentity(r *http.Request, l LinkRef) (pendingLink, bool) {
	var pend pendingLink
	if h.fed == nil || l.Pending == nil || !h.cookies.bound(r, l.ID) || json.Unmarshal(l.Pending, &pend) != nil {
		return pendingLink{}, false
	}
	return pend, true
}

// SECURITY: a sign-up link links only the end user its redeem creates; one that appeared meanwhile signs in plainly, never auto-linked.
func (h *Web) confirmPending(
	r *http.Request, s *site,
) func(ctx context.Context, l LinkRef, endUserID uuid.UUID, created bool) (bool, error) {
	if h.fed == nil {
		return nil
	}
	skip := r.PostFormValue("skip") == "1"
	return func(ctx context.Context, l LinkRef, endUserID uuid.UUID, created bool) (bool, error) {
		pend, ok := h.pendingIdentity(r, l)
		switch {
		case !ok, pend.SignUp && !created, !pend.SignUp && skip:
			return false, nil
		}
		if err := h.fed.Link(ctx, fedProject(s), endUserID, pend.Identity); err != nil {
			return false, err
		}
		if pend.SignUp {
			return true, h.users.FillName(ctx, s.project(), endUserID, pend.Name)
		}
		return true, nil
	}
}
