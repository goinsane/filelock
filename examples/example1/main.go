package main

import (
	"os"
	"time"

	"github.com/goinsane/filelock"
)

func main() {
	f, err := filelock.Obtain(os.TempDir() + string(os.PathSeparator) + "filelock.test")
	if err != nil {
		panic(err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer f.Release()
	time.Sleep(10 * time.Second)
}
