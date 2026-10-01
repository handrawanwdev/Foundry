package pipeline

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// recordVersion snapshots the design after a change landed. Best effort:
// the change itself is already valid without its snapshot.
func recordVersion(p *core.Project, source, note string) {
	if _, err := work.RecordVersion(p, source, note); err != nil {
		p.Log("system", "Versi rancangan tidak tersimpan: "+err.Error())
	}
}

// recordAIVersion snapshots the design an AI job just changed.
func recordAIVersion(p *core.Project, kind string) {
	d := p.Design
	if d == nil {
		return
	}
	switch kind {
	case work.KindDesign:
		if d.Revision != nil {
			recordVersion(p, "revision", d.Revision.Request)
		} else {
			recordVersion(p, "generate", "")
		}
	case work.KindDesignQA:
		recordVersion(p, "qa", "")
	case work.KindReference:
		note := ""
		if p.Reference != nil {
			note = p.Reference.Name
		}
		recordVersion(p, "reference", note)
	}
}

// editable is a draft design no job is working on.
func editable(p *core.Project) error {
	if p.Busy() {
		return ErrBusy
	}
	if p.Design == nil || len(p.Design.Pages) == 0 || p.Design.Status == core.DesignApproved {
		return ErrWrongStage
	}
	return nil
}

// CanvasResult is a landed canvas edit and what it made worse.
type CanvasResult struct {
	Project  *core.Project `json:"project"`
	Warnings []string      `json:"warnings,omitempty"`
}

// EditDesign applies canvas operations to one page's Design IR directly,
// without the AI. The result is validated, measured and saved as a version;
// a quality drop is reported, not refused (the user decides).
func (s *Service) EditDesign(pageID string, ops []work.CanvasOp) (*CanvasResult, error) {
	var warnings []string
	p, err := s.Store.Update(func(p *core.Project) error {
		if err := editable(p); err != nil {
			return err
		}
		before := work.MeasureDesign(p)
		done, err := work.EditPage(p, pageID, ops)
		if err != nil {
			return err
		}
		measureQuality(p)
		warnings = work.QualityWarnings(before, p.Design.Quality)
		note := strings.Join(done, "; ")
		recordVersion(p, "canvas", note)
		p.Log("ui", "Edit kanvas: "+note)
		return nil
	})
	if err != nil {
		return nil, err
	}
	saveQuality(p)
	return &CanvasResult{Project: p, Warnings: warnings}, nil
}

// SetDesignTemplate picks the template the next generated design starts
// from: "" lets the AI choose, core.TemplateBlank asks for none. A design
// that exists keeps its pages; "Buat ulang" applies the pick.
func (s *Service) SetDesignTemplate(id string) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if id != "" && id != core.TemplateBlank && work.FindDesignTemplate(id) == nil {
			return fmt.Errorf("template %q tidak tersedia", id)
		}
		if d := p.Design; d != nil && d.Status == core.DesignApproved {
			return ErrWrongStage
		}
		p.DesignTemplate = id
		p.Log("ui", "Template awal: "+cmp.Or(id, "dipilih AI"))
		return nil
	})
}

// RestoreDesignVersion makes a saved version the current design.
func (s *Service) RestoreDesignVersion(id int) (*core.Project, error) {
	p, err := s.Store.Update(func(p *core.Project) error {
		if err := editable(p); err != nil {
			return err
		}
		v, err := work.FindVersion(p, id)
		if err != nil {
			return err
		}
		if err := work.RestoreVersion(p, *v); err != nil {
			return err
		}
		measureQuality(p)
		recordVersion(p, "restore", fmt.Sprintf("dari versi %d", id))
		p.Log("ui", fmt.Sprintf("Rancangan dikembalikan ke versi %d", id))
		return nil
	})
	if err == nil {
		saveQuality(p)
	}
	return p, err
}

// RegenerateDesign asks the AI for a new design from the current features
// and flows, without running discovery again: the theme, the template
// selection and the design knowledge are applied anew. The current design
// stays until the new one lands (a failure leaves it untouched) and is kept
// as a version.
func (s *Service) RegenerateDesign() error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if err := editable(p); err != nil {
			return err
		}
		if p.Design.Revision != nil {
			return fmt.Errorf("simpan atau batalkan perubahan rancangan dulu")
		}
		recordVersion(p, "generate", "sebelum dibuat ulang")
		d := p.Design
		d.Status, d.Scope, d.Screens, d.Rejected = core.DesignDraft, nil, nil, ""
		p.Log("ui", "Buat ulang rancangan")
		return nil
	})
	if err != nil {
		return err
	}
	return s.startAI(work.KindDesign)
}
