//go:build !unix

package httpapi

// openNonblock is unused where the file system has no FIFOs.
const openNonblock = 0
