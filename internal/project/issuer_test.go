package project

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
)

func TestValidateIssuerHandle(t *testing.T) {
	for _, h := range []string{"abc", "acme-web", "a1-b2", strings.Repeat("a", 40)} {
		assert.NoError(t, ValidateIssuerHandle(h), h)
	}
	cases := map[string]IssuerHandleReason{
		"ab":                    IssuerHandleLength,
		strings.Repeat("a", 41): IssuerHandleLength,
		"Acme":                  IssuerHandleFormat,
		"-acme":                 IssuerHandleFormat,
		"acme-":                 IssuerHandleFormat,
		"ac_me":                 IssuerHandleFormat,
		"ac me":                 IssuerHandleFormat,
	}
	for _, r := range reservedIssuerHandles {
		cases[r] = IssuerHandleReserved
	}
	for h, want := range cases {
		err := ValidateIssuerHandle(h)
		var e *InvalidIssuerHandleError
		require.True(t, errors.As(err, &e), "%q: %v", h, err)
		assert.Equal(t, want, e.Reason, h)
	}
}

func TestDefaultIssuerHandle(t *testing.T) {
	assert.Equal(t, "acme-web", DefaultIssuerHandle("acme", "web"))
	long := DefaultIssuerHandle(strings.Repeat("o", 30), strings.Repeat("p", 30))
	assert.Len(t, long, issuerHandleMaxLen)
	assert.NoError(t, ValidateIssuerHandle(long))
	assert.Equal(t, strings.Repeat("o", 30)+"-pppppppp"+"p", long)
	cut := DefaultIssuerHandle(strings.Repeat("o", 39), "web")
	assert.Equal(t, strings.Repeat("o", 39), cut, "never ends with a dash")

	suffixed := withIssuerSuffix("acme-web")
	assert.Regexp(t, `^acme-web-[a-z0-9]{4}$`, suffixed)
	assert.NoError(t, ValidateIssuerHandle(suffixed))
	longSuffixed := withIssuerSuffix(strings.Repeat("a", 40))
	assert.Len(t, longSuffixed, issuerHandleMaxLen)
	assert.NoError(t, ValidateIssuerHandle(longSuffixed))
}

func TestNewProject_GetsAUniqueishIssuerHandle(t *testing.T) {
	a, err := NewProject(uuid.New(), "web", "Web", uuid.Nil)
	require.NoError(t, err)
	b, err := NewProject(uuid.New(), "web", "Web", uuid.Nil)
	require.NoError(t, err)
	assert.Regexp(t, `^web-[a-z0-9]{4}$`, a.IssuerHandle)
	assert.NoError(t, ValidateIssuerHandle(a.IssuerHandle))
	assert.True(t, a.SignInEmailLink, "the email link is on by default")
	assert.Zero(t, a.TermsVersion)
	_ = b
}

func TestProject_SetIssuerHandle(t *testing.T) {
	p := &Project{IssuerHandle: "acme-web"}
	require.NoError(t, p.SetIssuerHandle("acme-shop"))
	assert.Equal(t, "acme-shop", p.IssuerHandle)
	assert.True(t, IsInvalidIssuerHandleError(p.SetIssuerHandle("system")))

	now := time.Now()
	p.LockIssuerHandle(now)
	p.LockIssuerHandle(now.Add(time.Hour))
	assert.True(t, p.IssuerHandleLockedAt.Equal(now), "locking twice keeps the first time")
	assert.True(t, IsIssuerHandleLockedError(p.SetIssuerHandle("acme-other")))
	assert.NoError(t, p.SetIssuerHandle("acme-shop"), "setting the same handle is not a change")
}

func TestIssuerURL(t *testing.T) {
	assert.Equal(t, "https://id.example.com/p/acme-web", IssuerURL("https://id.example.com", "acme-web"))
	assert.Equal(t, "https://id.example.com/base/p/acme-web", IssuerURL("https://id.example.com/base/", "acme-web"))
}

func violation(t *testing.T, err error) apperror.Violation {
	t.Helper()
	ae, ok := apperror.AsAppError(err)
	require.True(t, ok, "%v", err)
	require.Len(t, ae.Violations(), 1)
	return ae.Violations()[0]
}

func TestProject_UpdateBranding(t *testing.T) {
	p := &Project{}
	require.NoError(t, p.UpdateBranding("  Acme Shop  ", "#4F46E5", " help@acme.test "))
	assert.Equal(t, "Acme Shop", p.DisplayName)
	assert.Equal(t, "#4F46E5", p.PrimaryColor)
	assert.Equal(t, "help@acme.test", p.SupportEmail)
	require.NoError(t, p.UpdateBranding("", "", ""), "every branding field is optional")

	v := violation(t, p.UpdateBranding(strings.Repeat("d", 61), "", ""))
	assert.Equal(t, "display_name", v.Field)
	assert.Equal(t, apperror.FieldTooLong, v.Reason)
	assert.Equal(t, "60", v.Param)
	for _, bad := range []string{"4F46E5", "#4F46E", "#GGGGGG", "red"} {
		assert.Equal(t, "primary_color", violation(t, p.UpdateBranding("", bad, "")).Field, bad)
	}
	for _, bad := range []string{"nope", "a@", "@b", "a b@c.d"} {
		assert.Equal(t, "support_email", violation(t, p.UpdateBranding("", "", bad)).Field, bad)
	}
	assert.Equal(t, "", p.DisplayName, "a refused update changes nothing")
}

func TestProject_UpdateTerms(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	p := &Project{}
	require.NoError(t, p.UpdateTerms("", "https://acme.test/privacy", now))
	assert.Zero(t, p.TermsVersion, "a privacy link alone publishes nothing")

	require.NoError(t, p.UpdateTerms("https://acme.test/terms", "https://acme.test/privacy", now))
	assert.Equal(t, int32(1), p.TermsVersion, "setting the terms link publishes version 1")
	require.NotNil(t, p.TermsPublishedAt)
	assert.True(t, p.TermsPublishedAt.Equal(now))

	later := now.Add(time.Hour)
	require.NoError(t, p.UpdateTerms("https://acme.test/terms-v2", "", later))
	assert.Equal(t, int32(1), p.TermsVersion, "changing a set link never bumps")

	require.NoError(t, p.UpdateTerms("", "", later))
	assert.Equal(t, int32(1), p.TermsVersion, "clearing the link turns the gate off and keeps the version")
	require.NoError(t, p.UpdateTerms("https://acme.test/terms", "", later))
	assert.Equal(t, int32(2), p.TermsVersion, "setting it again publishes a new version")

	for _, bad := range []string{"http://acme.test/terms", "ftp://x", "https://", "acme.test/terms", "https://" + strings.Repeat("a", 2048)} {
		assert.Equal(t, "terms_url", violation(t, p.UpdateTerms(bad, "", later)).Field, bad)
	}
	assert.Equal(t, "privacy_url", violation(t, p.UpdateTerms("", "javascript:alert(1)", later)).Field)
}

func TestProject_BumpTermsVersion(t *testing.T) {
	now := time.Now().UTC()
	p := &Project{}
	_, err := p.BumpTermsVersion(now)
	assert.True(t, IsTermsNotSetError(err))

	require.NoError(t, p.UpdateTerms("https://acme.test/terms", "", now))
	v, err := p.BumpTermsVersion(now.Add(time.Minute))
	require.NoError(t, err)
	assert.Equal(t, int32(2), v)
	assert.True(t, p.TermsPublishedAt.Equal(now.Add(time.Minute)))
}

func TestValidateLogo(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 16)...)
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 16)...)
	webp := append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 16)...)
	for want, content := range map[string][]byte{"image/png": png, "image/jpeg": jpeg, "image/webp": webp} {
		got, err := ValidateLogo(content)
		require.NoError(t, err, want)
		assert.Equal(t, want, got)
	}
	refused := map[string]struct {
		content []byte
		reason  LogoReason
	}{
		"svg":   {[]byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), LogoType},
		"gif":   {[]byte("GIF89a......"), LogoType},
		"text":  {[]byte("hello, logo"), LogoType},
		"empty": {nil, LogoSize},
		"large": {append(bytes.Clone(png), make([]byte, logoMaxBytes)...), LogoSize},
	}
	for name, c := range refused {
		_, err := ValidateLogo(c.content)
		var e *InvalidLogoError
		require.True(t, errors.As(err, &e), "%s: %v", name, err)
		assert.Equal(t, c.reason, e.Reason, name)
	}
}
