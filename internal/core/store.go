package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/handrawanw/foundry/internal/safefile"
)

// StateDir is the per-project folder holding the single source of truth.
const StateDir = ".foundry"

var ErrNotFound = errors.New("project belum dibuat")

var errExists = errors.New("project sudah ada")

// ErrLocked means another process held the state lock past lockWait.
var ErrLocked = errors.New("state sedang dikunci proses lain")

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
	err := s.withLock(func() error {
		if s.Exists() {
			return errExists // another process created it meanwhile
		}
		return s.write(p)
	})
	if errors.Is(err, errExists) {
		return s.Load()
	}
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
	// Build-era stages no longer exist: those projects had an approved design.
	switch p.Stage {
	case "PLAN", "BUILD", "VERIFY":
		p.Stage = StageReady
	}
	migrateLegacyDesign(&p, b)
	migrateArtifactIDs(&p)
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
	return safefile.WriteFile(s.path(), b, 0o644)
}

// lockWait is how long a mutation waits for another process to finish its
// own. The lock is held by the OS (see safefile), so it is never broken: a
// crashed owner frees it at once and a live one keeps it until it is done.
var lockWait = 40 * time.Second

func (s *Store) withLock(fn func() error) error {
	l, err := safefile.Acquire(s.lockPath(), lockWait)
	if errors.Is(err, safefile.ErrTimeout) {
		return ErrLocked
	}
	if err != nil {
		return err
	}
	defer l.Unlock()
	return fn()
}
