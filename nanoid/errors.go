package nanoid

import (
	"errors"
)

var ErrInvalidLength = errors.New("nanoid: length must be > 0")
