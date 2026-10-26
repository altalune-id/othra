package oidcrp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
)

const (
	defaultTimeout   = 10 * time.Second
	defaultClockSkew = 30 * time.Second
	maxIDTokenAge    = 10 * time.Minute
	jwksMinRefetch   = 30 * time.Second
	maxBodyBytes     = 1 << 20
	randomBytes      = 32
	maxSubjectLength = 255
)

//nolint:gochecknoglobals // the ID-token signing algorithms a relying party accepts; never none or an HMAC.
var allowedAlgs = []jose.SignatureAlgorithm{jose.RS256, jose.ES256, jose.EdDSA}

//nolint:gochecknoglobals // the error codes of RFC 6749 §4.1.2.1 and OIDC Core §3.1.2.6; any other value is logged as "other".
var knownErrors = []string{
	"invalid_request", "unauthorized_client", "access_denied", "unsupported_response_type", "invalid_scope", "server_error",
	"temporarily_unavailable", "interaction_required", "login_required", "account_selection_required", "consent_required",
	"invalid_request_uri", "invalid_request_object", "request_not_supported", "request_uri_not_supported", "registration_not_supported",
}

//nolint:gochecknoglobals // how long discovery and the JWKS are trusted when Cache-Control says nothing usable.
var (
	metadataPolicy = ttlPolicy{floor: 5 * time.Minute, ceiling: 24 * time.Hour, fallback: 24 * time.Hour}
	jwksPolicy     = ttlPolicy{floor: time.Minute, ceiling: 24 * time.Hour, fallback: 15 * time.Minute}
)

// Client is a relying party for one upstream IdP and one client registration; safe for concurrent use.
type Client struct {
	cfg  Config
	http *http.Client
	now  func() time.Time

	meta *cache[Metadata]
	keys *cache[jose.JSONWebKeySet]

	mu          sync.Mutex
	lastRefetch time.Time
}

// New builds a Client; cfg.HTTPClient should refuse redirects and private hosts, as httpclient.New does.
func New(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{
			Timeout:       defaultTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	if cfg.ClockSkew <= 0 {
		cfg.ClockSkew = defaultClockSkew
	}
	return &Client{
		cfg: cfg, http: hc, now: time.Now, meta: &cache[Metadata]{policy: metadataPolicy}, keys: &cache[jose.JSONWebKeySet]{policy: jwksPolicy},
	}
}

// WithClock replaces the clock, for tests.
func (c *Client) WithClock(now func() time.Time) *Client {
	c.now = now
	return c
}

// NewAuthRequest makes the per-attempt secrets: state, nonce and the PKCE verifier, 256 random bits each.
func NewAuthRequest(prompt, loginHint string) (AuthRequest, error) {
	var vals [3]string
	for i := range vals {
		b := make([]byte, randomBytes)
		if _, err := rand.Read(b); err != nil {
			return AuthRequest{}, fmt.Errorf("oidcrp: random: %w", err)
		}
		vals[i] = base64.RawURLEncoding.EncodeToString(b)
	}
	return AuthRequest{State: vals[0], Nonce: vals[1], CodeVerifier: vals[2], Prompt: prompt, LoginHint: loginHint}, nil
}

// AuthCodeURL is the upstream IdP's authorization URL for req: the code flow with S256 PKCE, state and nonce (RFC 9700 §2.1.1).
func (c *Client) AuthCodeURL(ctx context.Context, req AuthRequest) (string, error) {
	meta, err := c.metadata(ctx)
	if err != nil {
		return "", err
	}
	if !slices.Contains(meta.CodeChallengeMethodsSupported, "S256") {
		return "", &Error{Reason: ReasonDiscovery, Err: errors.New("the upstream IdP doesn't advertise S256 PKCE")}
	}
	u, err := url.Parse(meta.AuthorizationEndpoint)
	if err != nil {
		return "", &Error{Reason: ReasonDiscovery, Err: err}
	}
	sum := sha256.Sum256([]byte(req.CodeVerifier))
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", c.cfg.ClientID)
	q.Set("redirect_uri", c.cfg.RedirectURI)
	q.Set("scope", strings.Join(c.cfg.Scopes, " "))
	q.Set("state", req.State)
	q.Set("nonce", req.Nonce)
	q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(sum[:]))
	q.Set("code_challenge_method", "S256")
	if req.Prompt != "" {
		q.Set("prompt", req.Prompt)
	}
	if req.LoginHint != "" {
		q.Set("login_hint", req.LoginHint)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ParseCallback reads the authorization response from the redirect URI's query.
func ParseCallback(q url.Values) Callback {
	return Callback{
		Code: q.Get("code"), State: q.Get("state"), Issuer: q.Get("iss"), Error: q.Get("error"), ErrorDescription: q.Get("error_description"),
	}
}

// Exchange checks the callback against req, redeems the code and returns the validated ID token; the upstream IdP's tokens are dropped.
func (c *Client) Exchange(ctx context.Context, cb Callback, req AuthRequest) (*IDToken, error) {
	if req.State == "" || subtle.ConstantTimeCompare([]byte(cb.State), []byte(req.State)) != 1 {
		return nil, &Error{Reason: ReasonState, Err: errors.New("state does not match")}
	}
	meta, err := c.metadata(ctx)
	if err != nil {
		return nil, err
	}
	// SECURITY: RFC 9207 §2.4: a present iss must name the upstream IdP, or it is a mix-up; a code needs iss when it is advertised.
	// SECURITY: Google interop: Google omits iss on error responses; an error with no code and no iss has nothing to inject or steal.
	missing := cb.Issuer == "" && meta.AuthorizationResponseISSParams && (cb.Code != "" || cb.Error == "")
	if missing || cb.Issuer != "" && cb.Issuer != meta.Issuer {
		return nil, &Error{Reason: ReasonIssuer, Err: errors.New("the authorization response iss does not match")}
	}
	switch {
	case cb.Error == "access_denied":
		return nil, &Error{Reason: ReasonCancelled, Err: errors.New("access_denied")}
	case cb.Error != "":
		return nil, &Error{Reason: ReasonError, Err: errors.New("the upstream IdP answered " + errorCode(cb.Error))}
	case cb.Code == "":
		return nil, &Error{Reason: ReasonState, Err: errors.New("no code")}
	}
	raw, err := c.redeem(ctx, meta, cb.Code, req.CodeVerifier)
	if err != nil {
		return nil, err
	}
	return c.verify(ctx, meta, raw, req.Nonce)
}

// NOTE: the error parameter is attacker-controlled and unbounded, so only a registered code reaches a log line.
func errorCode(code string) string {
	if slices.Contains(knownErrors, code) {
		return code
	}
	return "other"
}

func (c *Client) redeem(ctx context.Context, meta *Metadata, code, verifier string) (string, error) {
	form := url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {c.cfg.RedirectURI}, "code_verifier": {verifier},
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", &Error{Reason: ReasonTokenExchange, Err: err}
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Accept", "application/json")
	r.SetBasicAuth(url.QueryEscape(c.cfg.ClientID), url.QueryEscape(c.cfg.ClientSecret))
	var out struct {
		IDToken   string `json:"id_token"`
		TokenType string `json:"token_type"`
	}
	if _, err := c.doJSON(r, ReasonTokenExchange, &out); err != nil {
		return "", err
	}
	if out.IDToken == "" {
		return "", &Error{Reason: ReasonTokenExchange, Err: errors.New("the token response has no id_token")}
	}
	return out.IDToken, nil
}

type idTokenClaims struct {
	Issuer        string   `json:"iss"`
	Subject       string   `json:"sub"`
	Audience      audience `json:"aud"`
	AuthorizedBy  string   `json:"azp"`
	ExpiresAt     int64    `json:"exp"`
	IssuedAt      int64    `json:"iat"`
	Nonce         string   `json:"nonce"`
	Email         string   `json:"email"`
	EmailVerified flexBool `json:"email_verified"`
	Name          string   `json:"name"`
	HostedDomain  string   `json:"hd"`
}

// SECURITY: OIDC Core §3.1.3.7: signature by a published key with an allowed alg, iss, aud, azp, exp, iat and nonce, all fail closed.
func (c *Client) verify(ctx context.Context, meta *Metadata, raw, nonce string) (*IDToken, error) {
	jws, err := jose.ParseSigned(raw, signingAlgs(meta))
	if err != nil || len(jws.Signatures) != 1 {
		return nil, &Error{Reason: ReasonIDToken, Err: fmt.Errorf("not a single-signature JWS with an allowed alg: %w", err)}
	}
	header := jws.Signatures[0].Header
	key, err := c.key(ctx, meta, header.KeyID)
	if err != nil {
		return nil, err
	}
	if key.Algorithm != "" && key.Algorithm != header.Algorithm {
		return nil, &Error{Reason: ReasonIDToken, Err: errors.New("the key's alg is not the token's")}
	}
	payload, err := jws.Verify(key)
	if err != nil {
		return nil, &Error{Reason: ReasonIDToken, Err: fmt.Errorf("signature: %w", err)}
	}
	var cl idTokenClaims
	if err := json.Unmarshal(payload, &cl); err != nil {
		return nil, &Error{Reason: ReasonIDToken, Err: fmt.Errorf("claims: %w", err)}
	}
	now, skew := c.now(), c.cfg.ClockSkew
	iat := time.Unix(cl.IssuedAt, 0)
	switch {
	case cl.Issuer != c.cfg.Issuer && !slices.Contains(c.cfg.AcceptIssuers, cl.Issuer):
		return nil, &Error{Reason: ReasonIDToken, Err: errors.New("iss does not match")}
	case len(cl.Audience) == 0 || slices.ContainsFunc(cl.Audience, func(a string) bool { return a != c.cfg.ClientID }):
		return nil, &Error{Reason: ReasonIDToken, Err: errors.New("aud is not exactly this client")}
	case cl.AuthorizedBy != "" && cl.AuthorizedBy != c.cfg.ClientID:
		return nil, &Error{Reason: ReasonIDToken, Err: errors.New("azp is another client")}
	case cl.ExpiresAt == 0 || !now.Before(time.Unix(cl.ExpiresAt, 0).Add(skew)):
		return nil, &Error{Reason: ReasonIDToken, Err: errors.New("expired")}
	case cl.IssuedAt == 0 || iat.After(now.Add(skew)) || iat.Before(now.Add(-maxIDTokenAge)):
		return nil, &Error{Reason: ReasonIDToken, Err: errors.New("iat is missing, in the future or too old")}
	case nonce == "" || subtle.ConstantTimeCompare([]byte(cl.Nonce), []byte(nonce)) != 1:
		return nil, &Error{Reason: ReasonNonce, Err: errors.New("nonce does not match")}
	case cl.Subject == "" || len(cl.Subject) > maxSubjectLength:
		return nil, &Error{Reason: ReasonIDToken, Err: errors.New("sub is missing or too long")}
	}
	return &IDToken{
		Issuer: cl.Issuer, Subject: cl.Subject, Audience: cl.Audience, Email: cl.Email, EmailVerified: bool(cl.EmailVerified),
		Name: cl.Name, HostedDomain: cl.HostedDomain, IssuedAt: iat, ExpiresAt: time.Unix(cl.ExpiresAt, 0),
	}, nil
}

// NOTE: OIDC Core §3.1.3.7 step 7: the upstream IdP's advertised algs, RS256 when it names none, never beyond allowedAlgs.
func signingAlgs(meta *Metadata) []jose.SignatureAlgorithm {
	if len(meta.IDTokenSigningAlgValues) == 0 {
		return []jose.SignatureAlgorithm{jose.RS256}
	}
	out := make([]jose.SignatureAlgorithm, 0, len(allowedAlgs))
	for _, a := range allowedAlgs {
		if slices.Contains(meta.IDTokenSigningAlgValues, string(a)) {
			out = append(out, a)
		}
	}
	return out
}

func (c *Client) metadata(ctx context.Context) (*Metadata, error) {
	return c.meta.get(ctx, c.now(), false, c.fetchMetadata)
}

// SECURITY: OIDC Discovery §4.3, the document must name the configured issuer exactly, or it describes another upstream IdP.
func (c *Client) fetchMetadata(ctx context.Context) (*Metadata, http.Header, error) {
	discovery := strings.TrimRight(c.cfg.Issuer, "/") + "/.well-known/openid-configuration"
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, discovery, http.NoBody)
	if err != nil {
		return nil, nil, &Error{Reason: ReasonDiscovery, Err: err}
	}
	var meta Metadata
	h, err := c.doJSON(r, ReasonDiscovery, &meta)
	switch {
	case err != nil:
		return nil, nil, err
	case meta.Issuer != c.cfg.Issuer:
		return nil, nil, &Error{Reason: ReasonMisconfigured, Err: errors.New("the discovery issuer does not match")}
	case !sameScheme(c.cfg.Issuer, meta.AuthorizationEndpoint, meta.TokenEndpoint, meta.JWKSURI):
		return nil, nil, &Error{Reason: ReasonDiscovery, Err: errors.New("an endpoint is missing or not on the issuer's scheme")}
	}
	return &meta, h, nil
}

// NOTE: OIDC Core §10.1.1 rotation: an unknown kid refetches the JWKS once, at most every 30 s.
func (c *Client) key(ctx context.Context, meta *Metadata, kid string) (*jose.JSONWebKey, error) {
	fetch := func(ctx context.Context) (*jose.JSONWebKeySet, http.Header, error) {
		return c.fetchKeys(ctx, meta.JWKSURI)
	}
	now := c.now()
	set, err := c.keys.get(ctx, now, false, fetch)
	if err != nil {
		return nil, err
	}
	if k := find(set, kid); k != nil {
		return k, nil
	}
	c.mu.Lock()
	limited := now.Sub(c.lastRefetch) < jwksMinRefetch
	if !limited {
		c.lastRefetch = now
	}
	c.mu.Unlock()
	if limited {
		return nil, &Error{Reason: ReasonIDToken, Err: errors.New("unknown kid")}
	}
	if set, err = c.keys.get(ctx, now, true, fetch); err != nil {
		return nil, err
	}
	if k := find(set, kid); k != nil {
		return k, nil
	}
	return nil, &Error{Reason: ReasonIDToken, Err: errors.New("unknown kid")}
}

func (c *Client) fetchKeys(ctx context.Context, jwksURI string) (*jose.JSONWebKeySet, http.Header, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, http.NoBody)
	if err != nil {
		return nil, nil, &Error{Reason: ReasonJWKS, Err: err}
	}
	var set jose.JSONWebKeySet
	h, err := c.doJSON(r, ReasonJWKS, &set)
	if err != nil {
		return nil, nil, err
	}
	return &set, h, nil
}

func find(set *jose.JSONWebKeySet, kid string) *jose.JSONWebKey {
	for i, k := range set.Keys {
		if k.KeyID == kid && k.IsPublic() && (k.Use == "" || k.Use == "sig") {
			return &set.Keys[i]
		}
	}
	return nil
}

func (c *Client) doJSON(r *http.Request, reason Reason, dst any) (http.Header, error) {
	resp, err := c.http.Do(r)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			reason = ReasonTimeout
		}
		return nil, &Error{Reason: reason, Err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	switch {
	case err != nil:
		return nil, &Error{Reason: reason, Err: err}
	case resp.StatusCode != http.StatusOK:
		return nil, &Error{Reason: reason, Err: fmt.Errorf("status %d", resp.StatusCode)}
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return nil, &Error{Reason: reason, Err: err}
	}
	return resp.Header, nil
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func sameScheme(issuer string, endpoints ...string) bool {
	scheme := "https://"
	if strings.HasPrefix(issuer, "http://") {
		scheme = "http://"
	}
	for _, e := range endpoints {
		if !strings.HasPrefix(e, scheme) || len(e) == len(scheme) {
			return false
		}
	}
	return true
}

type audience []string

func (a *audience) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*a = []string{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

type flexBool bool

func (f *flexBool) UnmarshalJSON(b []byte) error {
	var v bool
	if err := json.Unmarshal(b, &v); err == nil {
		*f = flexBool(v)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*f = flexBool(s == "true")
	return nil
}
