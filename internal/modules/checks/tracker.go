package checks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	hashLimit    = 1 << 20  // files above this are tracked by mtime+size only
	contentLimit = 64 << 10 // files above this keep no "before" copy
	contentTotal = 32 << 20 // total "before" copies kept in memory
	maxTracked   = 20000
)

type fileState struct {
	mod     time.Time
	size    int64
	hash    string // "" when the file is above hashLimit or unreadable
	content []byte // "before" copy for diffing; nil when not kept
	binary  bool
}

// Baseline is the state of a working tree at one moment, taken without
// touching it. Compare reports exactly what changed since, including renames,
// deletions and binary files, so a task in a shared tree can be reviewed on
// its own changes.
type Baseline struct {
	dir   string
	files map[string]fileState
	// Truncated is set when the tree had more than maxTracked files; changes
	// beyond the cap are not seen, and reviewers are told so.
	Truncated bool
}

// Capture records the tree under dir (same ignore list as Take).
func Capture(dir string) *Baseline {
	b := &Baseline{dir: dir, files: map[string]fileState{}}
	kept := 0
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
		if len(b.files) >= maxTracked {
			b.Truncated = true
			return filepath.SkipAll
		}
		rel, _ := filepath.Rel(dir, path)
		st, ok := readState(path, kept < contentTotal)
		if !ok {
			return nil
		}
		kept += len(st.content)
		b.files[filepath.ToSlash(rel)] = st
		return nil
	})
	return b
}

func readState(path string, keep bool) (fileState, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fileState{}, false
	}
	st := fileState{mod: info.ModTime(), size: info.Size()}
	if st.size > hashLimit {
		return st, true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return st, true
	}
	sum := sha256.Sum256(data)
	st.hash = hex.EncodeToString(sum[:8])
	st.binary = isBinary(data)
	if keep && !st.binary && len(data) <= contentLimit {
		st.content = data
	}
	return st, true
}

func isBinary(data []byte) bool {
	return bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0
}

// Kind is how a file changed.
type Kind string

const (
	Added    Kind = "added"
	Modified Kind = "modified"
	Removed  Kind = "deleted"
	Renamed  Kind = "renamed"
)

// Change is one changed file. Diff is a unified diff of the text; it is empty
// for binary files, renames without edits, and files too large to diff
// (Incomplete says why the reviewer sees less than the full change).
type Change struct {
	Path       string
	From       string // previous path, for Renamed
	Kind       Kind
	Binary     bool
	Incomplete bool
	Diff       string
}

// Compare lists what changed under dir since the baseline, sorted by path.
func (b *Baseline) Compare(dir string) []Change {
	now := Capture(dir)
	var added, gone []string
	var out []Change
	for rel, st := range now.files {
		old, ok := b.files[rel]
		switch {
		case !ok:
			added = append(added, rel)
		case old.hash != "" && old.hash == st.hash:
			// identical content (touched only)
		case old.hash == "" && old.size == st.size && old.mod.Equal(st.mod):
			// large file, untouched
		default:
			out = append(out, b.modified(dir, rel, old, st))
		}
	}
	for rel := range b.files {
		if _, ok := now.files[rel]; !ok {
			gone = append(gone, rel)
		}
	}
	sort.Strings(added)
	sort.Strings(gone)
	// A deleted + added pair with the same content is a rename.
	byHash := map[string][]string{}
	for _, g := range gone {
		if h := b.files[g].hash; h != "" {
			byHash[h] = append(byHash[h], g)
		}
	}
	used := map[string]bool{}
	for _, a := range added {
		st := now.files[a]
		if h := st.hash; h != "" && len(byHash[h]) > 0 {
			from := byHash[h][0]
			byHash[h] = byHash[h][1:]
			used[from] = true
			out = append(out, Change{Path: a, From: from, Kind: Renamed, Binary: st.binary})
			continue
		}
		out = append(out, b.created(dir, a, st))
	}
	for _, g := range gone {
		if !used[g] {
			out = append(out, Change{Path: g, Kind: Removed, Binary: b.files[g].binary})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func (b *Baseline) modified(dir, rel string, old, now fileState) Change {
	c := Change{Path: rel, Kind: Modified, Binary: now.binary || old.binary}
	switch {
	case c.Binary:
	case old.content == nil || now.hash == "":
		c.Incomplete = true
	default:
		cur, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			c.Incomplete = true
			break
		}
		c.Diff = textDiff(rel, old.content, cur)
		c.Incomplete = c.Diff == ""
	}
	return c
}

func (b *Baseline) created(dir, rel string, st fileState) Change {
	c := Change{Path: rel, Kind: Added, Binary: st.binary}
	if st.binary {
		return c
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		c.Incomplete = true
		return c
	}
	c.Diff = textDiff(rel, nil, data)
	return c
}

// textDiff is a unified diff of old → cur using git (a Foundry requirement
// already), with headers normalised to a/ and b/ paths. Nothing is written
// inside the project.
func textDiff(rel string, old, cur []byte) string {
	tmp, err := os.MkdirTemp("", "foundry-diff-*")
	if err != nil {
		return ""
	}
	defer os.RemoveAll(tmp)
	a, c := filepath.Join(tmp, "a"), filepath.Join(tmp, "b")
	if os.WriteFile(a, old, 0o600) != nil || os.WriteFile(c, cur, 0o600) != nil {
		return ""
	}
	cmd := exec.Command("git", "diff", "--no-index", "--no-color", "--no-ext-diff", "--no-textconv", "-U3", "--", a, c)
	out, err := cmd.Output()
	if ee, ok := err.(*exec.ExitError); err != nil && (!ok || ee.ExitCode() != 1) { // 1 = files differ
		return ""
	}
	_, hunks, found := strings.Cut(string(out), "\n@@")
	if !found {
		return ""
	}
	from := "a/" + rel
	if old == nil {
		from = "/dev/null"
	}
	return fmt.Sprintf("diff --git a/%s b/%s\n--- %s\n+++ b/%s\n@@%s", rel, rel, from, rel, hunks)
}

// Render turns changes into one review text in git-diff form that
// DiffExcerpt can bound. Files without a text diff get an explicit line, so
// the reviewer is never shown less than the truth without being told.
func Render(changes []Change, truncated bool) string {
	var b strings.Builder
	for _, c := range changes {
		switch {
		case c.Kind == Removed:
			fmt.Fprintf(&b, "diff --git a/%s b/%s\ndeleted file\n", c.Path, c.Path)
		case c.Kind == Renamed:
			fmt.Fprintf(&b, "diff --git a/%s b/%s\nrename from %s\nrename to %s\n", c.From, c.Path, c.From, c.Path)
		case c.Binary:
			fmt.Fprintf(&b, "diff --git a/%s b/%s\n%s binary file (GIT binary patch omitted)\n", c.Path, c.Path, c.Kind)
		case c.Incomplete:
			fmt.Fprintf(&b, "diff --git a/%s b/%s\n%s file; diff not available (file too large or unreadable); read the file itself\n", c.Path, c.Path, c.Kind)
		default:
			b.WriteString(c.Diff)
			if !strings.HasSuffix(c.Diff, "\n") {
				b.WriteByte('\n')
			}
		}
	}
	if truncated {
		b.WriteString("NOTE: project has too many files; changes beyond the first 20000 files are not listed.\n")
	}
	return b.String()
}
