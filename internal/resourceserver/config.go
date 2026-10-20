package resourceserver

import "time"

// Config bounds verification and trust; boot maps resourceServers.* and the deployment mode into it.
type Config struct {
	Interval       time.Duration
	StaleAfter     time.Duration
	TrustAvailable bool
}
