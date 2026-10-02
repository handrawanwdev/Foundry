package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StateDir is the per-project folder holding the single source of truth.
const StateDir = ".foundry"

var ErrNotFound = errors.New("project belum dibuat")

// Store persists one project to <dir>/.foundry/project.json. The HTTP server
// and the MCP process may run concurrently, so every mutation goes through a
// cross-process lock file and re-reads state from disk before applying.
type Store struct {
	Dir string
}

func NewStore(dir string) (*Store, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	return &Store{Dir: abs}, nil
}

func (s *Store) path() string     { return filepath.Join(s.Dir, StateDir, "project.json") }
func (s *Store) lockPath() string { return filepath.Join(s.Dir, StateDir, "project.lock") }

func (s *Store) Exists() bool {
	_, err := os.Stat(s.path())
	return err == nil
}

// Init creates the project file if it does not exist yet.
func (s *Store) Init(name string) (*Project, error) {
	if s.Exists() {
		return s.Load()
	}
	if err := os.MkdirAll(filepath.Join(s.Dir, StateDir), 0o755); err != nil {
		return nil, err
	}
	if name == "" {
		name = filepath.Base(s.Dir)
	}
	p := &Project{Name: name, Dir: s.Dir, Stage: StageTell}
	p.Log("system", "Project dibuat")
	err := s.withLock(func() error { return s.write(p) })
	return p, err
}

func (s *Store) Load() (*Project, error) {
	b, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var p Project
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("state rusak: %w", err)
	}
	p.Dir = s.Dir
	if d := p.Design; d != nil {
		// Designs saved before the Design IR get their layout here, once.
		EnsureIR(d.Pages, p.Name)
		if d.Revision != nil {
			EnsureIR(d.Revision.PreviousPages, p.Name)
		}
	}
	return &p, nil
}

// Update loads the latest state, applies fn and saves it atomically. If fn
// returns an error nothing is written.
func (s *Store) Update(fn func(p *Project) error) (*Project, error) {
	var out *Project
	err := s.withLock(func() error {
		p, err := s.Load()
		if err != nil {
			return err
		}
		if err := fn(p); err != nil {
			return err
		}
		p.Version++
		if err := s.write(p); err != nil {
			return err
		}
		out = p
		return nil
	})
	return out, err
}

// ModTime lets watchers detect changes made by another process (MCP).
func (s *Store) ModTime() time.Time {
	st, err := os.Stat(s.path())
	if err != nil {
		return time.Time{}
	}
	return st.ModTime()
}

func (s *Store) write(p *Project) error {
	p.UpdatedAt = time.Now().UTC()
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path())
}

// staleLock is when a lock file counts as left by a crashed process. The
// wait outlasts it, so a lock left by a crash is broken instead of failing
// the caller.
const staleLock = 30 * time.Second

func (s *Store) withLock(fn func() error) error {
	deadline := time.Now().Add(staleLock + 10*time.Second)
	for {
		f, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			f.Close()
			defer os.Remove(s.lockPath())
			return fn()
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		// Break locks left behind by a crashed process.
		if st, serr := os.Stat(s.lockPath()); serr == nil && time.Since(st.ModTime()) > staleLock {
			os.Remove(s.lockPath())
			continue
		}
		if time.Now().After(deadline) {
			return errors.New("state sedang dikunci proses lain")
		}
		time.Sleep(25 * time.Millisecond)
	}
}
