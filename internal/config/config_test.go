package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

// The test binary doubles as a second Foundry process (UI or MCP) that
// updates the config when CONFIG_HELPER names it.
func TestMain(m *testing.M) {
	if id := os.Getenv("CONFIG_HELPER"); id != "" {
		n, _ := strconv.Atoi(os.Getenv("CONFIG_N"))
		if err := addMany(id, n); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// addMany makes n updates, each adding one entry named after id.
func addMany(id string, n int) error {
	for i := 0; i < n; i++ {
		entry := fmt.Sprintf("%s-%d", id, i)
		if _, err := Update(func(c *Config) { c.Recent = append(c.Recent, entry) }); err != nil {
			return err
		}
	}
	return nil
}

func TestUpdateAcrossProcesses(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FOUNDRY_HOME", home)
	const procs, n = 4, 25

	// A reader polls the file the whole time: it must always parse.
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
			b, err := os.ReadFile(path())
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			var c Config
			if err == nil {
				err = json.Unmarshal(b, &c)
			}
			if err != nil {
				readErr <- err
				return
			}
		}
	}()

	var wg sync.WaitGroup
	errs := make(chan error, procs+2)
	for i := 0; i < procs; i++ {
		cmd := exec.Command(os.Args[0], "-test.run=^$")
		cmd.Env = append(os.Environ(), fmt.Sprintf("CONFIG_HELPER=proc%d", i), fmt.Sprintf("CONFIG_N=%d", n))
		wg.Add(1)
		go func() {
			defer wg.Done()
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- fmt.Errorf("%v: %s", err, out)
			}
		}()
	}
	// Goroutines of this process take part too.
	for i := 0; i < 2; i++ {
		id := fmt.Sprintf("local%d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := addMany(id, n); err != nil {
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
		t.Fatalf("reader saw a broken config: %v", err)
	}

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := (procs + 2) * n; len(c.Recent) != want {
		t.Fatalf("entries = %d, want %d (updates lost)", len(c.Recent), want)
	}
	seen := map[string]bool{}
	for _, e := range c.Recent {
		seen[e] = true
	}
	for i := 0; i < procs; i++ {
		for j := 0; j < n; j++ {
			if e := fmt.Sprintf("proc%d-%d", i, j); !seen[e] {
				t.Fatalf("missing %s", e)
			}
		}
	}
	if tmp, _ := filepath.Glob(filepath.Join(home, "*.tmp")); len(tmp) > 0 {
		t.Fatalf("temporary files left: %v", tmp)
	}
}
