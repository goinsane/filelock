// Package filelock provides locked File struct similar with os.File.
package filelock

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// File represents an open file descriptor such as os.File. But File always has Posix write lock.
type File struct {
	internalFile
	name      string
	absPath   string
	closeOnce sync.Once
}

type internalFile = *os.File

// Open opens the named file with OpenFile for reading by using OpenFile.
// When an error occurs, Open returns the error from OpenFile.
// If successful, methods on the returned file can be used for reading;
// the associated file descriptor has mode os.O_RDONLY.
func Open(name string) (*File, error) {
	return OpenFile(name, os.O_RDONLY, 0)
}

// Create creates or truncates the named file by using OpenFile.
// If the file already exists, it is truncated.
// If the file does not exist, it is created with mode 0666 (before umask).
// When an error occurs, Create returns the error from OpenFile.
// If LockError occurs, Create will not delete created file.
// If successful, methods on the returned File can be used for I/O;
// the associated file descriptor has mode os.O_RDWR.
func Create(name string) (*File, error) {
	return OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

// OpenFile opens the named file with specified flag (O_RDONLY etc.) by using os.OpenFile, after locks with Posix lock.
// If the file does not exist and the os.O_CREATE flag is passed, it is created with mode perm (before umask).
// When an error occurs, OpenFile returns the os.OpenFile error or LockError.
// If the file created with os.O_CREATE flag and LockError occurs, OpenFile will not delete created file.
// If successful, methods on the returned File can be used for I/O.
func OpenFile(name string, flag int, perm os.FileMode) (f *File, err error) {
	absPath, err := filepath.Abs(name)
	if err != nil {
		return nil, fmt.Errorf("unable to get abs path: %w", err)
	}

	filesMu.Lock()
	if _, ok := files[absPath]; ok {
		filesMu.Unlock()
		return nil, ErrLocked
	}
	files[absPath] = nil
	filesMu.Unlock()
	defer func() {
		if err != nil {
			filesMu.Lock()
			delete(files, absPath)
			filesMu.Unlock()
		}
	}()

	f2, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = f2.Close()
		}
	}()

	ok, err := posixLock(f2.Fd())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrLocked
	}

	f = &File{
		internalFile: f2,
		name:         name,
		absPath:      absPath,
	}

	filesMu.Lock()
	files[absPath] = f
	filesMu.Unlock()

	return f, nil
}

// Acquire tries to obtain file lock given period.
func Acquire(ctx context.Context, name string, perm os.FileMode, period time.Duration) (*File, error) {
	f, err := Create(name, perm)
	if err != ErrLocked {
		return f, err
	}
	tkr := time.NewTicker(period)
	defer tkr.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tkr.C:
			f, err = Create(name, perm)
			if err != ErrLocked {
				return f, err
			}
		}
	}
}

// Close closes and unlocks the File.
func (f *File) Close() (err error) {
	err = f.internalFile.Close()
	f.closeOnce.Do(func() {
		filesMu.Lock()
		delete(files, f.absPath)
		filesMu.Unlock()
	})
	return
}

// Release deletes, closes and unlocks the File.
func (f *File) Release() (err error) {
	f.closeOnce.Do(func() {
		_ = os.Remove(f.name)
		err = f.internalFile.Close()
		filesMu.Lock()
		delete(files, f.absPath)
		filesMu.Unlock()
	})
	return
}

var files = make(map[string]*File)
var filesMu sync.Mutex

func posixLock(fd uintptr) (ok bool, err error) {
	err = syscall.FcntlFlock(fd, syscall.F_SETLK, &syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Whence: 0,
		Start:  0,
		Len:    0,
	})
	if err != nil {
		if err != syscall.EWOULDBLOCK {
			return false, &LockError{err}
		}
		return false, nil
	}
	return true, nil
}
