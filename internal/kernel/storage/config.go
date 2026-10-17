package storage

import "time"

// ImageTransform selects how resized image URLs are built.
type ImageTransform string

// The image transforms: none serves the original, cloudflare builds /cdn-cgi/image/ URLs.
const (
	ImageTransformNone       ImageTransform = "none"
	ImageTransformCloudflare ImageTransform = "cloudflare"
)

// Config is the feature's settings; boot maps storage.* into it.
type Config struct {
	App            string
	PublicBaseURL  string
	ImageTransform ImageTransform
	UploadURLTTL   time.Duration
	PendingTTL     time.Duration
	PrivateURLTTL  time.Duration
}
