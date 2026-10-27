package capabilities

// Config is the deployment posture the switches derive from.
type Config struct {
	Selfhosted bool
	Cloud      bool
	BasicAuth  bool
	// EnvGenesis is true when the first operator comes from the environment.
	EnvGenesis bool
	// FileStorage is true when a storage driver is configured.
	FileStorage bool
	// Passwords is true when console password sign-in is configured on.
	Passwords bool
}
