// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux,!appengine darwin

package fastwalk

import (
	"syscall"
	"unsafe"
)

func direntInodeFrom(buf []byte) uint64 {
	u, ok := readUint(buf, unsafe.Offsetof(syscall.Dirent{}.Ino), unsafe.Sizeof(syscall.Dirent{}.Ino))
	if !ok {
		return 0
	}
	return u
}
