//go:build linux || darwin
// +build linux darwin

package fastwalk

import (
	"syscall"
	"testing"
	"unsafe"
)

// A dirent record returned by ReadDirent is only Reclen bytes long, which
// is much smaller than sizeof(syscall.Dirent). Overlaying *syscall.Dirent
// on such a record trips checkptr under -race
// ("converted pointer straddles multiple allocations"). See #49.
func TestParseDirEntShortRecord(t *testing.T) {
	var d syscall.Dirent
	nameOff := int(unsafe.Offsetof(d.Name))
	name := "hello"
	reclen := (nameOff + len(name) + 1 + 7) &^ 7

	buf := make([]byte, reclen)
	*(*uint16)(unsafe.Pointer(&buf[unsafe.Offsetof(d.Reclen)])) = uint16(reclen)
	buf[unsafe.Offsetof(d.Type)] = syscall.DT_REG
	copy(buf[nameOff:], name)
	// Set the inode field (Ino on linux/darwin) to a non-zero value.
	inoOff := unsafe.Offsetof(d.Ino)
	buf[inoOff] = 1

	consumed, got, typ := parseDirEnt(buf)
	if consumed != reclen {
		t.Errorf("consumed = %d, want %d", consumed, reclen)
	}
	if got != name {
		t.Errorf("name = %q, want %q", got, name)
	}
	if typ != 0 {
		t.Errorf("typ = %v, want 0", typ)
	}
}
