package work

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
)

// KindStructure turns a natural-language structure change into edits of
// the Structure IR. The result is a proposal: the user sees its impact and
// applies or cancels it.
const KindStructure = "structure"

const maxStructureRequest = 6000

var ErrStructureUnclear = errors.New("AI tidak menemukan perubahan struktur untuk permintaan ini")

func CleanStructureRequest(text string) (string, error) {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return "", errors.New("tuliskan perubahan struktur yang diinginkan")
	case len(text) > maxStructureRequest:
		return "", fmt.Errorf("permintaan terlalu panjang (maks. %d karakter)", maxStructureRequest)
	}
	return text, nil
}

func StructurePrompt(p *core.Project) string {
	s := p.Structure
	ir := s.IR
	var b strings.Builder
	fmt.Fprintf(&b, `You edit a product's project structure from the user's request.
The user decides the structure. Change ONLY what the request names or clearly implies; keep everything
else. Boundaries follow business domains, never endpoints or CRUD operations (no "create-booking").
Prefer the simplest structure that meets the request; do not add services, workers or packages the
user did not ask for.
The structure has two halves: the PARTS (folders: apps, services, modules, workers, packages) and the
RULES (conventions every part follows: internal module layout and layers, file responsibilities,
interfaces/ports, dependencies between modules, naming, tests, shared code). A request that states a
standard, convention, guideline or principle (even a long numbered list) is a change of RULES: turn each
point into one short imperative Indonesian rule, keep the user's meaning and concrete names (folders,
layers), merge points that say the same thing, and skip what a current rule already says. A request may
change parts and rules together. Return no ops only when the request has nothing to do with the project
structure.

CURRENT STRUCTURE (type %s):
%s
`, ir.Type, structure.Tree(ir))
	section := func(label string, ps []core.StructurePart) {
		for _, x := range ps {
			fmt.Fprintf(&b, "- %s %s", label, x.Name)
			if len(x.Modules) > 0 {
				fmt.Fprintf(&b, " (modules: %s)", strings.Join(x.Modules, ", "))
			}
			b.WriteString("\n")
		}
	}
	section("app", ir.Apps)
	section("service", ir.Services)
	section("worker", ir.Workers)
	section("package", ir.Packages)
	if len(ir.Rules) > 0 {
		b.WriteString("\nCURRENT RULES:\n")
		for _, r := range ir.Rules {
			fmt.Fprintf(&b, "- %s\n", r)
		}
	}
	if bp := p.Blueprint; bp != nil && len(bp.Modules) > 0 {
		b.WriteString("\nDOMAINS FROM DISCOVERY:\n")
		for _, m := range bp.Modules {
			fmt.Fprintf(&b, "- %s: %s\n", m.ID, m.Responsibility)
		}
	}
	if st := p.Stack; st != nil {
		fmt.Fprintf(&b, "\nTECHNOLOGY: architecture %s, repository %s, backend %s\n",
			st.Choice(core.TechArchitecture), st.Choice(core.TechRepository), st.Choice(core.TechBackend))
	}
	fmt.Fprintf(&b, "\nUSER REQUEST:\n%s\n", s.Request)
	b.WriteString(`
Answer with one JSON object only:
{"ops":[{"op":"move","kind":"service","name":"reporting","to":"operations","reason":"<why, Indonesian>"}],
 "note":"<one Indonesian sentence on what you changed, or why nothing>"}
Ops (names are lowercase folder names):
- {"op":"type","name":"simple|modular|monorepo|microservices"} regroups the same domains for that type
- {"op":"add","kind":"app|service|worker|package","name":"x"} ; {"op":"add","kind":"module","name":"x","to":"<service>"}
- {"op":"remove","kind":"app|service|module|worker|package","name":"x"}
- {"op":"rename","kind":"...","name":"x","to":"y"}
- {"op":"move","kind":"module","name":"x","to":"<service>"} ; {"op":"move","kind":"service","name":"x","to":"<service>"} makes service x a module of the target
- {"op":"add","kind":"rule","name":"<rule text>"} ; {"op":"remove","kind":"rule","name":"<exact current rule>"} ;
  {"op":"rename","kind":"rule","name":"<exact current rule>","to":"<new text>"} (rule text is a sentence, not a folder name)
Ops apply in order. Return "ops": [] only when the request is not about the project structure.
`)
	return b.String()
}

type structureAnswer struct {
	Ops  []core.StructureOp `json:"ops"`
	Note string             `json:"note"`
}

// ApplyStructureChange records the AI's answer as a proposal.
func ApplyStructureChange(p *core.Project, raw string) error {
	s := p.Structure
	if s == nil || s.Request == "" {
		return errors.New("tidak ada permintaan perubahan struktur")
	}
	var a structureAnswer
	if err := ExtractJSONFunc(raw, &a, func() bool { return a.Ops != nil || a.Note != "" }); err != nil {
		return err
	}
	note := strings.TrimSpace(a.Note)
	unclear := func(why ...string) error {
		if why = slices.DeleteFunc(why, func(w string) bool { return w == "" }); len(why) > 0 {
			return fmt.Errorf("%w (%s)", ErrStructureUnclear, strings.Join(why, "; "))
		}
		return ErrStructureUnclear
	}
	ops, skipped := structure.Usable(s, p, a.Ops)
	if len(ops) == 0 {
		return unclear(append([]string{note}, skipped...)...)
	}
	if len(skipped) > 0 {
		note = strings.TrimSpace(note + " Dilewati: " + strings.Join(skipped, "; ") + ".")
	}
	if err := structure.Propose(s, p, ops, "ai", s.Request, note); err != nil {
		if errors.Is(err, structure.ErrNoChange) {
			return unclear(note)
		}
		return err
	}
	s.Request = ""
	return nil
}
