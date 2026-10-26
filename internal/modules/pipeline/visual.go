package pipeline

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/uicheck"
	"github.com/handrawanwdev/foundry/internal/modules/work"
	"github.com/handrawanwdev/foundry/internal/modules/workflow"
)

// Visual quality: implemented screens are measured through snapshots and
// screenshots submitted by a browser tool or the coding agent. Foundry
// checks them against the UI/UX blueprint and turns open findings into a
// concrete correction command in the existing command workflow.

// RecordVisualReview checks one page and stores its audit. It works on a
// finalized project too: quality is measured after implementation.
func (s *Service) RecordVisualReview(in core.VisualReviewIn, source string) (core.PageAudit, error) {
	var audit core.PageAudit
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if source != "" {
			in.Source = source
		}
		a, err := uicheck.Review(p, in, time.Now().UTC())
		if err != nil {
			return err
		}
		audit = a
		p.Log("agent", fmt.Sprintf("Evaluasi visual %s: %s (%d temuan)", a.Page, core.StatusLabel(a.Status), len(a.Findings)))
		return nil
	})
	return audit, err
}

// VisualReport is the current measured quality of every designed page.
func (s *Service) VisualReport() (core.VisualSummary, *core.VisualAudit, error) {
	p, err := s.Store.Load()
	if err != nil {
		return core.VisualSummary{}, nil, err
	}
	if p.UI == nil || p.UI.Layout == nil || len(p.UI.Layout.Pages) == 0 {
		return core.VisualSummary{}, nil, uicheck.ErrNoDesign
	}
	return uicheck.Summary(p), p.Visual, nil
}

// CreateVisualFixes appends one correction command to the workflow with
// every open finding as a specific instruction. It reopens a finalized
// project because the approved specification is changing; that is explicit
// in the call, never silent.
func (s *Service) CreateVisualFixes() (string, error) {
	var id string
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if p.UI == nil || p.UI.Layout == nil || len(p.UI.Layout.Pages) == 0 {
			return uicheck.ErrNoDesign
		}
		if p.Workflow == nil || len(p.Workflow.Commands) == 0 {
			return errors.New("workflow belum disusun; instruksi perbaikan tidak punya tujuan")
		}
		findings := openFindings(p)
		if len(findings) == 0 {
			return errors.New("tidak ada temuan visual terbuka; tidak ada perbaikan yang perlu dikirim")
		}
		cmds := slices.Clone(p.Workflow.Commands)
		if len(cmds) >= work.MaxCommands {
			return fmt.Errorf("workflow sudah mencapai batas %d command; gabungkan atau hapus command sebelum menambah perbaikan", work.MaxCommands)
		}
		next := nextCommandID(cmds)
		pages := affectedPages(findings)
		text := visualFixInstructions(p, findings)
		cmd := core.Command{
			ID: next, Name: "Perbaiki tampilan: " + strings.Join(pages, ", "),
			Objective:    "Menghilangkan temuan evaluasi visual pada halaman " + strings.Join(pages, ", ") + ".",
			Context:      fmt.Sprintf("%d temuan evaluasi visual terbuka. Sumber: blueprint design-rules dan layout-spec.", len(findings)),
			Instructions: text,
			DependsOn:    fixDependencies(cmds, pages),
			Artifacts: []core.ArtifactRef{
				{ID: core.ArtifactDesignRules}, {ID: core.ArtifactLayoutSpec},
			},
			Outputs: []string{"Perubahan kode UI pada halaman yang disebut", "Screenshot ulang setiap viewport yang diuji"},
			Acceptance: []string{
				"Setiap temuan di atas tidak lagi muncul pada evaluasi visual ulang",
				"Screenshot dan snapshot baru dikirim melalui record_visual_review untuk halaman yang berubah",
				"Token, pola layout, dan state dari design-rules/layout-spec tidak dilanggar",
			},
			Constraints: []string{
				"Ikuti design-rules dan layout-spec; jangan mengubah halaman yang tidak disebut",
				"Jangan menambah pola layar atau warna di luar design system",
			},
			Status: core.CommandDraft,
		}
		cmds = append(cmds, cmd)
		exists := map[string]bool{}
		for _, a := range spec.Artifacts(p) {
			exists[a.ID] = true
		}
		if issues := workflow.Validate(cmds, exists); len(issues) > 0 {
			return fmt.Errorf("command perbaikan tidak valid: %s %s", issues[0].Command, issues[0].Message)
		}
		if p.Finalized() {
			unfinalize(p)
			p.Stage = core.StageDesign
			p.Log("system", "Blueprint dibuka kembali untuk perbaikan visual")
		}
		p.Workflow.Commands = cmds
		p.Workflow.Version++
		id = next
		p.Log("ui", "Command perbaikan visual dibuat: "+next+" ("+strings.Join(pages, ", ")+")")
		return nil
	})
	return id, err
}

// openFindings are the findings that still need a fix, worst first, with the
// page each belongs to.
func openFindings(p *core.Project) []core.VisualFinding {
	if p.Visual == nil {
		return nil
	}
	var out []core.VisualFinding
	for id, a := range p.Visual.Pages {
		for _, f := range a.Findings {
			if f.Page == "" {
				f.Page = id
			}
			out = append(out, f)
		}
	}
	rank := map[string]int{core.SeverityCritical: 0, core.SeverityHigh: 1, core.SeverityMedium: 2, core.SeverityLow: 3}
	slices.SortStableFunc(out, func(a, b core.VisualFinding) int {
		if rank[a.Severity] != rank[b.Severity] {
			return rank[a.Severity] - rank[b.Severity]
		}
		return strings.Compare(a.Page, b.Page)
	})
	return out
}

func affectedPages(findings []core.VisualFinding) []string {
	var out []string
	for _, f := range findings {
		if !slices.Contains(out, f.Page) {
			out = append(out, f.Page)
		}
	}
	slices.Sort(out)
	return out
}

func visualFixInstructions(p *core.Project, findings []core.VisualFinding) string {
	var b strings.Builder
	b.WriteString("Perbaiki tampilan yang diimplementasikan agar sesuai blueprint. Temuan terukur berikut berasal dari evaluasi snapshot browser.\n\n")
	for _, f := range findings {
		fmt.Fprintf(&b, "- **%s** halaman `%s`, kriteria %s", strings.ToUpper(f.Severity), f.Page, f.Criterion)
		if f.Evidence != "" {
			fmt.Fprintf(&b, " (%s)", f.Evidence)
		}
		fmt.Fprintf(&b, ": %s\n", f.Message)
		if f.Fix != "" {
			fmt.Fprintf(&b, "  Perbaikan: %s\n", f.Fix)
		}
	}
	b.WriteString("\nAcuan wajib: `blueprint/design-rules.md` dan `blueprint/layout-spec.md` (token, komponen, hierarchy, states, responsive). ")
	b.WriteString("Setelah memperbaiki, kirim ulang screenshot + snapshot halaman yang berubah untuk verifikasi ulang.\n")
	return b.String()
}

// fixDependencies makes the correction wait for the commands that build the
// affected pages, so it lands in a later wave. When no command names a page,
// it waits for every existing command: the screen exists somewhere.
func fixDependencies(cmds []core.Command, pages []string) []string {
	var out []string
	for _, c := range cmds {
		refs := slices.ContainsFunc(c.Artifacts, func(r core.ArtifactRef) bool { return r.ID == core.ArtifactLayoutSpec })
		mentions := slices.ContainsFunc(pages, func(pg string) bool { return strings.Contains(c.Instructions, pg) })
		if refs || mentions {
			out = append(out, c.ID)
		}
	}
	if len(out) == 0 {
		for _, c := range cmds {
			out = append(out, c.ID)
		}
	}
	return out
}

func nextCommandID(cmds []core.Command) string {
	next := 1
	for _, c := range cmds {
		var n int
		if _, err := fmt.Sscanf(c.ID, "CMD-%d", &n); err == nil && n >= next {
			next = n + 1
		}
	}
	return fmt.Sprintf("CMD-%03d", next)
}
