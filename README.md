# go-zglob

[![Build Status](https://github.com/mattn/go-zglob/actions/workflows/go.yml/badge.svg)](https://github.com/mattn/go-zglob/actions/workflows/go.yml)

**zglob** is a Go package for pattern matching file paths with support for advanced glob patterns like `**` for recursive directory matching. It is similar to Unix shell-style globbing but offers more flexibility.

## Features

- Glob pattern matching for file paths.
- Recursive directory searching with `**`.
- Supports character ranges like `[a-z]`, brace expansion such as `{foo,bar}`, and advanced patterns.
- Handles symlink traversal with options to follow them.

## Usage

### Using the Library

#### Basic Glob

```go
package main

import (
    "fmt"
    "github.com/mattn/go-zglob"
)

func main() {
    matches, err := zglob.Glob("./foo/**/*.txt")
    if err != nil {
        fmt.Println("Error:", err)
    }
    fmt.Println("Matched files:", matches)
}
```

This example will recursively find all `.txt` files in the `foo` directory and its subdirectories.

#### Matching with Character Ranges

```go
matches, err := zglob.Glob("./foo/b[a-z]*")
if err != nil {
    fmt.Println("Error:", err)
}
fmt.Println("Matched files:", matches)
```

This will match files in the `foo` directory that start with the letter `b` followed by any character between `a` and `z`.

#### Following Symlinks

```go
matches, err := zglob.GlobFollowSymlinks("./foo/**")
if err != nil {
    fmt.Println("Error:", err)
}
fmt.Println("Matched files:", matches)
```

This example will find all files and directories within `foo`, following symbolic links as well.

### Using the Command Line Tool

Use the `zglob` command in your terminal for file matching:

```bash
zglob './foo/**/*.txt'
```

This will search for `.txt` files in the `foo` directory recursively, similar to the library usage.

## Installation

For using library:

```console
$ go get github.com/mattn/go-zglob
```

For using command:

```console
$ go install github.com/mattn/go-zglob/cmd/zglob@latest
```

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a mattn)
