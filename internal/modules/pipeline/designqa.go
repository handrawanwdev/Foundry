package pipeline

import (
	"context"
	"errors"
	"fmt"
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

// prepareQA renders the changed pages. When nothing can be rendered the
// check is recorded as skipped, the job closes and nil is returned.
func (s *Service) prepareQA(p *core.Project) (*core.Project, error) {
	rev := p.Design.Revision
	if rev == nil || rev.QA == nil {
		return nil, s.finishQA("skipped", "tidak ada perubahan untuk diperiksa")
	}
	ids, devs := work.QAPages(p)
	var shots []core.QAShot
	for i, id := range ids {
		img := filepath.Join(p.Dir, core.StateDir, "qa", fmt.Sprintf("%s-%s-r%d.png", id, devs[i], rev.QA.Round))
		outline, err := s.Render.Shot(context.Background(), p.Dir, id, devs[i], img)
		if err != nil {
			return nil, s.finishQA("skipped", "Browser tidak bisa merender halaman: "+err.Error())
		}
		shots = append(shots, core.QAShot{PageID: id, Device: devs[i], Image: img, Outline: outline})
	}
	if len(shots) == 0 {
		return nil, s.finishQA("skipped", "tidak ada halaman yang berubah")
	}
	return s.Store.Update(func(p *core.Project) error {
		if r := p.Design.Revision; r != nil && r.QA != nil {
			r.QA.Shots = shots
		}
		return nil
	})
}

func (s *Service) finishQA(status, notes string) error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if d := p.Design; d != nil && d.Revision != nil && d.Revision.QA != nil {
			d.Revision.QA.Status, d.Revision.QA.Notes = status, notes
		}
		p.Job = &core.Job{Kind: work.KindDesignQA, Status: core.JobDone, Message: "Pemeriksaan visual dilewati: " + notes}
		p.Log("system", "Pemeriksaan visual dilewati: "+notes)
		return nil
	})
	return err
}
