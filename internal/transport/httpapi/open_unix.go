//go:build unix

package httpapi

import "syscall"

// openNonblock makes opening a FIFO return at once instead of waiting for
// a writer; regular files and folders read as usual.
const openNonblock = syscall.O_NONBLOCK
