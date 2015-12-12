package zglob

import (
	"io/ioutil"
	"os"
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

var testzglob = []struct {
	pattern  string
	expected []string
	err      error
}{
	{`fo*`, []string{`foo`}, nil},
	{`foo`, []string{`foo`}, nil},
	{`foo/*`, []string{`foo/bar`, `foo/baz`}, nil},
	{`foo/**`, []string{`foo/bar`, `foo/baz`}, nil},
	{`f*o/**`, []string{`foo/bar`, `foo/baz`}, nil},
	{`*oo/**`, []string{`foo/bar`, `foo/baz`, `hoo/bar`}, nil},
	{`*oo/b*`, []string{`foo/bar`, `foo/baz`, `hoo/bar`}, nil},
	{`*oo/*z`, []string{`foo/baz`}, nil},
	{`foo/**/*`, []string{`foo/bar`, `foo/bar/baz`, `foo/bar/baz.txt`, `foo/bar/baz/noo.txt`, `foo/baz`}, nil},
	{`*oo/**/*`, []string{`foo/bar`, `foo/bar/baz`, `foo/bar/baz.txt`, `foo/bar/baz/noo.txt`, `foo/baz`, `hoo/bar`}, nil},
	{`*oo/*.txt`, []string{}, nil},
	{`*oo/*/*.txt`, []string{`foo/bar/baz.txt`}, nil},
	{`*oo/**/*.txt`, []string{`foo/bar/baz.txt`, `foo/bar/baz/noo.txt`}, nil},
	{`doo`, nil, os.ErrNotExist},
	{`./f*`, []string{`foo`}, nil},
	{`${PWD}/f*`, []string{`foo`}, nil},
	{`${PWD}/**/*`, []string{`foo`, `foo/bar`, `foo/bar/baz`, `foo/bar/baz.txt`, `foo/bar/baz/noo.txt`, `foo/baz`, `hoo`, `hoo/bar`}, nil},
}

func TestZGlob(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "zglob")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	os.MkdirAll(filepath.Join(tmpdir, "foo/baz"), 0755)
	os.MkdirAll(filepath.Join(tmpdir, "foo/bar"), 0755)
	ioutil.WriteFile(filepath.Join(tmpdir, "foo/bar/baz.txt"), []byte{}, 0644)
	os.MkdirAll(filepath.Join(tmpdir, "foo/bar/baz"), 0755)
	ioutil.WriteFile(filepath.Join(tmpdir, "foo/bar/baz/noo.txt"), []byte{}, 0644)
	os.MkdirAll(filepath.Join(tmpdir, "hoo/bar"), 0755)
	ioutil.WriteFile(filepath.Join(tmpdir, "foo/bar/baz.txt"), []byte{}, 0644)

	err = os.Chdir(tmpdir)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range testzglob {
		test.pattern = strings.Replace(test.pattern, `${PWD}`, tmpdir, -1)
		got, err := Glob(test.pattern)
		if err != nil {
			if test.err != err {
				t.Error(err)
			}
			continue
		}
		if !check(test.expected, got) {
			t.Errorf(`zglob failed: pattern %q: expected %v but got %v`, test.pattern, test.expected, got)
		}
	}
}
