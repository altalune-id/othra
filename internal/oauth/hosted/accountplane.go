package hosted

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/enduser"
	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/oauth/oidc"
)

const signOutForm = "signout"

// EndUser resolves the account.v1 caller: the path issuer, this browser's live active account, and the project's terms.
func (h *Web) EndUser(w http.ResponseWriter, r *http.Request) (eu session.EndUser, answered bool, err error) {
	i, err := h.issuers.Resolve(r)
	var limited *oidc.LookupLimitedError
	switch {
	case errors.Is(err, oidc.ErrUnknownIssuer):
		http.NotFound(w, r)
		return session.EndUser{}, true, nil
	case errors.As(err, &limited):
		w.Header().Set("Retry-After", strconv.Itoa(limited.RetryAfter))
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return session.EndUser{}, true, nil
	case err != nil:
		return session.EndUser{}, false, err
	}
	s := h.site(i)
	p, secret := s.project(), h.cookies.browser(r)
	b, err := h.users.Browser(r.Context(), p, secret)
	if err != nil {
		return session.EndUser{}, false, err
	}
	a, ok := b.Active()
	if !ok {
		return session.EndUser{}, false, &enduser.SignedOutError{}
	}
	state, err := h.users.TermsState(r.Context(), p, secret, a.UserSession)
	switch {
	case codeOf(err) == apperror.CodeSignInSessionGone:
		return session.EndUser{}, false, &enduser.SignedOutError{}
	case err != nil:
		return session.EndUser{}, false, err
	case state.Needed:
		return session.EndUser{}, false, &enduser.TermsRequiredError{}
	}
	return session.EndUser{
		OrgID: i.OrgID, ProjectID: i.ProjectID, ProjectPublicID: i.PublicID, ProjectName: s.name, EndUserID: a.EndUserID,
		Subject: a.EndUser, UserSession: a.UserSession, AuthTime: a.AuthTime,
		SignOutToken: h.token(w, r, s, signOutForm), LinkToken: h.token(w, r, s, federationForm),
	}, false, nil
}
