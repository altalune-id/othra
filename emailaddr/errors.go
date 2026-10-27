package emailaddr

import (
	"errors"
)

// ErrInvalid is returned for anything that is not a bare address.
var ErrInvalid = errors.New("emailaddr: invalid email address")
