package layout

// PortalValues are one request's end-user portal settings: the router base and the project's endpoints and brand.
type PortalValues struct {
	BaseURL    string
	APIURL     string
	HostedURL  string
	Handle     string
	Project    string
	LogoURL    string
	TermsURL   string
	PrivacyURL string
	BrandCSS   string
	Nonce      string
}
