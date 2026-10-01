package pipeline

import (
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

// EditDesign applies canvas operations to one page's Design IR directly,
// without the AI. The result is validated, measured and saved as a version.
func (s *Service) EditDesign(pageID string, ops []work.CanvasOp) (*core.Project, error) {
	p, err := s.Store.Update(func(p *core.Project) error {
		if err := editable(p); err != nil {
			return err
		}
		done, err := work.EditPage(p, pageID, ops)
		if err != nil {
			return err
		}
		measureQuality(p)
		note := strings.Join(done, "; ")
		recordVersion(p, "canvas", note)
		p.Log("ui", "Edit kanvas: "+note)
		return nil
	})
	if err == nil {
		saveQuality(p)
	}
	return p, err
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
