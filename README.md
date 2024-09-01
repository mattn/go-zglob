# go-zglob

[![Build Status](https://github.com/mattn/go-zglob/actions/workflows/go.yml/badge.svg)](https://github.com/mattn/go-zglob/actions/workflows/go.yml)

zglob

## Usage

```go
matches, err := zglob.Glob(`./foo/b*/**/z*.txt`)
```

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
