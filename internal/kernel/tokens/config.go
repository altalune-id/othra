package tokens

import "time"

// Config is one resource server at one issuer; boot maps it from the console issuer, mcp.audience and mcp.clockSkew.
type Config struct {
	Issuer    string
	Audience  string
	ClockSkew time.Duration
}
