// Package gitwt gives each parallel coding agent its own git worktree based on
// a snapshot of the project's current working tree, then carries the agent's
// diff back to the main working tree. It never touches the user's HEAD, index
// or refs.
package gitwt

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrConflict means a patch does not apply cleanly.
var ErrConflict = errors.New("patch tidak bisa diterapkan bersih")

// depDirs are dependency directories shared into worktrees via symlink.
var depDirs = []string{"node_modules", ".venv", "venv", "vendor"}

// Tree is one agent worktree.
type Tree struct {
	Dir   string // worktree path
	Base  string // commit the worktree currently starts from
	Repo  string
	links []string // relative paths of dependency symlinks created by Add
}

// git runs git -C dir with extra env and optional stdin; errors carry stderr.
func git(ctx context.Context, dir string, env []string, stdin []byte, args ...string) ([]byte, error) {
	full := append([]string{"-C", dir, "-c", "core.hooksPath=" + os.DevNull}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Env = append(cleanEnv(), env...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		return out.Bytes(), fmt.Errorf("git %s: %v: %s", args[0], err, msg)
	}
	return out.Bytes(), nil
}

// cleanEnv drops inherited variables that would redirect git elsewhere.
func cleanEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		switch strings.SplitN(kv, "=", 2)[0] {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_COMMON_DIR", "GIT_PREFIX":
			continue
		}
		env = append(env, kv)
	}
	return env
}

func canon(p string) string {
	p = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// Usable reports whether git exists and dir is the top level of a git work tree.
func Usable(dir string) bool {
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	out, err := git(context.Background(), abs, nil, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	top := strings.TrimSpace(string(out))
	return top != "" && canon(top) == canon(abs)
}

// Snapshot commits the current working tree of repo (tracked + untracked,
// honoring .gitignore), excluding the given top-level paths, without touching
// HEAD, the user's index or any ref. Returns the commit hash.
func Snapshot(ctx context.Context, repo string, exclude ...string) (string, error) {
	tmp, err := os.MkdirTemp("", "foundry-idx-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	env := []string{
		"GIT_INDEX_FILE=" + filepath.Join(tmp, "index"),
		"GIT_AUTHOR_NAME=Foundry", "GIT_AUTHOR_EMAIL=foundry@localhost",
		"GIT_COMMITTER_NAME=Foundry", "GIT_COMMITTER_EMAIL=foundry@localhost",
	}
	if hasHead(ctx, repo) {
		if _, err := git(ctx, repo, env, nil, "read-tree", "HEAD"); err != nil {
			return "", err
		}
	}
	if err := stage(ctx, repo, env, exclude); err != nil {
		return "", err
	}
	out, err := git(ctx, repo, env, nil, "write-tree")
	if err != nil {
		return "", err
	}
	tree := strings.TrimSpace(string(out))
	out, err = git(ctx, repo, env, nil, "commit-tree", tree, "-m", "foundry-snapshot")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func hasHead(ctx context.Context, repo string) bool {
	_, err := git(ctx, repo, nil, nil, "rev-parse", "--verify", "-q", "HEAD")
	return err == nil
}

// TreeOf returns the tree hash of commit.
func TreeOf(ctx context.Context, repo, commit string) (string, error) {
	out, err := git(ctx, repo, nil, nil, "rev-parse", commit+"^{tree}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Add creates a detached worktree of commit in a temp dir and symlinks
// untracked dependency dirs (node_modules, .venv, venv, vendor) from repo.
func Add(ctx context.Context, repo, commit, name string) (*Tree, error) {
	repo, err := filepath.Abs(repo)
	if err != nil {
		return nil, err
	}
	var rb [8]byte
	if _, err := rand.Read(rb[:]); err != nil {
		return nil, err
	}
	parent := filepath.Join(os.TempDir(), "foundry-wt", hex.EncodeToString(rb[:]))
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, err
	}
	if name == "" {
		name = "wt"
	}
	dir := filepath.Join(parent, filepath.Base(name))
	if _, err := git(ctx, repo, nil, nil, "worktree", "add", "--detach", dir, commit); err != nil {
		os.RemoveAll(parent)
		return nil, err
	}
	t := &Tree{Dir: dir, Base: commit, Repo: repo}
	for _, rel := range findDeps(repo) {
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		if fi, err := os.Stat(filepath.Dir(dst)); err != nil || !fi.IsDir() {
			continue
		}
		if err := os.Symlink(filepath.Join(repo, filepath.FromSlash(rel)), dst); err != nil {
			continue
		}
		t.links = append(t.links, rel)
	}
	return t, nil
}

// findDeps lists dependency dirs in repo at depth 0..2 (slash-separated).
func findDeps(repo string) []string {
	var found []string
	var walk func(rel string, depth int)
	walk = func(rel string, depth int) {
		entries, err := os.ReadDir(filepath.Join(repo, filepath.FromSlash(rel)))
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			n := e.Name()
			p := n
			if rel != "" {
				p = rel + "/" + n
			}
			if isDep(n) {
				found = append(found, p)
				continue
			}
			if n == ".git" || n == ".foundry" || depth >= 2 {
				continue
			}
			walk(p, depth+1)
		}
	}
	walk("", 0)
	return found
}

func isDep(n string) bool {
	for _, d := range depDirs {
		if n == d {
			return true
		}
	}
	return false
}

// stage adds everything in dir to the index, then drops the skipped paths.
// An ":(exclude)" pathspec is not used: git add fails when it names an
// ignored path (".foundry/" is usually gitignored).
func stage(ctx context.Context, dir string, env, skip []string) error {
	if _, err := git(ctx, dir, env, nil, "add", "-A", "--", "."); err != nil {
		return err
	}
	if len(skip) == 0 {
		return nil
	}
	args := append([]string{"rm", "-r", "-q", "--cached", "--ignore-unmatch", "--"}, skip...)
	_, err := git(ctx, dir, env, nil, args...)
	return err
}

// Diff stages everything in the worktree (except dependency links) and
// returns the binary patch against Base plus the sorted changed paths.
func (t *Tree) Diff(ctx context.Context) (patch []byte, files []string, err error) {
	if err := stage(ctx, t.Dir, nil, t.links); err != nil {
		return nil, nil, err
	}
	common := []string{"diff", "--cached", "--no-renames", "--no-ext-diff", "--no-color", "--no-textconv"}
	names, err := git(ctx, t.Dir, nil, nil, append(common, "--name-only", "-z", t.Base)...)
	if err != nil {
		return nil, nil, err
	}
	for _, n := range strings.Split(string(names), "\x00") {
		if n != "" {
			files = append(files, n)
		}
	}
	if len(files) == 0 {
		return nil, nil, nil
	}
	sort.Strings(files)
	patch, err = git(ctx, t.Dir, nil, nil, append(common, "--binary", "--src-prefix=a/", "--dst-prefix=b/", t.Base)...)
	if err != nil {
		return nil, nil, err
	}
	return patch, files, nil
}

// Rebase resets the worktree to commit and re-applies patch.
func (t *Tree) Rebase(ctx context.Context, commit string, patch []byte) error {
	if _, err := git(ctx, t.Dir, nil, nil, "reset", "-q", "--hard", commit); err != nil {
		return err
	}
	args := []string{"clean", "-ffdq"}
	for _, l := range t.links {
		args = append(args, "-e", "/"+l)
	}
	if _, err := git(ctx, t.Dir, nil, nil, args...); err != nil {
		return err
	}
	if len(patch) > 0 {
		if _, err := git(ctx, t.Dir, nil, patch, "apply", "--binary", "--index"); err != nil {
			return fmt.Errorf("%w: %v", ErrConflict, err)
		}
	}
	t.Base = commit
	return nil
}

// Apply applies patch to repo's working tree only; on conflict nothing changes.
func Apply(ctx context.Context, repo string, patch []byte) error {
	if len(patch) == 0 {
		return nil
	}
	if _, err := git(ctx, repo, nil, patch, "apply", "--binary", "--check"); err != nil {
		return fmt.Errorf("%w: %v", ErrConflict, err)
	}
	_, err := git(ctx, repo, nil, patch, "apply", "--binary")
	return err
}

// Remove deletes the worktree and its temp parent; symlink targets are never
// touched (links are removed first).
func (t *Tree) Remove() {
	for _, l := range t.links {
		p := filepath.Join(t.Dir, filepath.FromSlash(l))
		if fi, err := os.Lstat(p); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			os.Remove(p)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	git(ctx, t.Repo, nil, nil, "worktree", "remove", "--force", t.Dir)
	os.RemoveAll(filepath.Dir(t.Dir))
	git(ctx, t.Repo, nil, nil, "worktree", "prune")
}

// Prune runs `git worktree prune` to clean stale entries from crashed runs.
func Prune(repo string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	git(ctx, repo, nil, nil, "worktree", "prune")
}
