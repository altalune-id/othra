package storage

import (
	"errors"
	"strconv"

	"github.com/altalune-id/othra/apperror"
)

// TypeNotAllowedError signals a content type the purpose does not accept.
type TypeNotAllowedError struct {
	Purpose     string
	ContentType string
}

func (e *TypeNotAllowedError) Error() string {
	return "storage: " + e.Purpose + " does not accept " + e.ContentType
}

// IsTypeNotAllowedError reports whether err's tree contains a *TypeNotAllowedError.
func IsTypeNotAllowedError(err error) bool {
	_, ok := errors.AsType[*TypeNotAllowedError](err)
	return ok
}

// TooLargeError signals a declared size above the purpose's cap.
type TooLargeError struct {
	Purpose  string
	MaxBytes int64
}

func (e *TooLargeError) Error() string {
	return "storage: " + e.Purpose + " accepts at most " + strconv.FormatInt(e.MaxBytes, 10) + " bytes"
}

// IsTooLargeError reports whether err's tree contains a *TooLargeError.
func IsTooLargeError(err error) bool {
	_, ok := errors.AsType[*TooLargeError](err)
	return ok
}

// UploadNotFoundError signals an upload that does not exist for this owner, is no longer pending, or has expired.
type UploadNotFoundError struct{ PublicID string }

func (e *UploadNotFoundError) Error() string {
	return "storage: upload " + e.PublicID + " not found or expired"
}

// IsUploadNotFoundError reports whether err's tree contains an *UploadNotFoundError.
func IsUploadNotFoundError(err error) bool {
	_, ok := errors.AsType[*UploadNotFoundError](err)
	return ok
}

// ToAppError maps UploadNotFoundError to STO001.
func (e *UploadNotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeStorageUploadNotFound, "Upload not found or expired").WithCause(e)
}

// MismatchReason is the stable cause of an UploadMismatchError, sent to clients as meta.reason.
type MismatchReason string

// The causes of an UploadMismatchError.
const (
	MismatchMissing MismatchReason = "missing"
	MismatchSize    MismatchReason = "size"
	MismatchType    MismatchReason = "type"
	MismatchContent MismatchReason = "content"
)

// UploadMismatchError signals a stored object that is absent or differs from what was declared.
type UploadMismatchError struct {
	PublicID string
	Reason   MismatchReason
}

func (e *UploadMismatchError) Error() string {
	return "storage: upload " + e.PublicID + " does not match what was declared: " + string(e.Reason)
}

// IsUploadMismatchError reports whether err's tree contains an *UploadMismatchError.
func IsUploadMismatchError(err error) bool {
	_, ok := errors.AsType[*UploadMismatchError](err)
	return ok
}

// ToAppError maps UploadMismatchError to STO002 with its reason.
func (e *UploadMismatchError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeStorageUploadMismatch, "The uploaded file does not match what was declared").
		WithMeta(map[string]string{"reason": string(e.Reason)}).WithCause(e)
}

// NotConfiguredError signals that this server has no file storage.
type NotConfiguredError struct{}

func (*NotConfiguredError) Error() string { return "storage: file storage is not configured" }

// IsNotConfiguredError reports whether err's tree contains a *NotConfiguredError.
func IsNotConfiguredError(err error) bool {
	_, ok := errors.AsType[*NotConfiguredError](err)
	return ok
}

// ToAppError maps NotConfiguredError to STO003.
func (*NotConfiguredError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeStorageNotConfigured, "File storage is not configured on this server").WithCause(&NotConfiguredError{})
}

// ObjectNotFoundError signals that storage holds no object under the key.
type ObjectNotFoundError struct{ Key string }

func (e *ObjectNotFoundError) Error() string { return "storage: no object at " + e.Key }

// IsObjectNotFoundError reports whether err's tree contains an *ObjectNotFoundError.
func IsObjectNotFoundError(err error) bool {
	_, ok := errors.AsType[*ObjectNotFoundError](err)
	return ok
}

// IsExpectedError reports whether err is a typed storage error a service returns as-is.
func IsExpectedError(err error) bool {
	return IsTypeNotAllowedError(err) || IsTooLargeError(err) || IsUploadNotFoundError(err) ||
		IsUploadMismatchError(err) || IsNotConfiguredError(err)
}
