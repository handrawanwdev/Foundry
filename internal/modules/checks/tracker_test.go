package checks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func byPath(cs []Change) map[string]Change {
	m := map[string]Change{}
	for _, c := range cs {
		m[c.Path] = c
	}
	return m
}

func TestCompareReportsEveryKind(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "mod.go", "package a\n\nfunc One() {}\nfunc Two() {}\n")
	write(t, dir, "gone.go", "package gone\n")
	write(t, dir, "old/name.go", "package moved // same content\n")
	write(t, dir, "logo.png", "\x89PNG\x00\x00binary")
	write(t, dir, "same.go", "package same\n")
	base := Capture(dir)

	write(t, dir, "mod.go", "package a\n\nfunc One() {}\nfunc Two() { println(2) }\n")
	os.Remove(filepath.Join(dir, "gone.go"))
	os.MkdirAll(filepath.Join(dir, "new"), 0o755)
	os.Rename(filepath.Join(dir, "old/name.go"), filepath.Join(dir, "new/name.go"))
	write(t, dir, "added.go", "package added\n")
	write(t, dir, "logo.png", "\x89PNG\x00\x00changed")
	write(t, dir, "same.go", "package same\n") // touched, identical

	got := byPath(base.Compare(dir))
	if len(got) != 5 {
		t.Fatalf("want 5 changes (touched-only file is not one), got %d: %+v", len(got), got)
	}
	if c := got["mod.go"]; c.Kind != Modified || !strings.Contains(c.Diff, "+func Two() { println(2) }") || !strings.Contains(c.Diff, "-func Two() {}") || strings.Contains(c.Diff, "+func One()") {
		t.Fatalf("modified must show only changed lines: %+v", c)
	}
	if c := got["gone.go"]; c.Kind != Removed {
		t.Fatalf("gone = %+v", c)
	}
	if c := got["new/name.go"]; c.Kind != Renamed || c.From != "old/name.go" || c.Diff != "" {
		t.Fatalf("rename = %+v", c)
	}
	if c := got["added.go"]; c.Kind != Added || !strings.Contains(c.Diff, "+package added") || !strings.Contains(c.Diff, "--- /dev/null") {
		t.Fatalf("added = %+v", c)
	}
	if c := got["logo.png"]; !c.Binary || c.Diff != "" {
		t.Fatalf("binary = %+v", c)
	}
	txt := Render(base.Compare(dir), false)
	for _, want := range []string{"deleted file", "rename from old/name.go", "binary file", "+package added"} {
		if !strings.Contains(txt, want) {
			t.Fatalf("render missing %q:\n%s", want, txt)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "mod.go.orig")); err == nil {
		t.Fatal("tree must stay untouched")
	}
}

// Another agent edits its own files while this task works: the baseline
// reports both, and ownership filtering (pipeline) separates them.
func TestCompareSeesConcurrentEditsFromOthers(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "mine/a.go", "package mine\n")
	write(t, dir, "theirs/b.go", "package theirs\n")
	base := Capture(dir)
	write(t, dir, "mine/a.go", "package mine // task\n")
	write(t, dir, "theirs/b.go", "package theirs // other agent\n")
	got := byPath(base.Compare(dir))
	if got["mine/a.go"].Kind != Modified || got["theirs/b.go"].Kind != Modified {
		t.Fatalf("both edits must be reported so the owner filter can split them: %+v", got)
	}
}

func TestCompareFlagsIncompleteDiffs(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("line of text\n", 10000) // > contentLimit: no "before" copy
	write(t, dir, "big.txt", big)
	huge := strings.Repeat("x", hashLimit+10)
	write(t, dir, "huge.bin.txt", huge)
	base := Capture(dir)
	write(t, dir, "big.txt", big+"one more\n")
	write(t, dir, "huge.bin.txt", huge+"y")
	got := byPath(base.Compare(dir))
	for _, name := range []string{"big.txt", "huge.bin.txt"} {
		c := got[name]
		if c.Kind != Modified || !c.Incomplete || c.Diff != "" {
			t.Fatalf("%s must be flagged incomplete, not silently dropped: %+v", name, c)
		}
	}
	txt := Render(base.Compare(dir), true)
	if !strings.Contains(txt, "diff not available") || !strings.Contains(txt, "too many files") {
		t.Fatalf("reviewer must be told the diff is partial:\n%s", txt)
	}
	if len(base.Compare(t.TempDir())) != 2 { // everything "deleted" from an empty tree
		t.Fatal("deleted files must be reported even without content")
	}
}

func TestDiffExcerptKeepsEveryFile(t *testing.T) {
	big := strings.Repeat("+line\n", 3000)
	patch := "diff --git a/a.go b/a.go\n@@\n+small\ndiff --git a/b.go b/b.go\n@@\n" + big + "diff --git a/c.go b/c.go\n@@\n+tail\n" +
		"diff --git a/logo.png b/logo.png\nGIT binary patch\nliteral 10\nzcmV\n"
	got := DiffExcerpt(patch, 6050)
	for _, want := range []string{"a/a.go b/a.go", "+small", "(dipotong)", "c.go", "logo.png (file biner"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%.400s", want, got)
		}
	}
	if strings.Contains(got, "+tail") || strings.Contains(got, "zcmV") || len(got) > 6600 {
		t.Fatalf("over-budget file must be name only, binary hidden, bounded (%d):\n%.300s", len(got), got)
	}
	if strings.Count(got, "diff --git a/a.go") != 1 {
		t.Fatalf("section headers must not repeat:\n%.300s", got)
	}
	if DiffExcerpt("", 100) != "(tidak ada perubahan)" {
		t.Fatal("empty diff")
	}
}
