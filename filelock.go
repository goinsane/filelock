package filelock

import (
	"context"
	"errors"
	"os"
	"sync"
	"syscall"
	"time"
)

type File struct {
	internalFile
	name      string
	closeOnce sync.Once
}

type internalFile = *os.File

func Open(name string) (*File, error) {
	return OpenFile(name, os.O_RDONLY, 0)
}

func OpenFile(name string, flag int, perm os.FileMode) (f *File, err error) {
	filesMu.Lock()
	if _, ok := files[name]; ok {
		filesMu.Unlock()
		return nil, ErrLocked
	}
	files[name] = nil
	filesMu.Unlock()
	defer func() {
		if err != nil {
			filesMu.Lock()
			delete(files, name)
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
	}
	filesMu.Lock()
	files[name] = f
	filesMu.Unlock()
	return f, nil
}

func Obtain(name string) (*File, error) {
	f, err := OpenFile(name, os.O_RDWR, 0666)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		f, err = OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
		if err != nil {
			if !errors.Is(err, os.ErrExist) {
				return nil, err
			}
			return nil, ErrLocked
		}
	}
	return f, nil
}

func Acquire(ctx context.Context, name string) (*File, error) {
	f, err := Obtain(name)
	if err != ErrLocked {
		return f, err
	}
	tkr := time.NewTicker(100 * time.Millisecond)
	defer tkr.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tkr.C:
			f, err = Obtain(name)
			if err != ErrLocked {
				return f, err
			}
		}
	}
}

func (f *File) Close() (err error) {
	err = f.internalFile.Close()
	f.closeOnce.Do(func() {
		filesMu.Lock()
		delete(files, f.name)
		filesMu.Unlock()
	})
	return
}

func (f *File) Release() (err error) {
	f.closeOnce.Do(func() {
		_ = os.Remove(f.name)
		err = f.internalFile.Close()
		filesMu.Lock()
		delete(files, f.name)
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
