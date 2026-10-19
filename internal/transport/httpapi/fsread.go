package httpapi

import (
	"io/fs"
	"os"
	"sort"
)

// openNoWait opens path for reading without blocking: opening a FIFO
// waits for a writer, which would hang the handler for good.
func openNoWait(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|openNonblock, 0)
}

// readDir is os.ReadDir that refuses anything but a folder instead of
// blocking on a FIFO.
func readDir(path string) ([]fs.DirEntry, error) {
	f, err := openNoWait(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, badRequest("bukan folder")
	}
	entries, err := f.ReadDir(-1)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, err
}
