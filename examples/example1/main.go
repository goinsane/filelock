package main

import (
	"os"
	"time"

	"github.com/goinsane/filelock"
)

func main() {
	f, err := filelock.Create(os.TempDir()+string(os.PathSeparator)+"filelock.test", 0666)
	if err != nil {
		panic(err)
	}
	defer f.Release()
	time.Sleep(10 * time.Second)
}
