package project

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
)

func TestErrors(t *testing.T) {
	assert.Contains(t, (&NotFoundError{Slug: "s"}).Error(), "s")
	assert.Contains(t, (&NotFoundError{ID: "i"}).Error(), "i")
	assert.Contains(t, (*NotFoundError)(nil).Error(), "not found")
	assert.Contains(t, (&NotFoundError{}).Error(), "not found")
	assert.True(t, IsNotFoundError(&NotFoundError{}))
	assert.False(t, IsNotFoundError(nil))

	assert.Contains(t, (&AlreadyExistsError{Slug: "s"}).Error(), "s")
	assert.True(t, IsAlreadyExistsError(&AlreadyExistsError{}))
	assert.False(t, IsAlreadyExistsError(nil))

	assert.Contains(t, (&InvalidSlugError{Slug: "s"}).Error(), "slug")
	assert.Contains(t, (&InvalidSlugError{Reason: SlugLength}).Error(), "must be 3 to 64 characters")
	assert.True(t, IsInvalidSlugError(&InvalidSlugError{}))
	assert.False(t, IsInvalidSlugError(nil))

	assert.Contains(t, (&InvalidNameError{}).Error(), "name")
	assert.Contains(t, (&InvalidNameError{Reason: NameLength}).Error(), "must be 1 to 200 characters")
	assert.True(t, IsInvalidNameError(&InvalidNameError{}))
	assert.False(t, IsInvalidNameError(nil))
}

func TestIsExpectedError(t *testing.T) {
	assert.True(t, IsExpectedError(&NotFoundError{}))
	assert.True(t, IsExpectedError(&AlreadyExistsError{}))
	assert.True(t, IsExpectedError(&InvalidSlugError{}))
	assert.True(t, IsExpectedError(&InvalidNameError{}))
	assert.False(t, IsExpectedError(assert.AnError))
	assert.False(t, IsExpectedError(nil))
}

func TestToAppError(t *testing.T) {
	assert.Equal(t, apperror.CodeProjectNotFound, (&NotFoundError{Slug: "s"}).ToAppError().Code())
	assert.Equal(t, apperror.CodeProjectAlreadyExists, (&AlreadyExistsError{Slug: "s"}).ToAppError().Code())
	assert.Equal(t, apperror.CodeProjectInvalidSlug, (&InvalidSlugError{}).ToAppError().Code())
	assert.Equal(t, apperror.CodeProjectInvalidName, (&InvalidNameError{}).ToAppError().Code())
}

func TestValidateSlugReasons(t *testing.T) {
	for in, want := range map[string]SlugReason{"ab": SlugLength, "Not_A_Slug": SlugFormat} {
		e, ok := errors.AsType[*InvalidSlugError](ValidateSlug(in))
		require.True(t, ok, in)
		assert.Equal(t, want, e.Reason, in)
	}
	assert.Equal(t,
		map[string]string{"slug": "ab", "reason": "length", "min": "3", "max": "64"},
		(&InvalidSlugError{Slug: "ab", Reason: SlugLength}).ToAppError().Meta())
	assert.Equal(t,
		map[string]string{"reason": "length", "min": "1", "max": "200"},
		(&InvalidNameError{Reason: NameLength}).ToAppError().Meta())
	assert.Equal(t, map[string]string{"slug": "taken"}, (&AlreadyExistsError{Slug: "taken"}).ToAppError().Meta())
}

func TestIssuerErrors(t *testing.T) {
	cases := []struct {
		err    error
		is     func(error) bool
		code   apperror.Code
		meta   map[string]string
		reason string
	}{
		{&IssuerHandleTakenError{Handle: "acme-web"}, IsIssuerHandleTakenError, apperror.CodeProjectIssuerHandleTaken, map[string]string{"issuer_handle": "acme-web"}, ""},
		{&InvalidIssuerHandleError{Handle: "system", Reason: IssuerHandleReserved}, IsInvalidIssuerHandleError, apperror.CodeProjectInvalidIssuerHandle,
			map[string]string{"issuer_handle": "system", "min": "3", "max": "40"}, "reserved"},
		{&IssuerHandleLockedError{}, IsIssuerHandleLockedError, apperror.CodeProjectIssuerHandleLocked, nil, ""},
		{&InvalidLogoError{Reason: LogoSize}, IsInvalidLogoError, apperror.CodeProjectInvalidLogo, map[string]string{"max": "256"}, "size"},
		{&TermsNotSetError{}, IsTermsNotSetError, apperror.CodeProjectTermsNotSet, nil, ""},
	}
	for _, c := range cases {
		assert.NotEmpty(t, c.err.Error())
		assert.True(t, c.is(c.err))
		assert.False(t, c.is(errors.New("x")))
		assert.True(t, IsExpectedError(c.err))
		ae, ok := apperror.AsAppError(c.err)
		require.True(t, ok)
		assert.Equal(t, c.code, ae.Code())
		for k, v := range c.meta {
			assert.Equal(t, v, ae.Meta()[k], k)
		}
		if c.reason != "" {
			assert.Equal(t, c.reason, ae.Meta()["reason"])
		}
	}
}
