package redirecturi

// Reason is the stable cause of an *Error.
type Reason string

// The causes of an *Error.
const (
	Required      Reason = "required"
	Unparsable    Reason = "unparsable"
	Fragment      Reason = "fragment"
	Wildcard      Reason = "wildcard"
	Userinfo      Reason = "userinfo"
	DotSegments   Reason = "dot_segments"
	HTTPSRequired Reason = "https_required"
	LoopbackOnly  Reason = "loopback_only"
	PortIgnored   Reason = "port_ignored"
	NotCanonical  Reason = "not_canonical"
	TooLong       Reason = "too_long"
	TooMany       Reason = "too_many"
	Duplicate     Reason = "duplicate"
)

// Error is a redirect URI that breaks a rule; Index is its row, -1 for the whole list, and Canonical is set on NotCanonical.
type Error struct {
	URI       string
	Index     int
	Reason    Reason
	Canonical string
}

func (e *Error) Error() string { return "redirecturi: " + string(e.Reason) }
