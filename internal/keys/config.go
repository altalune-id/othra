package keys

import "time"

// Config bounds key generation and rotation; boot maps keys.* into it.
type Config struct {
	RSABits    int
	PrePublish time.Duration
	Overlap    time.Duration
}
