package httpapi

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// Read-only file explorer for the active project. Every path is resolved
// inside the project folder; symlinks pointing outside are refused.

const maxFileBytes = 1 << 20 // viewer limit

var hiddenEntries = map[string]bool{".git": true}

type fileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"` // relative, slash-separated
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// resolve maps a relative path to an absolute one inside root.
func resolve(root, rel string) (string, error) {
	rel = filepath.FromSlash(strings.TrimPrefix(rel, "/"))
	abs := filepath.Join(root, rel) // Join cleans ".." segments
	if !within(root, abs) {
		return "", badRequest("path di luar project")
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", httpError{http.StatusNotFound, "file tidak ditemukan"}
		}
		return "", err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if !within(realRoot, real) {
		return "", badRequest("symlink mengarah ke luar project")
	}
	return real, nil
}

func within(root, p string) bool {
	r, err := filepath.Rel(root, p)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}

func (s *Server) listFiles(r *http.Request) (any, error) {
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	rel := r.URL.Query().Get("path")
	dir, err := resolve(svc.Store.Dir, rel)
	if err != nil {
		return nil, err
	}
	entries, err := readDir(dir)
	if err != nil {
		return nil, badRequest("bukan folder")
	}
	out := []fileEntry{}
	for _, e := range entries {
		if hiddenEntries[e.Name()] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		isDir := e.IsDir()
		if info.Mode()&fs.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(dir, e.Name())); err == nil {
				isDir = st.IsDir()
			}
		}
		out = append(out, fileEntry{Name: e.Name(), Path: filepath.ToSlash(filepath.Join(rel, e.Name())), IsDir: isDir, Size: info.Size()})
	}
	// Folders first, then files, each alphabetical (case-insensitive).
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func (s *Server) readFile(r *http.Request) (any, error) {
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	rel := r.URL.Query().Get("path")
	p, err := resolve(svc.Store.Dir, rel)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, badRequest("path adalah folder")
	}
	if !st.Mode().IsRegular() {
		return nil, badRequest("bukan file biasa")
	}
	f, err := openNoWait(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// The path may have been replaced since the stat.
	if st, err = f.Stat(); err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, badRequest("bukan file biasa")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxFileBytes))
	if err != nil {
		return nil, err
	}
	binary := bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(trimPartialRune(data))
	res := map[string]any{
		"path":      filepath.ToSlash(rel),
		"size":      st.Size(),
		"binary":    binary,
		"truncated": st.Size() > maxFileBytes,
	}
	if !binary {
		res["content"] = string(data)
	}
	return res, nil
}

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

// trimPartialRune drops a UTF-8 sequence cut by the size limit.
func trimPartialRune(b []byte) []byte {
	for i := 0; i < utf8.UTFMax && len(b) > 0; i++ {
		if r, _ := utf8.DecodeLastRune(b); r != utf8.RuneError {
			return b
		}
		b = b[:len(b)-1]
	}
	return b
}
