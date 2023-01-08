# go-zglob

[![Build Status](https://travis-ci.org/mattn/go-zglob.svg)](https://travis-ci.org/mattn/go-zglob)

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
