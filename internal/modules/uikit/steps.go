package uikit

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// A stepper shows step names. A design often writes more into the section
// than the stepper can show: how a step is validated, what happens when it
// fails, how the fields look. The quality gate lists that content so the
// designer moves it where it is drawn (a state, a rule, another section).

// stepNumber is the number a step name was written with ("1 Data", "2. Akun").
var stepNumber = regexp.MustCompile(`^\d+[.)]?\s+`)

// stepJoin is the word that joins the last step of a written list
// ("pilih file, validasi, lalu pulihkan").
var stepJoin = regexp.MustCompile(`(?i)^(lalu|kemudian|dan|then|and)\s+`)

// stepList splits a stepper section into its step names and the content
// lines that name no step. Several numbered lines are the steps; else the
// first line listing several elements; else every line is one step.
func stepList(s core.Section) (steps, notes []string) {
	var lines, numbered, rest []string
	for _, c := range s.Content {
		if c = strings.TrimSpace(c); c == "" {
			continue
		}
		lines = append(lines, c)
		if stepNumber.MatchString(c) {
			numbered = append(numbered, c)
		} else {
			rest = append(rest, c)
		}
	}
	if len(numbered) >= 2 {
		for _, c := range numbered {
			steps = append(steps, stepName(c))
		}
		return steps, rest
	}
	for i, c := range lines {
		els := items(core.Section{Content: []string{c}})
		if len(els) < 2 {
			continue
		}
		for _, e := range els {
			steps = append(steps, stepName(e))
		}
		notes = append(notes, lines[:i]...)
		return steps, append(notes, lines[i+1:]...)
	}
	for _, c := range lines {
		steps = append(steps, stepName(c))
	}
	return steps, nil
}

func stepName(s string) string {
	s = strings.TrimSpace(stepNumber.ReplaceAllString(strings.TrimSpace(s), ""))
	return capital(strings.TrimSpace(stepJoin.ReplaceAllString(s, "")))
}

// stepIssues are the findings on one stepper section.
func stepIssues(pg *core.LayoutPage, s core.Section, steps, notes []string) []string {
	where := fmt.Sprintf("halaman %s, bagian %s", pg.ID, s.Name)
	var out []string
	if len(steps) < 2 {
		out = append(out, fmt.Sprintf("%s: stepper butuh minimal 2 nama langkah, isinya %q", where, strings.Join(s.Content, "; ")))
	}
	for _, n := range notes {
		out = append(out, fmt.Sprintf("%s: %q bukan nama langkah; stepper hanya menampilkan nama langkah", where, n))
	}
	return out
}

// ContentCheck checks the components of the design as the mockups draw
// them: each stepper names at least two steps and nothing else, and no
// table or list holds a form's fields.
func (k *Kit) ContentCheck(p *core.Project) core.GateCheck {
	c := core.GateCheck{ID: "component-content", Name: "Isi komponen", Status: core.GateSkip}
	d := DocOf(p)
	if d.Layout == nil {
		return c
	}
	steppers, colls := 0, 0
	for i := range d.Layout.Pages {
		r := &render{kit: k, doc: d, page: &d.Layout.Pages[i]}
		r.html()
		steppers, colls = steppers+r.steppers, colls+r.collections
		for _, is := range r.contentIssues {
			c.Status = core.GateWarn
			if len(c.Items) < core.MaxGateItems {
				c.Items = append(c.Items, is)
			}
		}
	}
	if steppers+colls > 0 && c.Status == core.GateSkip {
		c.Status = core.GatePass
	}
	c.Detail = fmt.Sprintf("%d stepper, %d tabel/daftar diperiksa", steppers, colls)
	return c
}

func init() {
	core.RegisterGateCheck(func(p *core.Project) core.GateCheck { return Default().ContentCheck(p) })
}
