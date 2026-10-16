// Package checks runs the project's own build/test commands and records the
// result. PASSED comes only from exit codes; only commands produced by scan
// are ever executed (allowlist).
package checks

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/proc"
)

const maxOutput = 6000

// StepTimeout bounds one command.
var StepTimeout = 10 * time.Minute

// Run executes cmds in order and stops at the first failure.
func Run(ctx context.Context, dir string, cmds []core.Command, taskID string) core.BuildRun {
	run := core.BuildRun{ID: fmt.Sprintf("BUILD-%d", time.Now().UnixNano()), TaskID: taskID, At: time.Now().UTC(), Passed: true}
	if len(cmds) == 0 {
		run.Passed = false
		run.Reason = "Tidak ada command build/test yang terdeteksi di project"
		return run
	}
	for _, c := range append(installs(dir, cmds), cmds...) {
		st := runOne(ctx, dir, c)
		run.Steps = append(run.Steps, st)
		if st.ExitCode != 0 {
			run.Passed = false
			run.Reason = fmt.Sprintf("%s gagal (%s)", c.Name, st.Command)
			break
		}
	}
	return run
}

// installs returns one dependency install per Node directory the commands
// run in whose node_modules is missing or older than its manifest/lockfile.
// Agents run headless and cannot install, so without this a fresh scaffold
// fails every check with "tsc: not found".
func installs(dir string, cmds []core.Command) []core.Command {
	var out []core.Command
	seen := map[string]bool{}
	for _, c := range cmds {
		if seen[c.Dir] {
			continue
		}
		seen[c.Dir] = true
		base := filepath.Join(dir, c.Dir)
		manifest, err := os.Stat(filepath.Join(base, "package.json"))
		if err != nil {
			continue
		}
		argv, lock := []string{"npm", "install", "--no-audit", "--no-fund"}, ""
		switch {
		case exists(filepath.Join(base, "pnpm-lock.yaml")):
			argv, lock = []string{"pnpm", "install", "--frozen-lockfile"}, "pnpm-lock.yaml"
		case exists(filepath.Join(base, "yarn.lock")):
			argv, lock = []string{"yarn", "install", "--frozen-lockfile"}, "yarn.lock"
		case exists(filepath.Join(base, "package-lock.json")):
			argv, lock = []string{"npm", "ci", "--no-audit", "--no-fund"}, "package-lock.json"
		}
		if !depsStale(base, manifest.ModTime(), lock) {
			continue
		}
		// A dependency added to the manifest leaves the lockfile behind:
		// frozen installs refuse that, so update the lockfile instead.
		if st, err := os.Stat(filepath.Join(base, lock)); lock != "" && err == nil && manifest.ModTime().After(st.ModTime()) {
			argv = []string{argv[0], "install"}
			if argv[0] == "npm" {
				argv = append(argv, "--no-audit", "--no-fund")
			}
		}
		out = append(out, core.Command{Name: "install", Dir: c.Dir, Argv: argv})
	}
	out = append(out, goInstalls(dir, cmds)...)
	return append(out, browserInstalls(dir, cmds)...)
}

// goInstalls returns one `go mod tidy` per Go module the commands run in
// whose go.sum is missing or older than go.mod or a source file. Agents run
// headless and may not fetch modules, so a scaffold that imports packages
// absent from go.mod fails `go vet` with "no required module provides
// package" on every attempt. tidy is idempotent and only rewrites go.mod and
// go.sum, never source.
func goInstalls(dir string, cmds []core.Command) []core.Command {
	var out []core.Command
	seen := map[string]bool{}
	for _, c := range cmds {
		if len(c.Argv) == 0 || c.Argv[0] != "go" || seen[c.Dir] {
			continue
		}
		seen[c.Dir] = true
		base := filepath.Join(dir, c.Dir)
		mod, err := os.Stat(filepath.Join(base, "go.mod"))
		if err != nil || !goDepsStale(base, mod.ModTime()) {
			continue
		}
		argv := []string{"go", "mod", "tidy"}
		// Pin the declared version: an unpinned tidy picks the newest module
		// versions and silently raises the go directive (1.22 → 1.26) to fit
		// them. Pinned, an incompatible version fails with "requires go@X,
		// but Y is requested", which the agent fixes by pinning the module.
		if b, err := os.ReadFile(filepath.Join(base, "go.mod")); err == nil {
			if v := goDirective(string(b)); v != "" {
				argv = append(argv, "-go="+v)
			}
		}
		out = append(out, core.Command{Name: "install-go", Dir: c.Dir, Argv: argv})
	}
	return out
}

// goDepsStale reports whether go.sum is missing (with something to fetch) or
// older than go.mod or any Go source below base.
func goDepsStale(base string, mod time.Time) bool {
	sum, err := os.Stat(filepath.Join(base, "go.sum"))
	if err != nil {
		b, rerr := os.ReadFile(filepath.Join(base, "go.mod"))
		return rerr == nil && (hasRequire(string(b)) || hasGoImports(base, modulePath(string(b))))
	}
	if mod.After(sum.ModTime()) {
		return true
	}
	stale := false
	filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", ".git", ".foundry", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".go") {
			if info, ierr := d.Info(); ierr == nil && info.ModTime().After(sum.ModTime()) {
				stale = true
				return fs.SkipAll
			}
		}
		return nil
	})
	return stale
}

// hasGoImports reports whether any Go source below base imports a path with
// a dot in its first element, i.e. a third-party module.
func hasGoImports(base, own string) bool {
	found := false
	filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", ".git", ".foundry", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".go") {
			if b, rerr := os.ReadFile(path); rerr == nil {
				for _, m := range thirdParty.FindAllSubmatch(b, -1) {
					if imp := string(m[1]); own == "" || imp != own && !strings.HasPrefix(imp, own+"/") {
						found = true
						return fs.SkipAll
					}
				}
			}
		}
		return nil
	})
	return found
}

var thirdParty = regexp.MustCompile(`(?m)^\s*(?:import\s+)?(?:[\w._]+\s+)?"([a-z0-9-]+\.[a-z]+/[^"]+)"`)

var (
	modLine     = regexp.MustCompile(`(?m)^module\s+(\S+)`)
	goLine      = regexp.MustCompile(`(?m)^go\s+([0-9][0-9.]*)\s*$`)
	requireLine = regexp.MustCompile(`(?m)^\s*require\b`)
)

// modulePath is the module path declared in go.mod, "" when unreadable.
func modulePath(mod string) string {
	if m := modLine.FindStringSubmatch(mod); m != nil {
		return strings.Trim(m[1], `"`)
	}
	return ""
}

// goDirective is the go version declared in go.mod, "" when absent.
func goDirective(mod string) string {
	if m := goLine.FindStringSubmatch(mod); m != nil {
		return m[1]
	}
	return ""
}

// hasRequire reports whether go.mod has a require directive (not a comment).
func hasRequire(mod string) bool { return requireLine.MatchString(mod) }

// browserInstalls fetches the Chromium build Playwright needs for a script
// that runs its tests: node_modules alone does not include the browser, so a
// fresh machine fails every e2e test with "Executable doesn't exist". The
// command is a no-op when the browser is already there.
func browserInstalls(dir string, cmds []core.Command) []core.Command {
	var out []core.Command
	seen := map[string]bool{}
	for _, c := range cmds {
		if c.Name != "test:e2e" || seen[c.Dir] {
			continue
		}
		seen[c.Dir] = true
		if exists(filepath.Join(dir, c.Dir, "node_modules", "@playwright", "test")) || hasPlaywright(filepath.Join(dir, c.Dir)) {
			out = append(out, core.Command{Name: "install-browser", Dir: c.Dir, Argv: []string{"npx", "playwright", "install", "chromium"}})
		}
	}
	return out
}

// hasPlaywright reports whether the package declares @playwright/test, before
// its install has run.
func hasPlaywright(base string) bool {
	b, err := os.ReadFile(filepath.Join(base, "package.json"))
	return err == nil && strings.Contains(string(b), `"@playwright/test"`)
}

// depsStale reports whether base/node_modules is missing or older than the
// manifest or lockfile, judged by the marker each package manager writes.
func depsStale(base string, manifest time.Time, lock string) bool {
	var marker time.Time
	for _, m := range []string{".package-lock.json", ".modules.yaml", ".yarn-integrity", ".yarn-state.yml"} {
		if st, err := os.Stat(filepath.Join(base, "node_modules", m)); err == nil {
			marker = st.ModTime()
			break
		}
	}
	if marker.IsZero() {
		return true
	}
	if manifest.After(marker) {
		return true
	}
	if lock != "" {
		if st, err := os.Stat(filepath.Join(base, lock)); err == nil && st.ModTime().After(marker) {
			return true
		}
	}
	return false
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runOne(ctx context.Context, dir string, c core.Command) core.StepResult {
	ctx, cancel := context.WithTimeout(ctx, StepTimeout)
	defer cancel()
	st := core.StepResult{Name: c.Name, Command: strings.Join(c.Argv, " "), Dir: c.Dir}
	start := time.Now()
	cmd := exec.CommandContext(ctx, c.Argv[0], c.Argv[1:]...)
	cmd.Dir = filepath.Join(dir, c.Dir)
	cmd.Env = append(os.Environ(), "CI=true") // one-shot test runners, no watch mode
	// A background process the command started (a test server) keeps the
	// output pipe open; bound the wait so checks — serialized — never hang.
	cmd.WaitDelay = 5 * time.Second
	proc.Group(cmd)
	out := &tailWriter{max: maxOutput}
	cmd.Stdout, cmd.Stderr = out, out
	err := cmd.Run()
	proc.KillGroup(cmd) // a server it left must not hold the port for the next check
	st.Millis = time.Since(start).Milliseconds()
	st.Output = out.String()
	var ee *exec.ExitError
	switch {
	case err == nil, errors.Is(err, exec.ErrWaitDelay): // exited 0, a child kept the pipe
	case errors.As(err, &ee):
		st.ExitCode = ee.ExitCode()
		if st.ExitCode == 0 {
			st.ExitCode = -1
		}
	default:
		st.ExitCode = -1
		st.Output = tail(st.Output + "\n" + err.Error())
	}
	if ctx.Err() == context.DeadlineExceeded {
		st.ExitCode = -1
		st.Output = tail(st.Output + "\ntimeout")
	}
	return st
}

func tail(s string) string {
	if len(s) > maxOutput {
		return "…" + s[len(s)-maxOutput:]
	}
	return s
}

// tailWriter keeps only the last max bytes written, so a noisy command
// costs at most 2*max bytes of memory however much it prints. String reads
// the same as tail of the whole output.
type tailWriter struct {
	max   int
	buf   []byte
	total int64
}

func (w *tailWriter) Write(p []byte) (int, error) {
	w.total += int64(len(p))
	if w.buf == nil {
		w.buf = make([]byte, 0, 2*w.max)
	}
	if len(p) >= w.max {
		w.buf = append(w.buf[:0], p[len(p)-w.max:]...)
		return len(p), nil
	}
	if len(w.buf)+len(p) > 2*w.max {
		w.buf = append(w.buf[:0], w.buf[len(w.buf)-(w.max-len(p)):]...)
	}
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *tailWriter) String() string {
	if w.total > int64(w.max) {
		return "…" + string(w.buf[len(w.buf)-w.max:])
	}
	return string(w.buf)
}

// Snapshot records file modification times to detect what an agent changed.
type Snapshot map[string]time.Time

var skip = map[string]bool{"node_modules": true, ".git": true, core.StateDir: true, "dist": true, "vendor": true, ".next": true}

func Take(dir string) Snapshot {
	s := Snapshot{}
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != dir && skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if info, err := d.Info(); err == nil {
			rel, _ := filepath.Rel(dir, path)
			s[rel] = info.ModTime()
		}
		return nil
	})
	return s
}

// Changed lists files created or modified since before.
func Changed(dir string, before Snapshot) []string {
	var out []string
	for rel, t := range Take(dir) {
		if bt, ok := before[rel]; !ok || !bt.Equal(t) {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out
}

// Excerpt renders changed files for a reviewer, bounded in size.
func Excerpt(dir string, files []string, budget int) string {
	var b strings.Builder
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(&b, "--- %s (dihapus)\n", f)
			}
			continue
		}
		chunk := string(data)
		if len(chunk) > 4000 {
			chunk = chunk[:4000] + "\n…(dipotong)"
		}
		entry := fmt.Sprintf("--- %s\n%s\n", f, chunk)
		if b.Len()+len(entry) > budget {
			fmt.Fprintf(&b, "--- %s (tidak ditampilkan, batas ukuran)\n", f)
			continue
		}
		b.WriteString(entry)
	}
	if b.Len() == 0 {
		return "(tidak ada file yang berubah)"
	}
	return b.String()
}

// DiffExcerpt trims a unified diff for a reviewer: each file section is cut
// at 6000 bytes and sections past budget are listed by name only, so the
// reviewer always sees every changed file. Binary sections show no content.
func DiffExcerpt(patch string, budget int) string {
	const perFile = 6000
	var secs []string
	for _, line := range strings.SplitAfter(patch, "\n") {
		if strings.HasPrefix(line, "diff --git ") || len(secs) == 0 {
			secs = append(secs, "")
		}
		secs[len(secs)-1] += line
	}
	var b strings.Builder
	for _, sec := range secs {
		head, _, _ := strings.Cut(sec, "\n")
		name := strings.TrimPrefix(head, "diff --git ")
		switch {
		case strings.Contains(sec, "GIT binary patch") && !strings.Contains(sec, "omitted"):
			fmt.Fprintf(&b, "%s (file biner, isi tidak ditampilkan)\n", name)
		case b.Len()+min(len(sec), perFile) > budget:
			fmt.Fprintf(&b, "%s (tidak ditampilkan, batas ukuran)\n", name)
		case len(sec) > perFile:
			b.WriteString(sec[:perFile] + "\n…(dipotong)\n")
		default:
			b.WriteString(sec)
		}
	}
	if strings.TrimSpace(b.String()) == "" {
		return "(tidak ada perubahan)"
	}
	return b.String()
}
