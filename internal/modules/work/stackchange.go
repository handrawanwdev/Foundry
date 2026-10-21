package work

import (
	"errors"
	"fmt"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/stack"
)

// KindStack turns a natural-language technology change into structured
// changes of the Technology IR. The result is a proposal: the user sees
// its impact and applies or cancels it.
const KindStack = "stack"

const maxStackRequest = 1000

// ErrStackUnclear: the AI found nothing to change for the request.
var ErrStackUnclear = errors.New("AI tidak menemukan perubahan teknologi untuk permintaan ini")

func CleanStackRequest(text string) (string, error) {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return "", errors.New("tuliskan perubahan teknologi yang diinginkan")
	case len(text) > maxStackRequest:
		return "", fmt.Errorf("permintaan terlalu panjang (maks. %d karakter)", maxStackRequest)
	}
	return text, nil
}

func StackPrompt(p *core.Project) string {
	s := p.Stack
	var b strings.Builder
	b.WriteString(`You update a product's technology decision from the user's request.
The user decides the technology. Change ONLY the areas the request names or clearly implies
(for example "ganti backend ke Rust" changes backend). Areas the user says to keep, and every
area the request does not mention, stay unchanged: do not list them. Never swap a choice for
your own preference.

CURRENT DECISION (area: choice — reason):
`)
	for _, d := range s.Decisions {
		fmt.Fprintf(&b, "- %s: %s", d.Area, d.Choice())
		switch {
		case d.Override != nil:
			b.WriteString(" (chosen by the user)")
		case d.Source == core.SourceExisting || d.Source == "" && s.Mode == core.StructureExisting:
			b.WriteString(" (already used in the code)")
		}
		fmt.Fprintf(&b, " — %s\n", d.Reason)
	}
	fmt.Fprintf(&b, "\nTechnical needs: %s\n", strings.Join(s.Needs, ", "))
	b.WriteString("\nKNOWN OPTIONS (prefer these exact names; architecture, repository and cache must be one of them; other areas may name another technology):\n")
	for _, area := range core.TechAreas {
		if s.Decision(area) == nil {
			continue
		}
		var vs []string
		for _, o := range stack.Catalog[area] {
			vs = append(vs, o.Value)
		}
		fmt.Fprintf(&b, "- %s: %s\n", area, strings.Join(vs, " | "))
	}
	fmt.Fprintf(&b, "\nUSER REQUEST:\n%s\n", s.Request)
	b.WriteString(`
Answer with one JSON object only:
{"changes":[{"area":"backend","value":"Rust + Axum","reason":"<why, in the user's words, Indonesian>"}],
 "note":"<one Indonesian sentence on what you changed, or why nothing>"}
Use only areas from CURRENT DECISION. value is "Tidak perlu" to remove an optional component.
Replacing a technology "already used in the code" is a migration: change it only when the request
asks for it explicitly, and give the user's reason. Areas that depend on a change (UI library on
the frontend, testing tools on the language, ...) are re-evaluated by Foundry; do not list them.
Return "changes": [] when the request asks for nothing that can be changed.
`)
	return b.String()
}

type stackAnswer struct {
	Changes []stack.Change `json:"changes"`
	Note    string         `json:"note"`
}

// ApplyStackChange records the AI's answer as a proposal.
func ApplyStackChange(p *core.Project, raw string) error {
	s := p.Stack
	if s == nil || s.Request == "" {
		return errors.New("tidak ada permintaan perubahan teknologi")
	}
	var a stackAnswer
	if err := ExtractJSONFunc(raw, &a, func() bool { return a.Changes != nil || a.Note != "" }); err != nil {
		return err
	}
	req := s.Request
	if len(a.Changes) == 0 {
		return ErrStackUnclear
	}
	if err := stack.Propose(s, a.Changes, "ai", req, strings.TrimSpace(a.Note)); err != nil {
		if errors.Is(err, stack.ErrNoChange) {
			return ErrStackUnclear
		}
		return err
	}
	s.Request = ""
	return nil
}
