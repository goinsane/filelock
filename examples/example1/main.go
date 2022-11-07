package main

import (
	"fmt"
	"os"
	"time"

	"github.com/goinsane/filelock"
)

func main() {
	name := os.TempDir() + string(os.PathSeparator) + "filelock.test"
	fmt.Println(name)
	f, err := filelock.Create(name, 0666)
	if err != nil {
		panic(err)
	}
	defer f.Release()
	time.Sleep(10 * time.Second)
}
