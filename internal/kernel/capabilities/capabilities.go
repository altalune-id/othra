// Package capabilities derives the feature switches surfaces check instead of reading the deployment mode.
package capabilities

// Capabilities is the read-only switch snapshot derived from a Config.
type Capabilities struct {
	// LocalLogin is the console's own sign-in and sessions: a password where allowed, and Continue with Othra.
	LocalLogin   bool
	BasicAuth    bool
	OrgCreation  bool
	EnvGenesis   bool
	PersonalKeys bool
	// OperatorOrgSupport lets staff (an operator who is a person) manage, as an admin, the members of an org they do not belong to.
	OperatorOrgSupport bool
	// FileUploads lets surfaces offer uploads such as a project logo.
	FileUploads bool
	// Passwords is console password sign-in, beside Continue with Othra; off in cloud.
	Passwords bool
}

// From derives Capabilities from the deployment posture.
func From(c Config) Capabilities {
	local := (c.Selfhosted || c.Cloud) && !c.BasicAuth
	return Capabilities{
		LocalLogin:         local,
		BasicAuth:          c.Selfhosted && c.BasicAuth,
		OrgCreation:        c.Cloud,
		EnvGenesis:         local && c.EnvGenesis,
		PersonalKeys:       !c.BasicAuth,
		OperatorOrgSupport: c.Cloud,
		FileUploads:        c.FileStorage,
		Passwords:          local && c.Passwords,
	}
}
