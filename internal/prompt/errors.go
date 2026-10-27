package prompt

import (
	"errors"
)

// ErrCancelled reports a yes/no question answered with anything but yes, including no answer at all.
var ErrCancelled = errors.New("cancelled")

// ErrMismatch reports that the two answers to a confirmed secret differ.
var ErrMismatch = errors.New("the passwords do not match")
