// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux,!appengine darwin freebsd openbsd netbsd

package fastwalk

import (
	"bytes"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const blockSize = 8 << 10

// unknownFileMode is a sentinel (and bogus) os.FileMode
// value used to represent a syscall.DT_UNKNOWN Dirent.Type.
const unknownFileMode os.FileMode = os.ModeNamedPipe | os.ModeSocket | os.ModeDevice

func readDir(dirName string, fn func(dirName, entName string, typ os.FileMode) error) error {
	fd, err := syscall.Open(dirName, 0, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	// The buffer must be at least a block long.
	buf := make([]byte, blockSize) // stack-allocated; doesn't escape
	bufp := 0                      // starting read position in buf
	nbuf := 0                      // end valid data in buf
	for {
		if bufp >= nbuf {
			bufp = 0
			nbuf, err = syscall.ReadDirent(fd, buf)
			if err != nil {
				return os.NewSyscallError("readdirent", err)
			}
			if nbuf <= 0 {
				return nil
			}
		}
		consumed, name, typ := parseDirEnt(buf[bufp:nbuf])
		bufp += consumed
		if name == "" || name == "." || name == ".." {
			continue
		}
		// Fallback for filesystems (like old XFS) that don't
		// support Dirent.Type and have DT_UNKNOWN (0) there
		// instead.
		if typ == unknownFileMode {
			fi, err := os.Lstat(dirName + "/" + name)
			if err != nil {
				// It got deleted in the meantime.
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			typ = fi.Mode() & os.ModeType
		}
		if err := fn(dirName, name, typ); err != nil {
			return err
		}
	}
}

// readUint reads a native-endian unsigned integer of size bytes at off.
func readUint(buf []byte, off, size uintptr) (uint64, bool) {
	if uintptr(len(buf)) < off+size {
		return 0, false
	}
	ptr := unsafe.Pointer(&buf[off])
	switch size {
	case 1:
		return uint64(*(*uint8)(ptr)), true
	case 2:
		return uint64(*(*uint16)(ptr)), true
	case 4:
		return uint64(*(*uint32)(ptr)), true
	case 8:
		return *(*uint64)(ptr), true
	default:
		return 0, false
	}
}

func parseDirEnt(buf []byte) (consumed int, name string, typ os.FileMode) {
	// Read fields from the byte slice. Do not overlay *syscall.Dirent:
	// the remaining buffer is only Reclen bytes (often ~32), while
	// Dirent.Name is 1024 bytes on Darwin. That conversion trips
	// checkptr ("converted pointer straddles multiple allocations")
	// — see mattn/go-zglob#49 and golang/go#41941.
	reclenOff := unsafe.Offsetof(syscall.Dirent{}.Reclen)
	reclenSize := unsafe.Sizeof(syscall.Dirent{}.Reclen)
	if v := reclenOff + reclenSize; uintptr(len(buf)) < v {
		panic(fmt.Sprintf("buf size of %d smaller than dirent header size %d", len(buf), v))
	}
	reclen64, ok := readUint(buf, reclenOff, reclenSize)
	if !ok {
		panic(fmt.Sprintf("buf size of %d smaller than dirent header", len(buf)))
	}
	reclen := int(reclen64)
	if len(buf) < reclen {
		panic(fmt.Sprintf("buf size %d < record length %d", len(buf), reclen))
	}
	if reclen == 0 {
		return len(buf), "", 0
	}
	consumed = reclen
	rec := buf[:reclen]

	if direntInodeFrom(rec) == 0 { // File absent in directory.
		return
	}

	typeOff := unsafe.Offsetof(syscall.Dirent{}.Type)
	var dt byte
	if uintptr(len(rec)) > typeOff {
		dt = rec[typeOff]
	}
	switch dt {
	case syscall.DT_REG:
		typ = 0
	case syscall.DT_DIR:
		typ = os.ModeDir
	case syscall.DT_LNK:
		typ = os.ModeSymlink
	case syscall.DT_BLK:
		typ = os.ModeDevice
	case syscall.DT_FIFO:
		typ = os.ModeNamedPipe
	case syscall.DT_SOCK:
		typ = os.ModeSocket
	case syscall.DT_UNKNOWN:
		typ = unknownFileMode
	default:
		// Skip weird things.
		// It's probably a DT_WHT (http://lwn.net/Articles/325369/)
		// or something. Revisit if/when this package is moved outside
		// of goimports. goimports only cares about regular files,
		// symlinks, and directories.
		return
	}

	nameOff := unsafe.Offsetof(syscall.Dirent{}.Name)
	if uintptr(len(rec)) <= nameOff {
		return
	}
	nameBuf := rec[nameOff:]
	nameLen := bytes.IndexByte(nameBuf, 0)
	if nameLen < 0 {
		nameLen = len(nameBuf)
	}

	// Special cases for common things:
	if nameLen == 1 && nameBuf[0] == '.' {
		name = "."
	} else if nameLen == 2 && nameBuf[0] == '.' && nameBuf[1] == '.' {
		name = ".."
	} else {
		name = string(nameBuf[:nameLen])
	}
	return
}
