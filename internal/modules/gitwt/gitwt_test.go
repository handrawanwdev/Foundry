package gitwt

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git tidak terpasang")
	}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=T", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Env = cleanEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// newRepo makes a repo with one commit (a.txt, b.txt, .gitignore).
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "init", "-q")
	write(t, dir, "a.txt", "a1\na2\na3\n")
	write(t, dir, "b.txt", "b1\n")
	write(t, dir, ".gitignore", "node_modules/\n*.log\n")
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func addTree(t *testing.T, repo string) *Tree {
	t.Helper()
	ctx := context.Background()
	snap, err := Snapshot(ctx, repo, ".foundry")
	if err != nil {
		t.Fatal(err)
	}
	wt, err := Add(ctx, repo, snap, "task-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(wt.Remove)
	return wt
}

func TestUsable(t *testing.T) {
	needGit(t)
	if Usable(t.TempDir()) {
		t.Error("plain dir harus false")
	}
	repo := newRepo(t)
	if !Usable(repo) {
		t.Error("repo top harus true")
	}
	write(t, repo, "sub/x.txt", "x")
	if Usable(filepath.Join(repo, "sub")) {
		t.Error("subdir harus false")
	}
	empty := t.TempDir()
	run(t, empty, "init", "-q")
	if !Usable(empty) {
		t.Error("repo tanpa commit harus true")
	}
}

func TestSnapshot(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	repo := newRepo(t)
	write(t, repo, "a.txt", "changed\n")
	write(t, repo, "new.txt", "new\n")
	write(t, repo, ".foundry/x", "secret\n")
	write(t, repo, "debug.log", "log\n")
	write(t, repo, "b.txt", "b1 staged\n")
	run(t, repo, "add", "b.txt")

	head := run(t, repo, "rev-parse", "HEAD")
	status := run(t, repo, "status", "--porcelain")
	index := run(t, repo, "ls-files", "-s")
	refs := run(t, repo, "for-each-ref")

	snap, err := Snapshot(ctx, repo, ".foundry")
	if err != nil {
		t.Fatal(err)
	}
	files := strings.Fields(run(t, repo, "ls-tree", "-r", "--name-only", snap))
	want := []string{".gitignore", "a.txt", "b.txt", "new.txt"}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files = %v, want %v", files, want)
	}
	if got := run(t, repo, "show", snap+":a.txt"); got != "changed\n" {
		t.Errorf("a.txt = %q", got)
	}
	if run(t, repo, "rev-parse", "HEAD") != head || run(t, repo, "status", "--porcelain") != status ||
		run(t, repo, "ls-files", "-s") != index || run(t, repo, "for-each-ref") != refs {
		t.Error("snapshot mengubah HEAD/index/status/refs")
	}

	snap2, err := Snapshot(ctx, repo, ".foundry")
	if err != nil {
		t.Fatal(err)
	}
	t1, _ := TreeOf(ctx, repo, snap)
	t2, _ := TreeOf(ctx, repo, snap2)
	if t1 == "" || t1 != t2 {
		t.Errorf("tree beda: %q vs %q", t1, t2)
	}
}

func TestAddDiffApply(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	repo := newRepo(t)
	write(t, repo, "node_modules/pkg/index.js", "dep\n")
	write(t, repo, "apps/web/node_modules/x.js", "dep2\n")
	write(t, repo, "apps/web/package.json", "{}\n")
	write(t, repo, "apps/api/vendor/m.log", "orphan\n") // parent tidak ada di worktree
	wt := addTree(t, repo)

	for _, rel := range []string{"node_modules", "apps/web/node_modules"} {
		fi, err := os.Lstat(filepath.Join(wt.Dir, filepath.FromSlash(rel)))
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s bukan symlink: %v", rel, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(wt.Dir, "apps", "api")); !os.IsNotExist(err) {
		t.Error("apps/api tidak boleh dibuat")
	}
	if got := read(t, wt.Dir, "node_modules/pkg/index.js"); got != "dep\n" {
		t.Errorf("isi symlink = %q", got)
	}

	write(t, wt.Dir, "a.txt", "a1\nA2\na3\n")
	write(t, wt.Dir, "src/new.go", "package src\n")
	patch, files, err := wt.Diff(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.txt", "src/new.go"}; !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %v, want %v", files, want)
	}
	if err := Apply(ctx, repo, patch); err != nil {
		t.Fatal(err)
	}
	if read(t, repo, "a.txt") != "a1\nA2\na3\n" || read(t, repo, "src/new.go") != "package src\n" {
		t.Error("repo tidak sesuai worktree")
	}
	if err := Apply(ctx, repo, nil); err != nil {
		t.Error(err)
	}
}

func TestDiffEmpty(t *testing.T) {
	needGit(t)
	wt := addTree(t, newRepo(t))
	patch, files, err := wt.Diff(context.Background())
	if err != nil || len(patch) != 0 || files != nil {
		t.Errorf("patch=%q files=%v err=%v", patch, files, err)
	}
}

func TestApplyConflict(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	repo := newRepo(t)
	wt := addTree(t, repo)
	write(t, wt.Dir, "a.txt", "a1\nAGENT\na3\n")
	patch, _, err := wt.Diff(ctx)
	if err != nil {
		t.Fatal(err)
	}
	write(t, repo, "a.txt", "a1\nUSER\na3\n")
	err = Apply(ctx, repo, patch)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	if got := read(t, repo, "a.txt"); got != "a1\nUSER\na3\n" {
		t.Errorf("repo berubah: %q", got)
	}
}

func TestRebase(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	repo := newRepo(t)
	write(t, repo, "node_modules/pkg/index.js", "dep\n")
	wt := addTree(t, repo)
	write(t, wt.Dir, "a.txt", "a1\nAGENT\na3\n")
	patch, _, err := wt.Diff(ctx)
	if err != nil {
		t.Fatal(err)
	}
	write(t, repo, "b.txt", "USER\n")
	snap, err := Snapshot(ctx, repo, ".foundry")
	if err != nil {
		t.Fatal(err)
	}
	if err := wt.Rebase(ctx, snap, patch); err != nil {
		t.Fatal(err)
	}
	if wt.Base != snap {
		t.Error("Base tidak diperbarui")
	}
	if read(t, wt.Dir, "a.txt") != "a1\nAGENT\na3\n" || read(t, wt.Dir, "b.txt") != "USER\n" {
		t.Error("worktree tidak berisi kedua perubahan")
	}
	if fi, err := os.Lstat(filepath.Join(wt.Dir, "node_modules")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Error("symlink hilang setelah rebase")
	}
	_, files, err := wt.Diff(ctx)
	if err != nil || !reflect.DeepEqual(files, []string{"a.txt"}) {
		t.Errorf("files=%v err=%v", files, err)
	}

	// Konflik saat rebase.
	write(t, repo, "a.txt", "a1\nUSER\na3\n")
	snap2, _ := Snapshot(ctx, repo, ".foundry")
	if err := wt.Rebase(ctx, snap2, patch); !errors.Is(err, ErrConflict) {
		t.Errorf("err = %v, want ErrConflict", err)
	}
}

func TestRemove(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	repo := newRepo(t)
	write(t, repo, "node_modules/pkg/index.js", "dep\n")
	snap, err := Snapshot(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := Add(ctx, repo, snap, "rm")
	if err != nil {
		t.Fatal(err)
	}
	wt.Remove()
	if _, err := os.Lstat(wt.Dir); !os.IsNotExist(err) {
		t.Error("worktree masih ada")
	}
	if _, err := os.Lstat(filepath.Dir(wt.Dir)); !os.IsNotExist(err) {
		t.Error("parent temp masih ada")
	}
	if read(t, repo, "node_modules/pkg/index.js") != "dep\n" {
		t.Error("target symlink terhapus")
	}
	if strings.Contains(run(t, repo, "worktree", "list"), wt.Dir) {
		t.Error("worktree masih terdaftar")
	}
	Prune(repo)
}

func TestZeroCommitRepo(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	repo := t.TempDir()
	run(t, repo, "init", "-q")
	write(t, repo, "a.txt", "a\n")
	wt := addTree(t, repo)
	if read(t, wt.Dir, "a.txt") != "a\n" {
		t.Fatal("snapshot tidak berisi a.txt")
	}
	write(t, wt.Dir, "a.txt", "A\n")
	write(t, wt.Dir, "b.txt", "b\n")
	patch, files, err := wt.Diff(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, []string{"a.txt", "b.txt"}) {
		t.Fatalf("files = %v", files)
	}
	if err := Apply(ctx, repo, patch); err != nil {
		t.Fatal(err)
	}
	if read(t, repo, "a.txt") != "A\n" || read(t, repo, "b.txt") != "b\n" {
		t.Error("apply gagal")
	}
	if hasHead(ctx, repo) {
		t.Error("HEAD tidak boleh dibuat")
	}
}

// An excluded path that .gitignore already ignores must not fail the add.
func TestSnapshotExcludeIgnored(t *testing.T) {
	needGit(t)
	repo := newRepo(t)
	write(t, repo, ".gitignore", "node_modules/\n.foundry/\n")
	write(t, repo, ".foundry/project.json", "{}\n")
	snap, err := Snapshot(context.Background(), repo, ".foundry")
	if err != nil {
		t.Fatal(err)
	}
	if files := run(t, repo, "ls-tree", "-r", "--name-only", snap); strings.Contains(files, ".foundry") {
		t.Fatalf("files = %s", files)
	}
}

// Gitignored .env files are linked in, never carried in the diff.
func TestAddLinksEnv(t *testing.T) {
	needGit(t)
	repo := newRepo(t)
	write(t, repo, ".gitignore", ".env\n.env.*\n")
	write(t, repo, ".env", "DB=x\n")
	write(t, repo, ".env.test", "DB=t\n")
	write(t, repo, "a.txt", "a1\n")
	wt := addTree(t, repo)
	for _, f := range []string{".env", ".env.test"} {
		fi, err := os.Lstat(filepath.Join(wt.Dir, f))
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s bukan symlink: %v", f, err)
		}
	}
	if read(t, wt.Dir, ".env") != "DB=x\n" {
		t.Error("isi .env salah")
	}
	write(t, wt.Dir, "a.txt", "A\n")
	_, files, err := wt.Diff(context.Background())
	if err != nil || !reflect.DeepEqual(files, []string{"a.txt"}) {
		t.Fatalf("files=%v err=%v", files, err)
	}
	wt.Remove()
	if read(t, repo, ".env") != "DB=x\n" {
		t.Error(".env project terhapus")
	}
}

// Prune removes worktrees a crashed run left behind, links first.
func TestPruneOrphans(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	repo := newRepo(t)
	write(t, repo, "node_modules/pkg/index.js", "dep\n")
	snap, err := Snapshot(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := Add(ctx, repo, snap, "orphan")
	if err != nil {
		t.Fatal(err)
	}
	os.Symlink(filepath.Join(repo, ".git"), filepath.Join(wt.Dir, "state")) // like the .foundry link
	Prune(repo)                                                             // no Remove: the run "crashed"
	if _, err := os.Lstat(filepath.Dir(wt.Dir)); !os.IsNotExist(err) {
		t.Fatal("worktree yatim masih ada")
	}
	if strings.Contains(run(t, repo, "worktree", "list"), wt.Dir) {
		t.Error("worktree masih terdaftar")
	}
	if read(t, repo, "node_modules/pkg/index.js") != "dep\n" || !Usable(repo) {
		t.Error("isi project ikut terhapus")
	}
}

// Merge keeps both sides where it can and leaves conflict markers where it
// cannot, so an agent resolves them in place.
func TestMergeLeavesMarkers(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	repo := newRepo(t)
	wt := addTree(t, repo)
	write(t, wt.Dir, "a.txt", "a1\nAGENT\na3\n")
	write(t, wt.Dir, "c.txt", "new")
	patch, _, err := wt.Diff(ctx)
	if err != nil {
		t.Fatal(err)
	}
	write(t, repo, "a.txt", "a1\nUSER\na3\n")
	snap, err := Snapshot(ctx, repo, ".foundry")
	if err != nil {
		t.Fatal(err)
	}
	files, err := wt.Merge(ctx, snap, patch)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, []string{"a.txt"}) || wt.Base != snap {
		t.Fatalf("files=%v base=%s", files, wt.Base)
	}
	if a := read(t, wt.Dir, "a.txt"); !strings.Contains(a, "<<<<<<<") || !strings.Contains(a, "AGENT") || !strings.Contains(a, "USER") {
		t.Errorf("a.txt = %q", a)
	}
	if read(t, wt.Dir, "c.txt") != "new" {
		t.Error("clean part of the patch lost")
	}
	if _, files, err = wt.Diff(ctx); err != nil || !reflect.DeepEqual(files, []string{"a.txt", "c.txt"}) {
		t.Errorf("diff files=%v err=%v", files, err)
	}
}

// Drop restores changed files to Base and deletes files Base lacks.
func TestDrop(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	repo := newRepo(t)
	wt := addTree(t, repo)
	write(t, wt.Dir, "b.txt", "AGENT")
	write(t, wt.Dir, "x/new.txt", "new")
	write(t, wt.Dir, "c.txt", "keep")
	if _, _, err := wt.Diff(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(wt.FileDiff(ctx, "b.txt"), "+AGENT") {
		t.Errorf("FileDiff = %q", wt.FileDiff(ctx, "b.txt"))
	}
	if err := wt.Drop(ctx, []string{"b.txt", "x/new.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wt.Dir, "x", "new.txt")); err == nil {
		t.Error("new file not deleted")
	}
	_, files, err := wt.Diff(ctx)
	if err != nil || !reflect.DeepEqual(files, []string{"c.txt"}) {
		t.Errorf("files=%v err=%v", files, err)
	}
	if top := wt.TopLevel(ctx); !top["a.txt"] || !top[".gitignore"] || top["c.txt"] {
		t.Errorf("TopLevel = %v", top)
	}
}
