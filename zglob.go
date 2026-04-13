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
	cache sync.Map
)

type zenv struct {
	dirmask string
	matcher *globMatcher
	pattern string
	root    string
}

type globOpKind uint8

const (
	opLiteral globOpKind = iota
	opStar
	opDoubleStarSlash
	opCharClass
	opAlternatives
	opNotCharStar
)

type globOp struct {
	kind         globOpKind
	text         string
	alternatives []string
	charClass    *charClass
	ch           byte
}

type globMatcher struct {
	ops             []globOp
	caseInsensitive bool
}

type charClass struct {
	negated bool
	items   []charClassItem
}

type charClassItem struct {
	lo byte
	hi byte
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
	if cached, ok := cache.Load(pattern); ok {
		z := cached.(*zenv)
		return z, nil
	}

	z, err := newEnv(pattern)
	if err != nil {
		return nil, err
	}
	actual, _ := cache.LoadOrStore(pattern, z)
	return actual.(*zenv), nil
}

func newEnv(pattern string) (*zenv, error) {
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
			matcher: nil,
			pattern: pattern,
			root:    "",
		}, nil
	}
	if globmask == "" {
		globmask = "."
	}
	globmask = toSlash(path.Clean(globmask))
	matcher, dirmask, err := compileGlob(globmask)
	if err != nil {
		return nil, err
	}
	return &zenv{
		dirmask: path.Dir(dirmask) + "/",
		matcher: matcher,
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
	var mu sync.Mutex

	err = fastwalk.FastWalk(zenv.root, func(path string, info os.FileMode) error {
		if zenv.root == "." && len(zenv.root) < len(path) {
			path = path[len(zenv.root)+1:]
		}
		path = walkPathToSlash(path)

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
			if zenv.matcher.Match(path) {
				mu.Lock()
				matches = append(matches, path)
				mu.Unlock()
				return nil
			}
			if len(path) < len(zenv.dirmask) && !strings.HasPrefix(zenv.dirmask, path+"/") {
				return filepath.SkipDir
			}
		}

		if zenv.matcher.Match(path) {
			if relative && zenv.root != "." && filepath.IsAbs(path) {
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

	return z.matcher.Match(name)
}

func walkPathToSlash(path string) string {
	if filepath.Separator == '/' {
		return path
	}
	return filepath.ToSlash(path)
}

func compileGlob(pattern string) (*globMatcher, string, error) {
	var (
		ops       []globOp
		literal   strings.Builder
		dirmask   strings.Builder
		staticDir = true
	)
	flushLiteral := func() {
		if literal.Len() == 0 {
			return
		}
		ops = append(ops, globOp{kind: opLiteral, text: literal.String()})
		literal.Reset()
	}

	for i := 0; i < len(pattern); i++ {
		switch {
		case i < len(pattern)-1 && pattern[i] == '\\':
			i++
			literal.WriteByte(pattern[i])
			if staticDir {
				dirmask.WriteByte(pattern[i])
			}
		case i < len(pattern)-2 && pattern[i] == '*' && pattern[i+1] == '*' && pattern[i+2] == '/':
			flushLiteral()
			ops = append(ops, globOp{kind: opDoubleStarSlash})
			staticDir = false
			i += 2
		case pattern[i] == '*':
			flushLiteral()
			ops = append(ops, globOp{kind: opStar})
			staticDir = false
		case pattern[i] == '[':
			cls, next, ok, err := parseCharClass(pattern, i)
			if err != nil {
				return nil, "", err
			}
			if !ok {
				literal.WriteByte(pattern[i])
				if staticDir {
					dirmask.WriteByte(pattern[i])
				}
				continue
			}
			flushLiteral()
			ops = append(ops, globOp{kind: opCharClass, charClass: cls})
			staticDir = false
			i = next
		case pattern[i] == '{':
			alts, next, ok := parseAlternatives(pattern, i)
			if !ok {
				literal.WriteByte(pattern[i])
				if staticDir {
					dirmask.WriteByte(pattern[i])
				}
				continue
			}
			flushLiteral()
			ops = append(ops, globOp{kind: opAlternatives, alternatives: alts})
			staticDir = false
			i = next
		case i < len(pattern)-1 && pattern[i] == '!' && pattern[i+1] == '(':
			chars, next, ok := parseNotChars(pattern, i)
			if !ok {
				literal.WriteByte(pattern[i])
				if staticDir {
					dirmask.WriteByte(pattern[i])
				}
				continue
			}
			flushLiteral()
			for _, ch := range chars {
				ops = append(ops, globOp{kind: opNotCharStar, ch: ch})
			}
			staticDir = false
			i = next
		default:
			literal.WriteByte(pattern[i])
			if staticDir {
				dirmask.WriteByte(pattern[i])
			}
		}
	}

	flushLiteral()
	if len(pattern) > 0 && pattern[len(pattern)-1] == '/' {
		ops = append(ops, globOp{kind: opStar})
	}
	return &globMatcher{
		ops:             ops,
		caseInsensitive: runtime.GOOS == "windows" || runtime.GOOS == "darwin",
	}, dirmask.String(), nil
}

func parseCharClass(pattern string, start int) (*charClass, int, bool, error) {
	end := start + 1
	for end < len(pattern) && pattern[end] != ']' {
		end++
	}
	if end >= len(pattern) {
		return nil, 0, false, nil
	}
	content := pattern[start+1 : end]
	if len(content) == 0 {
		return nil, 0, false, nil
	}

	cls := &charClass{}
	if content[0] == '^' {
		cls.negated = true
		content = content[1:]
	}
	for i := 0; i < len(content); i++ {
		current := content[i]
		if current == '\\' && i+1 < len(content) {
			i++
			current = content[i]
		}
		if i+2 < len(content) && content[i+1] == '-' {
			hi := content[i+2]
			if hi == '\\' && i+3 < len(content) {
				i += 2
				hi = content[i+1]
			}
			if current > hi {
				return nil, 0, false, fmt.Errorf("error parsing regexp: invalid character class range: %c-%c", current, hi)
			}
			cls.items = append(cls.items, charClassItem{lo: current, hi: hi})
			i += 2
			continue
		}
		cls.items = append(cls.items, charClassItem{lo: current, hi: current})
	}
	return cls, end, true, nil
}

func parseAlternatives(pattern string, start int) ([]string, int, bool) {
	var (
		alternatives []string
		current      strings.Builder
	)
	for i := start + 1; i < len(pattern); i++ {
		switch pattern[i] {
		case ',':
			alternatives = append(alternatives, current.String())
			current.Reset()
		case '}':
			alternatives = append(alternatives, current.String())
			if len(alternatives) == 0 {
				return nil, 0, false
			}
			return alternatives, i, true
		default:
			current.WriteByte(pattern[i])
		}
	}
	return nil, 0, false
}

func parseNotChars(pattern string, start int) ([]byte, int, bool) {
	var chars []byte
	for i := start + 2; i < len(pattern); i++ {
		if pattern[i] == ')' {
			return chars, i, true
		}
		chars = append(chars, pattern[i])
	}
	return nil, 0, false
}

func (m *globMatcher) Match(name string) bool {
	if m == nil {
		return false
	}
	return m.match(name, 0, 0)
}

func (m *globMatcher) match(name string, opIndex, nameIndex int) bool {
	if opIndex == len(m.ops) {
		return nameIndex == len(name)
	}
	op := m.ops[opIndex]
	switch op.kind {
	case opLiteral:
		if m.hasPrefix(name[nameIndex:], op.text) {
			return m.match(name, opIndex+1, nameIndex+len(op.text))
		}
	case opStar:
		return m.matchStar(name, opIndex, nameIndex, 0)
	case opDoubleStarSlash:
		return m.matchDoubleStarSlash(name, opIndex, nameIndex)
	case opCharClass:
		if nameIndex < len(name) && name[nameIndex] != '/' && op.charClass.match(name[nameIndex], m.caseInsensitive) {
			return m.match(name, opIndex+1, nameIndex+1)
		}
	case opAlternatives:
		for _, alt := range op.alternatives {
			if m.hasPrefix(name[nameIndex:], alt) && m.match(name, opIndex+1, nameIndex+len(alt)) {
				return true
			}
		}
	case opNotCharStar:
		return m.matchStar(name, opIndex, nameIndex, op.ch)
	}
	return false
}

func (m *globMatcher) matchStar(name string, opIndex, nameIndex int, stop byte) bool {
	for i := nameIndex; ; i++ {
		if m.match(name, opIndex+1, i) {
			return true
		}
		if i >= len(name) || name[i] == '/' {
			return false
		}
		if stop != 0 && sameByte(name[i], stop, m.caseInsensitive) {
			return false
		}
	}
}

func (m *globMatcher) matchDoubleStarSlash(name string, opIndex, nameIndex int) bool {
	if m.match(name, opIndex+1, nameIndex) {
		return true
	}
	for i := nameIndex; i < len(name); i++ {
		if name[i] == '/' && m.match(name, opIndex+1, i+1) {
			return true
		}
	}
	return false
}

func (m *globMatcher) hasPrefix(name, prefix string) bool {
	if len(prefix) > len(name) {
		return false
	}
	if !m.caseInsensitive {
		return strings.HasPrefix(name, prefix)
	}
	for i := 0; i < len(prefix); i++ {
		if !sameByte(name[i], prefix[i], m.caseInsensitive) {
			return false
		}
	}
	return true
}

func (c *charClass) match(r byte, caseInsensitive bool) bool {
	matched := false
	for _, item := range c.items {
		if byteInRange(r, item.lo, item.hi, caseInsensitive) {
			matched = true
			break
		}
	}
	if c.negated {
		return !matched
	}
	return matched
}

func byteInRange(r, lo, hi byte, caseInsensitive bool) bool {
	if !caseInsensitive {
		return lo <= r && r <= hi
	}
	r = lowerASCII(r)
	lo = lowerASCII(lo)
	hi = lowerASCII(hi)
	return lo <= r && r <= hi
}

func sameByte(a, b byte, caseInsensitive bool) bool {
	if !caseInsensitive {
		return a == b
	}
	return lowerASCII(a) == lowerASCII(b)
}

func lowerASCII(b byte) byte {
	if 'A' <= b && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}
