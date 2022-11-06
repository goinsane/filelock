package filelock

import (
	"errors"
)

type LockError struct{ error }

var (
	ErrLocked = LockError{errors.New("locked")}
)
