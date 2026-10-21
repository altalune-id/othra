package layout

import (
	"context"
	"html"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/a-h/templ"
)

// OthraPrimary is Othra's primary colour as hex, for places that cannot read the theme, such as email.
const OthraPrimary = "#432dd7"

// CSRFField is the form field that carries the hosted pages' CSRF token.
const CSRFField = "csrf"

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Brand is a project's look on its hosted pages: data only, never tenant HTML, CSS or script.
type Brand struct {
	Name       string
	LogoURL    string
	Primary    string
	OnPrimary  string
	TermsURL   string
	PrivacyURL string
}

// NewBrand returns a brand whose primary colour is kept only when it is valid hex, with a readable text colour on it.
func NewBrand(name, logoURL, primary string) *Brand {
	b := &Brand{Name: name, LogoURL: logoURL}
	if hexColor.MatchString(primary) {
		b.Primary = primary
		b.OnPrimary = ReadableOn(primary)
	}
	return b
}

// ReadableOn returns white, or black when white has less than 4.5:1 contrast on hex (WCAG 2.2 §1.4.3).
func ReadableOn(hex string) string {
	if luminance(hex) <= 1.05/4.5-0.05 {
		return "#ffffff"
	}
	return "#000000"
}

func contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05)
}

func luminance(hex string) float64 {
	if !hexColor.MatchString(hex) {
		return 0
	}
	channel := func(i int) float64 {
		v, _ := strconv.ParseUint(hex[i:i+2], 16, 8)
		c := float64(v) / 255
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(1) + 0.7152*channel(3) + 0.0722*channel(5)
}

func (b *Brand) initial() string {
	for _, r := range b.Name {
		return strings.ToUpper(string(r))
	}
	return ""
}

// brandStyle writes the brand colours as CSS variables in a nonce style element; SECURITY: only re-checked hex reaches the CSS.
func brandStyle(b *Brand) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if b == nil || !hexColor.MatchString(b.Primary) {
			return nil
		}
		css := ":root{--color-primary:" + b.Primary + ";--color-primary-content:" + ReadableOn(b.Primary) + "}"
		_, err := io.WriteString(w, `<style nonce="`+html.EscapeString(templ.GetNonce(ctx))+`">`+css+`</style>`)
		return err
	})
}

type brandKey struct{}

func brandScope(b *Brand) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		children := templ.GetChildren(ctx)
		ctx = templ.ClearChildren(ctx)
		return children.Render(context.WithValue(ctx, brandKey{}, b), w)
	})
}

func brandFrom(ctx context.Context) *Brand {
	b, _ := ctx.Value(brandKey{}).(*Brand)
	return b
}

func mailButtonStyle(bg string) string {
	if !hexColor.MatchString(bg) {
		bg = OthraPrimary
	}
	return "display:inline-block;padding:10px 16px;background:" + bg + ";color:" + ReadableOn(bg) +
		";text-decoration:none;border-radius:6px;font-weight:600;"
}
