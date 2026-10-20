package core

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// The test binary doubles as another Foundry process (UI or MCP) working on
// the project in STORE_DIR when STORE_HELPER names what to do.
func TestMain(m *testing.M) {
	if mode := os.Getenv("STORE_HELPER"); mode != "" {
		if err := helper(mode); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func helper(mode string) error {
	s := &Store{Dir: os.Getenv("STORE_DIR")}
	n, _ := strconv.Atoi(os.Getenv("STORE_N"))
	switch mode {
	case "hold":
		// Takes the lock, says so, and keeps it until stdin closes.
		_, err := s.Update(func(p *Project) error {
			fmt.Println("locked")
			io.Copy(io.Discard, os.Stdin)
			p.Name = "held"
			return nil
		})
		return err
	case "update":
		return updateMany(s, n)
	case "exclusive":
		return exclusiveMany(s, n)
	}
	return fmt.Errorf("unknown mode %q", mode)
}

func updateMany(s *Store, n int) error {
	for i := 0; i < n; i++ {
		if _, err := s.Update(func(p *Project) error { return nil }); err != nil {
			return err
		}
	}
	return nil
}

// exclusiveMany makes n slow updates. Each marks itself as the active
// writer and ages the lock file past the old 30s stale limit, as a long
// scaffold would; a second writer inside at the same time is an error.
func exclusiveMany(s *Store, n int) error {
	marker := filepath.Join(s.Dir, StateDir, "writer")
	old := time.Now().Add(-time.Hour)
	for i := 0; i < n; i++ {
		_, err := s.Update(func(p *Project) error {
			f, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
			if err != nil {
				return fmt.Errorf("two writers at once: %w", err)
			}
			f.Close()
			defer os.Remove(marker)
			for j := 0; j < 300; j++ { // the long work, spanning many lock polls
				os.Chtimes(s.lockPath(), old, old)
				if _, err := s.Load(); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Init("demo"); err != nil {
		t.Fatal(err)
	}
	return s
}

func shortWait(t *testing.T) {
	old := lockWait
	lockWait = 300 * time.Millisecond
	t.Cleanup(func() { lockWait = old })
}

func helperCmd(s *Store, mode string, n int) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "STORE_HELPER="+mode, "STORE_DIR="+s.Dir, fmt.Sprintf("STORE_N=%d", n))
	return cmd
}

// holder is another process holding the project lock.
type holder struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	out   *bufio.Reader
}

func startHolder(t *testing.T, s *Store) *holder {
	t.Helper()
	cmd := helperCmd(s, "hold", 0)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	h := &holder{cmd: cmd, stdin: stdin, out: bufio.NewReader(stdout)}
	t.Cleanup(func() { stdin.Close(); cmd.Process.Kill(); cmd.Wait() })
	return h
}

// locked blocks until the holder reports it owns the lock.
func (h *holder) locked(t *testing.T) {
	t.Helper()
	line, err := h.out.ReadString('\n')
	if strings.TrimSpace(line) != "locked" {
		t.Fatalf("holder said %q, %v", line, err)
	}
}

// release lets the holder finish its update and waits for it to exit.
func (h *holder) release(t *testing.T) {
	t.Helper()
	h.stdin.Close()
	if err := h.cmd.Wait(); err != nil {
		t.Fatalf("holder: %v", err)
	}
}

func ageLock(t *testing.T, s *Store) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(s.lockPath(), old, old); err != nil {
		t.Fatal(err)
	}
}

func wantLocked(t *testing.T, s *Store) {
	t.Helper()
	_, err := s.Update(func(p *Project) error { p.Name = "intruder"; return nil })
	if err == nil || !strings.Contains(err.Error(), "dikunci") {
		t.Fatalf("Update while another process holds the lock: err = %v, want locked", err)
	}
}

func TestActiveLockNotStolen(t *testing.T) {
	s := newTestStore(t)
	shortWait(t)
	h := startHolder(t, s)
	h.locked(t)
	ageLock(t, s) // older than any stale limit: still held
	wantLocked(t, s)

	h.release(t)
	if _, err := s.Update(func(p *Project) error { return nil }); err != nil {
		t.Fatalf("Update after release: %v", err)
	}
	if p, _ := s.Load(); p.Name != "held" {
		t.Fatalf("name = %q, want the holder's write", p.Name)
	}
}

func TestFinishedOwnerKeepsNewOwnersLock(t *testing.T) {
	s := newTestStore(t)
	shortWait(t)
	a := startHolder(t, s)
	a.locked(t)
	ageLock(t, s)
	b := startHolder(t, s) // waits for a
	a.release(t)
	b.locked(t)
	wantLocked(t, s) // a finishing did not free b's lock
	b.release(t)
}

func TestCrashedOwnerLockRecovered(t *testing.T) {
	s := newTestStore(t)
	shortWait(t)
	h := startHolder(t, s)
	h.locked(t)
	h.cmd.Process.Kill()
	h.cmd.Wait()
	if _, err := os.Stat(s.lockPath()); err != nil {
		t.Fatalf("lock file: %v", err)
	}
	if _, err := s.Update(func(p *Project) error { return nil }); err != nil {
		t.Fatalf("Update after the owner crashed: %v", err)
	}

	// A lock file left by an older Foundry that crashed holds nothing.
	if err := os.WriteFile(s.lockPath(), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(func(p *Project) error { return nil }); err != nil {
		t.Fatalf("Update with a leftover lock file: %v", err)
	}
}

func TestConcurrentUpdatesAcrossProcesses(t *testing.T) {
	s := newTestStore(t)
	const procs, n = 4, 25

	done := make(chan struct{})
	readErr := make(chan error, 1)
	go func() {
		defer close(readErr)
		for {
			select {
			case <-done:
				return
			default:
			}
			if _, err := s.Load(); err != nil {
				readErr <- err
				return
			}
		}
	}()

	var wg sync.WaitGroup
	errs := make(chan error, procs+2)
	for i := 0; i < procs; i++ {
		cmd := helperCmd(s, "update", n)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- fmt.Errorf("%v: %s", err, out)
			}
		}()
	}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := updateMany(s, n); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(done)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if err := <-readErr; err != nil {
		t.Fatalf("reader saw a broken state: %v", err)
	}
	p, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := (procs + 2) * n; p.Version != want {
		t.Fatalf("version = %d, want %d (updates lost)", p.Version, want)
	}
	if tmp, _ := filepath.Glob(filepath.Join(s.Dir, StateDir, "*.tmp")); len(tmp) > 0 {
		t.Fatalf("temporary files left: %v", tmp)
	}
}

func TestLongUpdateHasOneWriter(t *testing.T) {
	s := newTestStore(t)
	const procs, n = 3, 10
	var wg sync.WaitGroup
	errs := make(chan error, procs)
	for i := 0; i < procs; i++ {
		cmd := helperCmd(s, "exclusive", n)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- fmt.Errorf("%v: %s", err, out)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if p, _ := s.Load(); p.Version != procs*n {
		t.Fatalf("version = %d, want %d", p.Version, procs*n)
	}
}

// A project saved during the old build pipeline opens as a finalized design;
// its task and build data are dropped, the design is kept.
func TestLoadLegacyBuildProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"name":"klinik","stage":"BUILD","story":"x","tasks":[{"id":"TASK-001","status":"TODO"}],"builds":[],
		"design":{"status":"approved","pages":[{"id":"PAGE-001","name":"Masuk","platform":"web","components":[]}]}}`
	if err := os.WriteFile(filepath.Join(dir, StateDir, "project.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Store{Dir: dir}
	p, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.Stage != StageReady || p.UI == nil || p.UI.Layout == nil || len(p.UI.Layout.Pages) != 1 || p.UI.Layout.Pages[0].ID != "page-001" {
		t.Fatalf("legacy project = stage %s ui %+v", p.Stage, p.UI)
	}
	if _, err := s.Update(func(*Project) error { return nil }); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, StateDir, "project.json"))
	if strings.Contains(string(b), `"tasks"`) || strings.Contains(string(b), `"builds"`) || strings.Contains(string(b), `"design"`) {
		t.Fatalf("old data kept: %s", b)
	}
}

func TestLoadLegacyDesignMigrates(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"name":"klinik","stage":"DESIGN","story":"x","theme":"modern-saas",
		"flows":[{"id":"FLOW-001","page_ids":["PAGE-001","PAGE-002"]}],
		"design":{"status":"draft","tech_version":3,
		  "system":{"direction":"tenang","colors":[{"name":"action.primary","value":"#1f6feb"}],"components":[{"name":"Button","value":"rounded","use":"aksi"}]},
		  "pages":[{"id":"PAGE-001","name":"Masuk","platform":"web","kind":"auth","route":"/masuk","states":{"error":"Gagal"},
		     "components":[{"id":"form-1","type":"form","label":"Login","items":["Email"]},{"type":"button","label":"Daftar","target":"PAGE-002"}]},
		   {"id":"PAGE-002","name":"Daftar","platform":"web","components":[]}]},
		"workflow":{"status":"draft","version":1,"commands":[{"id":"CMD-001","name":"UI","objective":"o","instructions":"i","status":"draft",
		  "artifacts":[{"id":"ui-design","hash":"abc"}]}]}}`
	if err := os.WriteFile(filepath.Join(dir, StateDir, "project.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := (&Store{Dir: dir}).Load()
	if err != nil {
		t.Fatal(err)
	}
	d := p.UI
	if d == nil || d.Rules == nil || d.Layout == nil || d.TechVersion != 3 {
		t.Fatalf("ui = %+v", d)
	}
	if d.Rules.Direction != "tenang" || len(d.Rules.Tokens.Colors) != 1 || d.Rules.Components[0].Name != "Button" {
		t.Errorf("rules = %+v", d.Rules)
	}
	pg := d.Layout.Pages[0]
	if pg.ID != "page-001" || pg.Pattern != "auth" || len(pg.Sections) != 2 || pg.Sections[0].ID != "form-1" ||
		len(pg.Links) != 1 || pg.Links[0].To != "page-002" || pg.States["error"] != "Gagal" {
		t.Errorf("page = %+v", pg)
	}
	if got := p.Flows[0].PageIDs; !slices.Equal(got, []string{"page-001", "page-002"}) {
		t.Errorf("flow pages = %v", got)
	}
	refs := p.Workflow.Commands[0].Artifacts
	if len(refs) != 2 || refs[0].ID != ArtifactDesignRules || refs[1].ID != ArtifactLayoutSpec || refs[0].Hash != "" {
		t.Errorf("command refs = %+v", refs)
	}
	// Nothing is invented: the missing rules show up as errors.
	if errs := Errors(p.DesignIssues(nil)); len(errs) == 0 {
		t.Error("migrated design must ask for the missing rules")
	}
}

// Platform targets saved with users (audience) open without them; a role
// named there gets the platform, a role with its own platform keeps it.
func TestLoadLegacyPlatformAudience(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"name":"klinik","stage":"DESIGN","story":"x",
		"discovery":{"roles":[{"name":"Pasien","description":"d"},{"name":"Dokter","description":"d","platform":"web"}]},
		"platform":{"targets":[{"kind":"android","audience":["Pasien","Dokter"],"rationale":"r"},{"kind":"backend","audience":[],"rationale":"r"}]},
		"stack":{"mode":"new","platform":{"web":false,"mobile":true,"backend":true},"approved":true,
		  "decisions":[{"area":"mobile","recommended":"Flutter","language":"Dart","reason":"x"},{"area":"cache","recommended":"Redis","reason":"x"}]}}`
	if err := os.WriteFile(filepath.Join(dir, StateDir, "project.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Store{Dir: dir}
	p, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if r := p.Discovery.Roles; r[0].Platform != PlatformAndroid || r[1].Platform != "web" {
		t.Fatalf("roles = %+v", r)
	}
	if len(p.Platform.Targets) != 2 || p.Platform.Targets[0].Rationale != "r" {
		t.Fatalf("targets = %+v", p.Platform.Targets)
	}
	// The old stack loads as it was: approved, Redis kept, nothing added.
	if st := p.Stack; st.State() != TechApproved || st.Choice(TechCache) != "Redis" || len(st.Decisions) != 2 ||
		st.DecisionState(st.Decisions[0]) != StateApproved {
		t.Fatalf("stack = %+v", st)
	}
	if _, err := s.Update(func(*Project) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, StateDir, "project.json")); strings.Contains(string(b), `"audience"`) {
		t.Fatalf("audience kept: %s", b)
	}
}
