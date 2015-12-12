package main

import (
	"fmt"
	"github.com/mattn/go-zglob"
	"os"
)

func main() {
	for _, arg := range os.Args {
		matches, err := zglob.Glob(arg)
		if err != nil {
			continue
		}
		for _, m := range matches {
			fmt.Println(m)
		}
	}
}
