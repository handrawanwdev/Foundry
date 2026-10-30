package pipeline

import (
	"errors"
	"fmt"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// UI design: design rules and layout spec, written and revised by the AI
// (validated, refused as a whole when invalid) or edited by the user. Both
// are blueprint artifacts; commands that recorded them are reported for
// review when they change.

var errNoScreens = errors.New("produk tidak punya platform dengan layar (web, android, ios, desktop)")

// designEditable is blueprintEditable for a product with screens.
func designEditable(p *core.Project) error {
	if err := blueprintEditable(p); err != nil {
		return err
	}
	if len(p.UIPlatforms()) == 0 {
		return errNoScreens
	}
	return nil
}

// GenerateDesign asks the AI to write the design rules and layout spec. It
// only starts while nothing is written; a written design is changed with
// ReviseDesign.
func (s *Service) GenerateDesign() error {
	return s.startAIIf(work.KindDesign, func(p *core.Project) error {
		if err := designEditable(p); err != nil {
			return err
		}
		if p.UI.Written() {
			return fmt.Errorf("%w: aturan desain dan layout sudah ada; minta perubahan lewat prompt", ErrWrongStage)
		}
		p.UI = nil
		p.Log("ui", "Susun aturan desain & layout")
		return nil
	})
}

// ReviseDesign asks the AI to change the design for a request; focus names
// the design artifacts, shells or pages it is about.
func (s *Service) ReviseDesign(text string, focus []string) error {
	return s.startAIIf(work.KindDesign, func(p *core.Project) error {
		if err := designEditable(p); err != nil {
			return err
		}
		if err := work.RequestDesignChange(p, text, focus); err != nil {
			return err
		}
		p.Log("ui", "Minta perubahan rancangan UI: "+text)
		return nil
	})
}

// EditDesignRules saves the user's design rules; invalid rules are refused.
func (s *Service) EditDesignRules(in core.DesignRules) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := designEditable(p); err != nil {
			return err
		}
		if err := work.EditDesignRules(p, in); err != nil {
			return err
		}
		p.Log("ui", "Aturan desain diedit")
		return nil
	})
}

// EditLayout saves the user's layout spec; an invalid spec is refused.
func (s *Service) EditLayout(in core.LayoutSpec) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := designEditable(p); err != nil {
			return err
		}
		if err := work.EditLayout(p, in); err != nil {
			return err
		}
		p.Log("ui", "Spesifikasi layout diedit")
		return nil
	})
}

// designBlockers: a product with screens needs valid design rules and a
// layout spec.
func designBlockers(p *core.Project) []string {
	if !p.HasUI() {
		return nil
	}
	if !p.UI.Written() {
		return []string{"aturan desain dan spesifikasi layout belum disusun"}
	}
	errs := core.Errors(p.DesignIssues(nil))
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return []string{"rancangan UI: " + errs[0].Message}
	}
	return []string{fmt.Sprintf("rancangan UI: %d hal wajib diperbaiki", len(errs))}
}
