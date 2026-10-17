package config

import (
	"net/url"
	"regexp"
	"time"
)

// StorageDriver selects where uploaded files live.
type StorageDriver string

// The storage drivers: s3 is any S3-compatible bucket, memory is for tests and development only.
const (
	StorageDriverS3     StorageDriver = "s3"
	StorageDriverMemory StorageDriver = "memory"
)

// StorageImageTransform selects how resized image URLs are built.
type StorageImageTransform string

// The image transforms: none serves the original, cloudflare builds /cdn-cgi/image/ URLs.
const (
	StorageImageTransformNone       StorageImageTransform = "none"
	StorageImageTransformCloudflare StorageImageTransform = "cloudflare"
)

// StoragePrivateSigner selects how private files get a short-lived URL.
type StoragePrivateSigner string

// StoragePrivateSignerPresign signs a GET on the S3 endpoint.
const StoragePrivateSignerPresign StoragePrivateSigner = "presign"

const maxPresignTTL = 7 * 24 * time.Hour

// StorageConfig configures file uploads; an empty driver turns them off.
type StorageConfig struct {
	Driver StorageDriver `yaml:"driver" mapstructure:"driver" validate:"omitempty,oneof=s3 memory"`
	// App is the second key segment, so several apps can share one bucket.
	App string `yaml:"app" mapstructure:"app"`
	// PublicBaseURL serves public files: the bucket's custom domain, e.g. https://files.altalune.id.
	PublicBaseURL  string                `yaml:"publicBaseURL"  mapstructure:"publicBaseURL"`
	ImageTransform StorageImageTransform `yaml:"imageTransform" mapstructure:"imageTransform" validate:"omitempty,oneof=none cloudflare"`
	Upload         StorageUploadConfig   `yaml:"upload"         mapstructure:"upload"`
	Private        StoragePrivateConfig  `yaml:"private"        mapstructure:"private"`
}

// StorageUploadConfig bounds an upload: how long its URL is valid and how long an unfinished one is kept.
type StorageUploadConfig struct {
	URLTTL     time.Duration `yaml:"urlTTL"     mapstructure:"urlTTL"`
	PendingTTL time.Duration `yaml:"pendingTTL" mapstructure:"pendingTTL"`
}

// StoragePrivateConfig configures the short-lived URLs of private files.
type StoragePrivateConfig struct {
	Signer StoragePrivateSigner `yaml:"signer" mapstructure:"signer" validate:"omitempty,oneof=presign"`
	URLTTL time.Duration        `yaml:"urlTTL" mapstructure:"urlTTL"`
}

//nolint:gochecknoglobals // compiled once, read-only.
var storageAppRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

func (c *Config) validateStorage() error {
	s := c.Storage
	if s.Driver == "" {
		return nil
	}
	if s.Driver == StorageDriverMemory && c.Env.IsProduction() {
		return &InvalidSettingError{Key: "storage.driver", Reason: "memory is for tests and development only"}
	}
	if !storageAppRE.MatchString(s.App) {
		return &InvalidSettingError{Key: "storage.app", Reason: "must be 1-32 lowercase letters, digits or -"}
	}
	if s.PublicBaseURL == "" {
		return &MissingSettingError{Keys: "storage.publicBaseURL", Reason: "storage.driver is set"}
	}
	if err := validatePublicBaseURL(s.PublicBaseURL, c.Env.IsProduction()); err != nil {
		return err
	}
	switch {
	case s.Upload.URLTTL < time.Second || s.Upload.URLTTL > maxPresignTTL:
		return &InvalidSettingError{Key: "storage.upload.urlTTL", Reason: "must be between 1s and 168h"}
	case s.Upload.PendingTTL < s.Upload.URLTTL:
		return &InvalidSettingError{Key: "storage.upload.pendingTTL", Reason: "must be at least storage.upload.urlTTL"}
	case s.Private.URLTTL < time.Second || s.Private.URLTTL > maxPresignTTL:
		return &InvalidSettingError{Key: "storage.private.urlTTL", Reason: "must be between 1s and 168h"}
	}
	return nil
}

func validatePublicBaseURL(raw string, production bool) error {
	u, err := url.Parse(raw)
	switch {
	case err != nil, u.Host == "", u.Scheme != "https" && (production || u.Scheme != "http"):
		reason := "must be an absolute https:// URL"
		if !production {
			reason = "must be an absolute http:// or https:// URL"
		}
		return &InvalidSettingError{Key: "storage.publicBaseURL", Reason: reason}
	case u.User != nil, u.RawQuery != "", u.Fragment != "":
		return &InvalidSettingError{Key: "storage.publicBaseURL", Reason: "must not carry credentials, a query or a fragment"}
	}
	return nil
}
