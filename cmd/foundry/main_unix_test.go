//go:build unix

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/handrawanw/foundry/internal/config"
	"github.com/handrawanw/foundry/internal/core"
)

// The test binary is foundry itself (FOUNDRY_TEST_MAIN=1) and, with the
// fake-agent argument, the coding agent it connects to.
func TestMain(m *testing.M) {
	if os.Getenv("FOUNDRY_TEST_MAIN") == "1" {
		if len(os.Args) > 1 && os.Args[1] == "fake-agent" {
			fakeAgent()
			return
		}
		main()
		return
	}
	os.Exit(m.Run())
}

// fakeAgent answers the preflight probe; a coding run starts a server like
// a dev server would, writes both pids and then works forever.
func fakeAgent() {
	in, _ := io.ReadAll(os.Stdin)
	if strings.HasPrefix(string(in), "Balas hanya") {
		fmt.Println("OK")
		return
	}
	c := exec.Command("sleep", "300")
	if c.Start() != nil {
		os.Exit(1)
	}
	path := os.Getenv("FAKE_PIDS")
	os.WriteFile(path+".tmp", []byte(fmt.Sprintf("%d %d\n", os.Getpid(), c.Process.Pid)), 0o644)
	os.Rename(path+".tmp", path)
	time.Sleep(time.Hour)
}

func exited(pid int) bool {
	if syscall.Kill(pid, 0) == syscall.ESRCH {
		return true
	}
	out, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	st := strings.TrimSpace(string(out))
	return st == "" || strings.HasPrefix(st, "Z")
}

// until polls cond; the deadline only bounds a broken run.
func until(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); !cond(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal(what)
		}
	}
}

// SIGTERM to `foundry serve` during an agent build stops the agent and the
// server it started, records the build as stopped, and exits.
func TestServeStopsJobsOnSignal(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep tidak ada")
	}
	home, dir := t.TempDir(), t.TempDir()
	pids := filepath.Join(t.TempDir(), "pids")
	cfg, _ := json.Marshal(config.Config{Connection: &config.Connection{Mode: config.ModeAgent, AgentID: "custom",
		Command: []string{os.Args[0], "fake-agent"}}})
	if err := os.WriteFile(filepath.Join(home, "foundry.json"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := core.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Init("demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Update(func(p *core.Project) error {
		p.Stage = core.StageBuild
		p.Tasks = []core.Task{{ID: "T1", Title: "a", Area: "api", Files: []string{"a/x.go"}, AcceptanceCriteria: []string{"ok"}, Status: core.TaskTodo}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	srv := exec.Command(os.Args[0], "serve", "--addr", addr, dir)
	srv.Env = append(os.Environ(), "FOUNDRY_TEST_MAIN=1", "FOUNDRY_HOME="+home, "FAKE_PIDS="+pids)
	srv.Stderr = os.Stderr
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	exit := make(chan error, 1)
	go func() { exit <- srv.Wait() }()
	t.Cleanup(func() { srv.Process.Kill() })

	until(t, "server did not start", func() bool {
		res, err := http.Get("http://" + addr + "/api/v1/project")
		if err == nil {
			res.Body.Close()
		}
		return err == nil && res.StatusCode == 200
	})
	res, err := http.Post("http://"+addr+"/api/v1/project/build/start", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("start build: %d", res.StatusCode)
	}
	var agentPid, childPid int
	until(t, "agent did not start", func() bool {
		b, err := os.ReadFile(pids)
		return err == nil && func() bool { fmt.Sscan(string(b), &agentPid, &childPid); return true }()
	})
	t.Cleanup(func() { syscall.Kill(childPid, syscall.SIGKILL); syscall.Kill(agentPid, syscall.SIGKILL) })

	srv.Process.Signal(syscall.SIGTERM)
	select {
	case <-exit:
	case <-time.After(30 * time.Second):
		t.Fatal("foundry did not exit after SIGTERM")
	}
	for _, pid := range []int{agentPid, childPid} {
		until(t, fmt.Sprintf("process %d outlived foundry", pid), func() bool { return exited(pid) })
	}
	p, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.Job == nil || p.Job.Kind != "build" || p.Job.Status != core.JobDone || p.Job.Message != "Build dihentikan" {
		t.Fatalf("job = %+v", p.Job)
	}
	if tk := p.Task("T1"); tk.Status != core.TaskTodo {
		t.Fatalf("task = %s, want TODO", tk.Status)
	}
}
