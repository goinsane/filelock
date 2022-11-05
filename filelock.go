package filelock

import (
	"os"
	"sync"
	"syscall"
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

func Create(name string) (*File, error) {
	return OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
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
