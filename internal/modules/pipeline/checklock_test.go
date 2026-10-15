package pipeline_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/pipeline"
	"github.com/handrawanw/foundry/internal/modules/work"
	"github.com/handrawanw/foundry/internal/safefile"
)

const waitingProgress = "Menunggu build & tes proses lain selesai"

// lockHelper runs when the test binary plays another process: an MCP host
// on the project in LOCK_DIR ("submit", "next"), or the project's own test
// command ("check"), which reports to the test over CHECK_ADDR and runs
// until the test answers.
func lockHelper(mode string) int {
	if mode == "check" {
		c, err := net.Dial("tcp", os.Getenv("CHECK_ADDR"))
		if err != nil {
			fmt.Println(err)
			return 1
		}
		defer c.Close()
		fmt.Fprintln(c, "start")
		line, _ := bufio.NewReader(c).ReadString('\n')
		if strings.TrimSpace(line) != "pass" {
			return 1
		}
		return 0
	}
	st, err := core.NewStore(os.Getenv("LOCK_DIR"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	s := pipeline.New(st, fakeAI{mcp: true})
	switch mode {
	case "submit":
		t, _, err := s.SubmitTask(os.Getenv("LOCK_TASK"), os.Getenv("LOCK_WORKER"),
			[]work.Verdict{{Index: 1, Met: true, Evidence: "x"}}, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("status", t.Status)
	case "next":
		w, err := s.NextWork(os.Getenv("LOCK_WORKER"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(w.Kind, w.Prompt)
	default:
		return 2
	}
	return 0
}

// hostProject is a project in an MCP build whose only check command is the
// test binary in "check" mode. owners maps task ID to its worker; "" leaves
// the task DONE.
func hostProject(t *testing.T, owners map[string]string) *core.Store {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the check script uses a POSIX shell")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm runs the check script")
	}
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	dir := t.TempDir()
	pkg, _ := json.Marshal(map[string]any{"name": "x", "private": true,
		"scripts": map[string]string{"test": fmt.Sprintf("FOUNDRY_LOCK_HELPER=check '%s' -test.run=^$", os.Args[0])}})
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkg, 0o644); err != nil {
		t.Fatal(err)
	}
	// Dependencies count as installed, so no install step runs.
	marker := filepath.Join(dir, "node_modules", ".package-lock.json")
	os.MkdirAll(filepath.Dir(marker), 0o755)
	os.WriteFile(marker, []byte("{}"), 0o644)
	future := time.Now().Add(time.Hour)
	os.Chtimes(marker, future, future)

	st, err := core.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Init("lock"); err != nil {
		t.Fatal(err)
	}
	_, err = st.Update(func(p *core.Project) error {
		p.Stage = core.StageBuild
		p.Job = &core.Job{Kind: "build", Status: core.JobWaiting, Started: time.Now().UTC()}
		for _, id := range sortedKeys(owners) {
			tk := core.Task{ID: id, Title: id, AcceptanceCriteria: []string{"works"}, Status: core.TaskDone, Attempts: 1}
			if w := owners[id]; w != "" {
				tk.Status, tk.Owner, tk.Seen = core.TaskInProgress, w, time.Now().UTC()
			}
			p.Tasks = append(p.Tasks, tk)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func sortedKeys(m map[string]string) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// checkServer is where running checks report; each waits for pass.
type checkServer struct {
	addr  string
	conns chan net.Conn
}

func newCheckServer(t *testing.T) *checkServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cs := &checkServer{addr: ln.Addr().String(), conns: make(chan net.Conn, 8)}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			bufio.NewReader(c).ReadString('\n') // "start"
			cs.conns <- c
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		for {
			select {
			case c := <-cs.conns:
				c.Close()
			default:
				return
			}
		}
	})
	return cs
}

func pass(c net.Conn) {
	fmt.Fprintln(c, "pass")
	c.Close()
}

// host is another Foundry process (an MCP host) on the project.
type host struct {
	cmd  *exec.Cmd
	out  strings.Builder
	done chan error
}

func startHost(t *testing.T, st *core.Store, cs *checkServer, mode, task, worker string) *host {
	t.Helper()
	h := &host{done: make(chan error, 1)}
	h.cmd = exec.Command(os.Args[0], "-test.run=^$")
	h.cmd.Env = append(os.Environ(), "FOUNDRY_LOCK_HELPER="+mode, "LOCK_DIR="+st.Dir,
		"LOCK_TASK="+task, "LOCK_WORKER="+worker, "CHECK_ADDR="+cs.addr)
	h.cmd.Stdout, h.cmd.Stderr = &h.out, &h.out
	if err := h.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { h.done <- h.cmd.Wait() }()
	t.Cleanup(func() {
		h.cmd.Process.Kill()
		<-h.done
		h.done <- nil
	})
	return h
}

// finished waits for the host to exit and returns what it printed.
func (h *host) finished(t *testing.T) string {
	t.Helper()
	err := <-h.done
	h.done <- err
	if err != nil {
		t.Fatalf("host: %v\n%s", err, h.out.String())
	}
	return h.out.String()
}

func checksLock(st *core.Store) string { return filepath.Join(st.Dir, core.StateDir, "checks.lock") }

func wantChecksLocked(t *testing.T, st *core.Store) {
	t.Helper()
	l, err := safefile.Acquire(checksLock(st), 0)
	if err == nil {
		l.Unlock()
		t.Fatal("checks lock is free while a check runs")
	}
	if !errors.Is(err, safefile.ErrTimeout) {
		t.Fatal(err)
	}
}

func wantChecksFree(t *testing.T, st *core.Store) {
	t.Helper()
	l, err := safefile.Acquire(checksLock(st), 0)
	if err != nil {
		t.Fatalf("checks lock not released: %v", err)
	}
	l.Unlock()
}

func TestChecksRunOneProcessAtATime(t *testing.T) {
	st := hostProject(t, map[string]string{"T1": "w1", "T2": "w2"})
	cs := newCheckServer(t)
	a := startHost(t, st, cs, "submit", "T1", "w1")
	first := <-cs.conns // a's check runs
	wantChecksLocked(t, st)

	// b reaches its check while a's runs: it must wait, not run beside it.
	b := startHost(t, st, cs, "submit", "T2", "w2")
	for {
		select {
		case c := <-cs.conns:
			c.Close()
			t.Fatal("two checks ran at once")
		case err := <-b.done:
			t.Fatalf("host b ended while a's check ran: %v\n%s", err, b.out.String())
		default:
		}
		p, err := st.Load()
		if err != nil {
			t.Fatal(err)
		}
		if p.Task("T2").Progress == waitingProgress {
			break
		}
		time.Sleep(10 * time.Millisecond) // poll interval for the state above, not a wait for timing
	}

	pass(first)
	if out := a.finished(t); !strings.Contains(out, "status DONE") {
		t.Fatalf("host a: %s", out)
	}
	second := <-cs.conns // b's check, only now
	wantChecksLocked(t, st)
	pass(second)
	if out := b.finished(t); !strings.Contains(out, "status DONE") {
		t.Fatalf("host b: %s", out)
	}
	wantChecksFree(t, st)
	p, _ := st.Load()
	if len(p.Builds) != 2 {
		t.Fatalf("builds = %d, want 2", len(p.Builds))
	}
}

func TestChecksLockFreedWhenOwnerCrashes(t *testing.T) {
	st := hostProject(t, map[string]string{"T1": "w1"})
	cs := newCheckServer(t)
	a := startHost(t, st, cs, "submit", "T1", "w1")
	c := <-cs.conns
	defer c.Close()
	wantChecksLocked(t, st)
	a.cmd.Process.Kill()
	<-a.done
	a.done <- nil
	wantChecksFree(t, st) // its check may still be running, the lock is not
}

func TestChecksLockFailureKeepsState(t *testing.T) {
	st := hostProject(t, map[string]string{"T1": "w1"})
	// A lock file that cannot be opened: the lock cannot be taken.
	if err := os.Mkdir(checksLock(st), 0o755); err != nil {
		t.Fatal(err)
	}
	s := pipeline.New(st, fakeAI{mcp: true})
	_, _, err := s.SubmitTask("T1", "w1", []work.Verdict{{Index: 1, Met: true, Evidence: "x"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "dikunci") {
		t.Fatalf("SubmitTask err = %v, want lock error", err)
	}
	p, _ := st.Load()
	tk := p.Task("T1")
	if tk.Status != core.TaskInProgress || tk.Owner != "w1" || tk.Attempts != 1 || tk.Progress != "" || len(tk.Notes) != 0 {
		t.Fatalf("task after lock failure = %+v, want unchanged and still the host's", tk)
	}
	if len(p.Builds) != 0 {
		t.Fatalf("builds = %d, want none recorded", len(p.Builds))
	}

	// The queue empties: the final verification fails visibly instead of
	// staying "running" or reporting results of checks that never ran.
	st.Update(func(p *core.Project) error { p.Task("T1").Status = core.TaskDone; return nil })
	if _, err := s.NextWork("w1"); err != nil {
		t.Fatal(err)
	}
	p, _ = st.Load()
	if p.Job == nil || p.Job.Kind != "verify" || p.Job.Status != core.JobFailed || !strings.Contains(p.Job.Error, "dikunci") {
		t.Fatalf("job = %+v, want verify failed on the lock", p.Job)
	}
	if p.Verification != nil || len(p.Builds) != 0 {
		t.Fatalf("verification = %+v, builds = %d: results without checks", p.Verification, len(p.Builds))
	}
}

func TestFinishBuildStartsOneVerification(t *testing.T) {
	st := hostProject(t, map[string]string{"T1": ""})
	cs := newCheckServer(t)
	hosts := []*host{startHost(t, st, cs, "next", "", "a"), startHost(t, st, cs, "next", "", "b")}
	first := <-cs.conns // the winner's verification runs

	// The other host must return without verifying.
	var loser, winner *host
	for loser == nil {
		select {
		case c := <-cs.conns:
			c.Close()
			t.Fatal("two hosts started the final verification")
		case err := <-hosts[0].done:
			hosts[0].done <- err
			loser, winner = hosts[0], hosts[1]
		case err := <-hosts[1].done:
			hosts[1].done <- err
			loser, winner = hosts[1], hosts[0]
		}
	}
	if out := loser.finished(t); !strings.Contains(out, "none Verifikasi sudah dijalankan host lain") {
		t.Fatalf("losing host: %s", out)
	}
	pass(first)
	if out := winner.finished(t); !strings.Contains(out, "Foundry menjalankan verifikasi") {
		t.Fatalf("winning host: %s", out)
	}

	p, _ := st.Load()
	if p.Job == nil || p.Job.Kind != "verify" || p.Job.Status != core.JobDone || p.Verification == nil {
		t.Fatalf("job = %+v, verification = %v", p.Job, p.Verification != nil)
	}
	if len(p.Builds) != 1 {
		t.Fatalf("builds = %d, want the winner's only", len(p.Builds))
	}
	at, logs := p.Verification.At, verifyLogs(p)
	if logs != 1 {
		t.Fatalf("verification results logged %d times", logs)
	}

	// A host arriving afterwards leaves the result alone.
	late := startHost(t, st, cs, "next", "", "c")
	late.finished(t)
	p, _ = st.Load()
	if !p.Verification.At.Equal(at) || len(p.Builds) != 1 || verifyLogs(p) != 1 {
		t.Fatal("a later host overwrote the verification")
	}
}

func verifyLogs(p *core.Project) int {
	n := 0
	for _, e := range p.Events {
		if strings.HasPrefix(e.Message, "Verifikasi:") || e.Message == "Project siap diperiksa" {
			n++
		}
	}
	return n
}
