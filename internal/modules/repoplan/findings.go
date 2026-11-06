package repoplan

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/handrawanwdev/foundry/internal/core"
)

// maxFanOut is how many units one unit may depend on before Foundry calls
// the coupling excessive.
const maxFanOut = 5

// maxHealthyDepth is the deepest path before the nesting looks like
// over-engineering.
const maxHealthyDepth = 8

// genericNames say what a folder is made of, not what it is for.
var genericNames = map[string]bool{
	"utils": true, "util": true, "helpers": true, "helper": true, "misc": true, "stuff": true,
	"temp": true, "tmp": true, "other": true, "others": true, "general": true, "manager": true, "managers": true,
}

// findings are the weaknesses Foundry measures itself, next to the ones the
// architect reported: circular dependencies, coupling, pass-through
// folders, generic names, duplicated responsibilities and deep nesting.
func findings(x *index, edges []nodeEdge, cyc []cycle, a *core.RepoAnalysis) []core.RepoFinding {
	var out []core.RepoFinding
	add := func(cat, sev, path, msg, fix string) {
		out = append(out, core.RepoFinding{Category: cat, Severity: sev, Path: path, Message: msg, Resolution: fix, Source: "computed"})
	}
	for _, c := range cyc {
		add("circular-dependency", "error", "", fmt.Sprintf("Dependensi melingkar antar-%s: %s.", c.between, strings.Join(c.members, " ↔ ")),
			"Putus satu arah lewat event, public interface, atau pindahkan kode bersama ke shared.")
	}
	for _, u := range a.Units {
		if u.FanOut >= maxFanOut {
			add("excessive-coupling", "warning", "", fmt.Sprintf("%s bergantung langsung pada %d unit lain.", u.Name, u.FanOut),
				"Kelompokkan lewat satu public interface atau ganti sebagian dengan event.")
		}
	}

	children := map[string][]core.RepoNode{}
	for _, n := range x.plan.Nodes {
		children[n.Parent()] = append(children[n.Parent()], n)
	}
	for _, n := range x.plan.Nodes {
		if !n.IsDir() {
			continue
		}
		if kids := children[n.Path]; len(kids) == 1 && kids[0].IsDir() && !n.Implied && n.Depth() > 1 {
			add("unnecessary-directory", "info", n.Path, fmt.Sprintf("%s hanya berisi satu folder, %s.", n.Path, kids[0].Path),
				"Gabungkan kedua level kecuali folder induk punya tanggung jawab sendiri.")
		}
	}
	for _, n := range x.plan.Nodes {
		if n.IsDir() && genericNames[strings.ToLower(n.BaseName())] {
			add("poor-naming", "warning", n.Path, fmt.Sprintf("Nama %s tidak menyebut tanggung jawab.", n.BaseName()),
				"Beri nama sesuai isinya, mis. nama kemampuan atau domain.")
		}
	}

	// Two folders that claim the same responsibility.
	seen := map[string]string{}
	var keys []string
	for _, n := range x.plan.Nodes {
		if !n.IsDir() || n.Implied {
			continue
		}
		key := foldText(n.Responsibility)
		if len(key) < 25 {
			continue
		}
		first, dup := seen[key]
		if !dup {
			seen[key] = n.Path
			keys = append(keys, key)
			continue
		}
		cat, fix := "duplication", "Gabungkan, atau bedakan tanggung jawab keduanya."
		if parentOf(first) == parentOf(n.Path) {
			cat = "responsibility-overlap"
		}
		add(cat, "warning", n.Path, fmt.Sprintf("%s dan %s menyatakan tanggung jawab yang sama.", first, n.Path), fix)
	}
	sort.Strings(keys)

	if a.MaxDepth > maxHealthyDepth {
		deepest := ""
		for _, n := range x.plan.Nodes {
			if n.Depth() == a.MaxDepth {
				deepest = n.Path
				break
			}
		}
		add("over-engineering", "info", deepest, fmt.Sprintf("Kedalaman path mencapai %d level.", a.MaxDepth),
			"Ratakan level yang hanya meneruskan ke level berikutnya.")
	}
	return out
}

func parentOf(path string) string {
	return core.RepoNode{Path: path}.Parent()
}

// foldText lowercases text and drops punctuation, so two phrasings of the
// same sentence compare equal.
func foldText(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
