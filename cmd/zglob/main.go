package main

import (
	"fmt"
	"github.com/mattn/go-zglob"
	"os"
)

func main() {
	for _, arg := range os.Args[1:] {
		matches, err := zglob.Glob(arg)
		if err != nil {
			continue
		}
		for _, m := range matches {
			fmt.Println(m)
		}
	}
}
