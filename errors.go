package filelock

import (
	"errors"
)

type LockError struct{ error }

var (
	ErrLocked = errors.New("locked")
)
