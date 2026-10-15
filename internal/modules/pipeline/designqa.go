package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// Renderer screenshots one designed page in a browser at a device viewport
// and returns the outline of what the wireframe drew.
type Renderer interface {
	Shot(ctx context.Context, projectDir, pageID, device, out string) (string, error)
}

var devices = []string{"desktop", "tablet", "mobile"}

// cleanScope keeps only known pages, components and devices.
func cleanScope(p *core.Project, sc *core.ChangeScope) *core.ChangeScope {
	if sc == nil {
		return nil
	}
	out := &core.ChangeScope{Open: sc.Open, Fields: sc.Fields, General: sc.General}
	for _, sp := range sc.Pages {
		i := slices.IndexFunc(p.Design.Pages, func(pg core.Page) bool { return pg.ID == sp.ID })
		if i < 0 {
			continue
		}
		var comps []int
		for _, c := range sp.Components {
			if c >= 0 && c < len(p.Design.Pages[i].Components) && !slices.Contains(comps, c) {
				comps = append(comps, c)
			}
		}
		sp.Components = comps
		if !slices.Contains(devices, sp.Device) {
			sp.Device = ""
		}
		out.Pages = append(out.Pages, sp)
	}
	if len(out.Pages) == 0 {
		out.Open = true // nothing named: the request is about the whole design
	}
	return out
}

// noteRejection stores why a design or patch was refused (out of scope or
// invalid Design IR) so the next prompt shows it. It reports whether an
// automatic retry should run: only once.
func (s *Service) noteRejection(kind string, err error) bool {
	var rej *work.PatchRejected
	var invalid *core.IRInvalid
	if kind != work.KindDesign || !errors.As(err, &rej) && !errors.As(err, &invalid) {
		return false
	}
	retry := false
	s.Store.Update(func(p *core.Project) error {
		if p.Design == nil {
			// A first design that failed the IR check: keep the reason for
			// the retry prompt; the next valid design replaces this.
			p.Design = &core.Design{Status: core.DesignDraft}
		}
		retry = p.Design.Rejected == ""
		p.Design.Rejected = err.Error()
		p.Log("system", err.Error())
		return nil
	})
	return retry
}

// prepareQA renders the pages of the active check. When nothing can be
// rendered the check is recorded as skipped, the job closes and nil is
// returned.
func (s *Service) prepareQA(p *core.Project) (*core.Project, error) {
	qa, initial := work.ActiveQA(p.Design)
	if qa == nil {
		return nil, s.finishQA("skipped", "tidak ada perubahan untuk diperiksa")
	}
	tag := "r" // revision rounds
	if initial {
		tag = "i"
	}
	ids, devs := work.QAPages(p)
	var shots []core.QAShot
	for i, id := range ids {
		img := filepath.Join(p.Dir, core.StateDir, "qa", fmt.Sprintf("%s-%s-%s%d.png", id, devs[i], tag, qa.Round))
		outline, err := s.Render.Shot(s.context(), p.Dir, id, devs[i], img)
		if err != nil {
			return nil, s.finishQA("skipped", "Browser tidak bisa merender halaman: "+err.Error())
		}
		shots = append(shots, core.QAShot{PageID: id, Device: devs[i], Image: img, Outline: outline})
	}
	if len(shots) == 0 {
		return nil, s.finishQA("skipped", "tidak ada halaman yang berubah")
	}
	return s.Store.Update(func(p *core.Project) error {
		if qa, _ := work.ActiveQA(p.Design); qa != nil {
			qa.Shots = shots
		}
		return nil
	})
}

func (s *Service) finishQA(status, notes string) error {
	p, err := s.Store.Update(func(p *core.Project) error {
		if qa, initial := work.ActiveQA(p.Design); qa != nil {
			qa.Status, qa.Notes = status, notes
			if initial {
				qa.Issues = work.LintDesign(p, nil) // the design rules still apply without a browser
			}
		}
		measureQuality(p)
		p.Job = &core.Job{Kind: work.KindDesignQA, Status: core.JobDone, Message: "Pemeriksaan visual dilewati: " + notes}
		p.Log("system", "Pemeriksaan visual dilewati: "+notes)
		return nil
	})
	if err == nil {
		saveQuality(p)
	}
	return err
}

// MaxQualityHistory caps the history file (lines, one per page measured).
const MaxQualityHistory = 2000

// measureQuality re-measures the design after it or its check changed.
func measureQuality(p *core.Project) {
	if p.Design != nil {
		p.Design.Quality = work.MeasureDesign(p)
	}
}

// saveQuality writes the last measurement to .foundry/design-quality.json
// and appends it to .foundry/design-quality-history.json. Best effort: the
// measurement also lives in project.json.
func saveQuality(p *core.Project) {
	if p == nil || p.Design == nil || p.Design.Quality == nil {
		return
	}
	dir := filepath.Join(p.Dir, core.StateDir)
	if b, err := json.MarshalIndent(p.Design.Quality, "", "  "); err == nil {
		os.WriteFile(filepath.Join(dir, "design-quality.json"), b, 0o644)
	}
	path := filepath.Join(dir, "design-quality-history.json")
	var hist []work.QualityRecord
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, &hist)
	}
	hist = append(hist, work.QualityRecords(p)...)
	if len(hist) > MaxQualityHistory {
		hist = hist[len(hist)-MaxQualityHistory:]
	}
	if b, err := json.MarshalIndent(hist, "", "  "); err == nil {
		os.WriteFile(path, b, 0o644)
	}
}
