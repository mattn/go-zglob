package zglob

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/mattn/go-zglob/fastwalk"
)

var (
	envre = regexp.MustCompile(`^(\$[a-zA-Z][a-zA-Z0-9_]+|\$\([a-zA-Z][a-zA-Z0-9_]+\))$`)
	mu    sync.Mutex
)

type zenv struct {
	dirmask string
	fre     *regexp.Regexp
	pattern string
	root    string
}

func toSlash(path string) string {
	if filepath.Separator == '/' {
		return path
	}
	var buf bytes.Buffer
	cc := []rune(path)
	for i := 0; i < len(cc); i++ {
		if i < len(cc)-2 && cc[i] == '\\' && (cc[i+1] == '{' || cc[i+1] == '}') {
			buf.WriteRune(cc[i])
			buf.WriteRune(cc[i+1])
			i++
		} else if cc[i] == '\\' {
			buf.WriteRune('/')
		} else {
			buf.WriteRune(cc[i])
		}
	}
	return buf.String()
}

func New(pattern string) (*zenv, error) {
	globmask := ""
	root := ""
	for n, i := range strings.Split(toSlash(pattern), "/") {
		if root == "" && strings.ContainsAny(i, "*{") {
			if globmask == "" {
				root = "."
			} else {
				root = toSlash(globmask)
			}
		}
		if n == 0 && i == "~" {
			if runtime.GOOS == "windows" {
				i = os.Getenv("USERPROFILE")
			} else {
				i = os.Getenv("HOME")
			}
		}
		if envre.MatchString(i) {
			i = strings.Trim(strings.Trim(os.Getenv(i[1:]), "()"), `"`)
		}

		globmask = path.Join(globmask, i)
		if n == 0 {
			if runtime.GOOS == "windows" && filepath.VolumeName(i) != "" {
				globmask = i + "/"
			} else if len(globmask) == 0 {
				globmask = "/"
			}
		}
	}
	if root == "" {
		return &zenv{
			dirmask: "",
			fre:     nil,
			pattern: pattern,
			root:    "",
		}, nil
	}
	if globmask == "" {
		globmask = "."
	}
	globmask = toSlash(path.Clean(globmask))

	cc := []rune(globmask)
	var dirmask strings.Builder
	var filemask strings.Builder
	staticDir := true
	for i := 0; i < len(cc); i++ {
		if i < len(cc)-2 && cc[i] == '\\' {
			i++
			fmt.Fprintf(&filemask, "[\\x%02X]", cc[i])
			if staticDir {
				dirmask.WriteRune(cc[i])
			}
		} else if cc[i] == '*' {
			staticDir = false
			if i < len(cc)-2 && cc[i+1] == '*' && cc[i+2] == '/' {
				filemask.WriteString("(.*/)?")
				i += 2
			} else {
				filemask.WriteString("[^/]*")
			}
		} else if cc[i] == '[' { // range
			staticDir = false
			var b strings.Builder
			for j := i + 1; j < len(cc); j++ {
				if cc[j] == ']' {
					i = j
					break
				} else {
					b.WriteRune(cc[j])
				}
			}
			if pattern := b.String(); pattern != "" {
				filemask.WriteByte('[')
				filemask.WriteString(pattern)
				filemask.WriteByte(']')
				continue
			}
		} else {
			if cc[i] == '{' {
				staticDir = false
				var b strings.Builder
				for j := i + 1; j < len(cc); j++ {
					if cc[j] == ',' {
						b.WriteByte('|')
					} else if cc[j] == '}' {
						i = j
						break
					} else {
						c := cc[j]
						if c == '/' {
							b.WriteRune(c)
						} else if ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || 255 < c {
							b.WriteRune(c)
						} else {
							fmt.Fprintf(&b, "[\\x%02X]", c)
						}
					}
				}
				if pattern := b.String(); pattern != "" {
					filemask.WriteByte('(')
					filemask.WriteString(pattern)
					filemask.WriteByte(')')
					continue
				}
			} else if i < len(cc)-1 && cc[i] == '!' && cc[i+1] == '(' {
				i++
				var b strings.Builder
				for j := i + 1; j < len(cc); j++ {
					if cc[j] == ')' {
						i = j
						break
					} else {
						c := cc[j]
						fmt.Fprintf(&b, "[^\\x%02X/]*", c)
					}
				}
				if pattern := b.String(); pattern != "" {
					if dirmask.Len() == 0 {
						m := filemask.String()
						dirmask.WriteString(m)
						root = m
					}
					filemask.WriteString(pattern)
					continue
				}
			}
			c := cc[i]
			if c == '/' || ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || 255 < c {
				filemask.WriteRune(c)
			} else {
				fmt.Fprintf(&filemask, "[\\x%02X]", c)
			}
			if staticDir {
				dirmask.WriteRune(c)
			}
		}
	}
	if m := filemask.String(); len(m) > 0 && m[len(m)-1] == '/' {
		if root == "" {
			root = m
		}
		filemask.WriteString("[^/]*")
	}
	var pat string
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		pat = "^(?i:" + filemask.String() + ")$"
	} else {
		pat = "^" + filemask.String() + "$"
	}
	fre, err := regexp.Compile(pat)
	if err != nil {
		return nil, err
	}
	return &zenv{
		dirmask: path.Dir(dirmask.String()) + "/",
		fre:     fre,
		pattern: pattern,
		root:    filepath.Clean(root),
	}, nil
}

func Glob(pattern string) ([]string, error) {
	return glob(pattern, false)
}

func GlobFollowSymlinks(pattern string) ([]string, error) {
	return glob(pattern, true)
}

func glob(pattern string, followSymlinks bool) ([]string, error) {
	zenv, err := New(pattern)
	if err != nil {
		return nil, err
	}
	if zenv.root == "" {
		_, err := os.Stat(pattern)
		if err != nil {
			return nil, os.ErrNotExist
		}
		return []string{pattern}, nil
	}
	relative := !filepath.IsAbs(pattern)
	matches := []string{}

	err = fastwalk.FastWalk(zenv.root, func(path string, info os.FileMode) error {
		if zenv.root == "." && len(zenv.root) < len(path) {
			path = path[len(zenv.root)+1:]
		}
		path = filepath.ToSlash(path)

		if followSymlinks && info == os.ModeSymlink {
			followedPath, err := filepath.EvalSymlinks(path)
			if err == nil {
				fi, err := os.Lstat(followedPath)
				if err == nil && fi.IsDir() {
					return fastwalk.TraverseLink
				}
			}
		}

		if info.IsDir() {
			if path == "." || len(path) <= len(zenv.root) {
				return nil
			}
			if zenv.fre.MatchString(path) {
				mu.Lock()
				matches = append(matches, path)
				mu.Unlock()
				return nil
			}
			if len(path) < len(zenv.dirmask) && !strings.HasPrefix(zenv.dirmask, path+"/") {
				return filepath.SkipDir
			}
		}

		if zenv.fre.MatchString(path) {
			if relative && filepath.IsAbs(path) {
				path = path[len(zenv.root)+1:]
			}
			mu.Lock()
			matches = append(matches, path)
			mu.Unlock()
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return matches, nil
}

func Match(pattern, name string) (matched bool, err error) {
	zenv, err := New(pattern)
	if err != nil {
		return false, err
	}
	return zenv.Match(name), nil
}

func (z *zenv) Match(name string) bool {
	if z.root == "" {
		return z.pattern == name
	}

	name = filepath.ToSlash(name)

	if name == "." || len(name) <= len(z.root) {
		return false
	}

	if z.fre.MatchString(name) {
		return true
	}
	return false
}
