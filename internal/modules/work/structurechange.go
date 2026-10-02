package work

import (
	"errors"
	"fmt"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/structure"
)

// KindStructure turns a natural-language structure change into edits of
// the Structure IR. The result is a proposal: the user sees its impact and
// applies or cancels it.
const KindStructure = "structure"

const maxStructureRequest = 1000

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
Ops apply in order. Return "ops": [] when the request asks for nothing that can be changed.
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
	if len(a.Ops) == 0 {
		return ErrStructureUnclear
	}
	if err := structure.Propose(s, p, a.Ops, "ai", s.Request, strings.TrimSpace(a.Note)); err != nil {
		if errors.Is(err, structure.ErrNoChange) {
			return ErrStructureUnclear
		}
		return err
	}
	s.Request = ""
	return nil
}
