package zglob

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func check(got []string, expected []string) bool {
	sort.Strings(got)
	sort.Strings(expected)
	return reflect.DeepEqual(expected, got)
}

type testZGlob struct {
	pattern  string
	expected []string
	err      string
}

var testGlobs = []testZGlob{
	{`fo*`, []string{`foo`}, ""},
	{`foo`, []string{`foo`}, ""},
	{`foo/*`, []string{`foo/bar`, `foo/baz`}, ""},
	{`foo/b[a]*`, []string{`foo/bar`, `foo/baz`}, ""},
	{`foo/b[a][r]*`, []string{`foo/bar`}, ""},
	{`foo/b[a-z]*`, []string{`foo/bar`, `foo/baz`}, ""},
	{`foo/b[c-z]*`, []string{}, ""},
	{`foo/b[z-c]*`, []string{}, "error parsing regexp"},
	{`foo/**`, []string{`foo/bar`, `foo/baz`}, ""},
	{`f*o/**`, []string{`foo/bar`, `foo/baz`}, ""},
	{`*oo/**`, []string{`foo/bar`, `foo/baz`, `hoo/bar`}, ""},
	{`*oo/b*`, []string{`foo/bar`, `foo/baz`, `hoo/bar`}, ""},
	{`*oo/bar`, []string{`foo/bar`, `hoo/bar`}, ""},
	{`*oo/*z`, []string{`foo/baz`}, ""},
	{`foo/**/*`, []string{`foo/bar`, `foo/bar/baz`, `foo/bar/baz.txt`, `foo/bar/baz/noo.txt`, `foo/baz`}, ""},
	{`*oo/**/*`, []string{`foo/bar`, `foo/bar/baz`, `foo/bar/baz.txt`, `foo/bar/baz/noo.txt`, `foo/baz`, `hoo/bar`}, ""},
	{`*oo/*.txt`, []string{}, ""},
	{`*oo/*/*.txt`, []string{`foo/bar/baz.txt`}, ""},
	{`*oo/**/*.txt`, []string{`foo/bar/baz.txt`, `foo/bar/baz/noo.txt`}, ""},
	{`doo`, nil, "file does not exist"},
	{`./f*`, []string{`foo`}, ""},
	{`**/bar/**/*.txt`, []string{`foo/bar/baz.txt`, `foo/bar/baz/noo.txt`}, ""},
	{`**/bar/**/*.{jpg,png}`, []string{`zzz/bar/baz/joo.png`, `zzz/bar/baz/zoo.jpg`}, ""},
	{`zzz/bar/baz/zoo.{jpg,png}`, []string{`zzz/bar/baz/zoo.jpg`}, ""},
	{`zzz/bar/{baz,z}/zoo.jpg`, []string{`zzz/bar/baz/zoo.jpg`}, ""},
	{`zzz/nar/\{noo,x\}/joo.png`, []string{`zzz/nar/{noo,x}/joo.png`}, ""},
}

func fatalIf(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func setup() (string, string) {
	tmpdir, err := ioutil.TempDir("", "zglob")
	fatalIf(err)

	fatalIf(os.MkdirAll(filepath.Join(tmpdir, "foo/baz"), 0755))
	fatalIf(os.MkdirAll(filepath.Join(tmpdir, "foo/bar"), 0755))
	fatalIf(ioutil.WriteFile(filepath.Join(tmpdir, "foo/bar/baz.txt"), []byte{}, 0644))
	fatalIf(os.MkdirAll(filepath.Join(tmpdir, "foo/bar/baz"), 0755))
	fatalIf(ioutil.WriteFile(filepath.Join(tmpdir, "foo/bar/baz/noo.txt"), []byte{}, 0644))
	fatalIf(os.MkdirAll(filepath.Join(tmpdir, "hoo/bar"), 0755))
	fatalIf(ioutil.WriteFile(filepath.Join(tmpdir, "foo/bar/baz.txt"), []byte{}, 0644))
	fatalIf(os.MkdirAll(filepath.Join(tmpdir, "zzz/bar/baz"), 0755))
	fatalIf(ioutil.WriteFile(filepath.Join(tmpdir, "zzz/bar/baz/zoo.jpg"), []byte{}, 0644))
	fatalIf(ioutil.WriteFile(filepath.Join(tmpdir, "zzz/bar/baz/joo.png"), []byte{}, 0644))
	fatalIf(os.MkdirAll(filepath.Join(tmpdir, "zzz/nar/{noo,x}"), 0755))
	fatalIf(ioutil.WriteFile(filepath.Join(tmpdir, "zzz/nar/{noo,x}/joo.png"), []byte{}, 0644))

	curdir, err := os.Getwd()
	fatalIf(err)
	fatalIf(os.Chdir(tmpdir))

	return tmpdir, curdir
}

func TestGlob(t *testing.T) {
	tmpdir, savedCwd := setup()
	defer os.RemoveAll(tmpdir)
	defer os.Chdir(savedCwd)

	tmpdir = "."
	for _, test := range testGlobs {
		expected := make([]string, len(test.expected))
		for i, e := range test.expected {
			expected[i] = e
		}
		got, err := Glob(test.pattern)
		if err != nil {
			if !strings.Contains(err.Error(), test.err) {
				t.Error(err)
			}
			continue
		}
		if !check(expected, got) {
			t.Errorf(`zglob failed: pattern %q(%q): expected %v but got %v`, test.pattern, tmpdir, expected, got)
		}
	}
}

func TestGlobAbs(t *testing.T) {
	tmpdir, savedCwd := setup()
	defer os.RemoveAll(tmpdir)
	defer os.Chdir(savedCwd)

	for _, test := range testGlobs {
		pattern := toSlash(path.Join(tmpdir, test.pattern))
		expected := make([]string, len(test.expected))
		for i, e := range test.expected {
			expected[i] = filepath.ToSlash(filepath.Join(tmpdir, e))
		}
		got, err := Glob(pattern)
		if err != nil {
			if !strings.Contains(err.Error(), test.err) {
				t.Error(err)
			}
			continue
		}
		if !check(expected, got) {
			t.Errorf(`zglob failed: pattern %q(%q): expected %v but got %v`, pattern, tmpdir, expected, got)
		}
	}
}

func TestMatch(t *testing.T) {
	for _, test := range testGlobs {
		for _, f := range test.expected {
			got, err := Match(test.pattern, f)
			if err != nil {
				t.Error(err)
				continue
			}
			if !got {
				t.Errorf("%q should match with %q", f, test.pattern)
			}
		}
	}
}

func TestFollowSymlinks(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "zglob")
	if err != nil {
		t.Fatal(err)
	}

	os.MkdirAll(filepath.Join(tmpdir, "foo"), 0755)
	ioutil.WriteFile(filepath.Join(tmpdir, "foo/baz.txt"), []byte{}, 0644)
	defer os.RemoveAll(tmpdir)

	err = os.Symlink(filepath.Join(tmpdir, "foo"), filepath.Join(tmpdir, "bar"))
	if err != nil {
		t.Skip(err.Error())
	}

	curdir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chdir(tmpdir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(curdir)

	got, err := GlobFollowSymlinks("**/*")
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"foo", "foo/baz.txt", "bar/baz.txt"}

	if !check(expected, got) {
		t.Errorf(`zglob failed: expected %v but got %v`, expected, got)
	}
}

func TestGlobError(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "zglob")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	err = os.MkdirAll(filepath.Join(tmpdir, "foo"), 0222)
	if err != nil {
		t.Fatal(err)
	}

	curdir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chdir(tmpdir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(curdir)

	got, err := Glob("**/*")
	if !errors.Is(err, os.ErrPermission) {
		t.Errorf(`zglob failed: expected %v but got %v`, os.ErrPermission, err)
	}
	if !check(nil, got) {
		t.Errorf(`zglob failed: expected %v but got %v`, nil, got)
	}
}

func BenchmarkGlob(b *testing.B) {
	tmpdir, savedCwd := setup()
	defer os.RemoveAll(tmpdir)
	defer os.Chdir(savedCwd)

	for i := 0; i < b.N; i++ {
		for _, test := range testGlobs {
			if test.err != "" {
				continue
			}
			got, err := Glob(test.pattern)
			if err != nil {
				b.Fatal(err)
			}
			if len(got) != len(test.expected) {
				b.Fatalf(`zglob failed: pattern %q: expected %v but got %v`, test.pattern, test.expected, got)
			}
		}
	}
}
