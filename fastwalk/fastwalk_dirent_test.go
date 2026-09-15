//go:build linux || darwin
// +build linux darwin

package fastwalk

import (
	"os"
	"syscall"
	"testing"
	"unsafe"
)

func TestParseDirEntSmallRecord(t *testing.T) {
	// A real readdir record is only Reclen bytes, far smaller than
	// sizeof(Dirent) (1048 on Darwin; Name is 1024). Overlaying
	// *syscall.Dirent / [sizeof(Name)]byte on that slice trips
	// checkptr: "converted pointer straddles multiple allocations"
	// (mattn/go-zglob#49).
	nameOff := int(unsafe.Offsetof(syscall.Dirent{}.Name))
	want := "hello"
	reclen := nameOff + len(want) + 1
	if rem := reclen % 8; rem != 0 {
		reclen += 8 - rem
	}
	buf := make([]byte, reclen)

	if !putUint(buf, unsafe.Offsetof(syscall.Dirent{}.Ino), unsafe.Sizeof(syscall.Dirent{}.Ino), 1) {
		t.Fatal("failed to write inode")
	}
	if !putUint(buf, unsafe.Offsetof(syscall.Dirent{}.Reclen), unsafe.Sizeof(syscall.Dirent{}.Reclen), uint64(reclen)) {
		t.Fatal("failed to write reclen")
	}
	typeOff := unsafe.Offsetof(syscall.Dirent{}.Type)
	if int(typeOff) >= len(buf) {
		t.Fatal("type offset past record")
	}
	buf[typeOff] = syscall.DT_REG
	copy(buf[nameOff:], want)

	consumed, got, typ := parseDirEnt(buf)
	if consumed != reclen {
		t.Fatalf("consumed=%d want %d", consumed, reclen)
	}
	if got != want {
		t.Fatalf("name=%q want %q", got, want)
	}
	if typ != 0 {
		t.Fatalf("typ=%v want 0 (regular file)", typ)
	}
}

func TestParseDirEntDot(t *testing.T) {
	nameOff := int(unsafe.Offsetof(syscall.Dirent{}.Name))
	reclen := nameOff + 2
	if rem := reclen % 8; rem != 0 {
		reclen += 8 - rem
	}
	buf := make([]byte, reclen)
	putUint(buf, unsafe.Offsetof(syscall.Dirent{}.Ino), unsafe.Sizeof(syscall.Dirent{}.Ino), 1)
	putUint(buf, unsafe.Offsetof(syscall.Dirent{}.Reclen), unsafe.Sizeof(syscall.Dirent{}.Reclen), uint64(reclen))
	buf[unsafe.Offsetof(syscall.Dirent{}.Type)] = syscall.DT_DIR
	buf[nameOff] = '.'

	consumed, got, typ := parseDirEnt(buf)
	if consumed != reclen {
		t.Fatalf("consumed=%d want %d", consumed, reclen)
	}
	if got != "." {
		t.Fatalf("name=%q want .", got)
	}
	if typ != os.ModeDir {
		t.Fatalf("typ=%v want dir", typ)
	}
}

func TestParseDirEntZeroInode(t *testing.T) {
	nameOff := int(unsafe.Offsetof(syscall.Dirent{}.Name))
	reclen := nameOff + 2
	if rem := reclen % 8; rem != 0 {
		reclen += 8 - rem
	}
	buf := make([]byte, reclen)
	putUint(buf, unsafe.Offsetof(syscall.Dirent{}.Reclen), unsafe.Sizeof(syscall.Dirent{}.Reclen), uint64(reclen))
	buf[unsafe.Offsetof(syscall.Dirent{}.Type)] = syscall.DT_REG
	copy(buf[nameOff:], "x")

	consumed, got, _ := parseDirEnt(buf)
	if consumed != reclen {
		t.Fatalf("consumed=%d want %d", consumed, reclen)
	}
	if got != "" {
		t.Fatalf("name=%q want empty for inode 0", got)
	}
}

func putUint(buf []byte, off, size uintptr, v uint64) bool {
	if uintptr(len(buf)) < off+size {
		return false
	}
	ptr := unsafe.Pointer(&buf[off])
	switch size {
	case 1:
		*(*uint8)(ptr) = uint8(v)
	case 2:
		*(*uint16)(ptr) = uint16(v)
	case 4:
		*(*uint32)(ptr) = uint32(v)
	case 8:
		*(*uint64)(ptr) = v
	default:
		return false
	}
	return true
}
