package pipeline

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// Capturer screenshots an external page; the Chrome renderer is one.
type Capturer interface {
	Capture(ctx context.Context, url, device, out string) error
}

// SetReference names the website or app the design takes inspiration from
// ("" removes it). Once the story is understood it is analyzed at once;
// before that the analysis runs after discovery, before the first design.
func (s *Service) SetReference(rawURL, notes string) (*core.Project, error) {
	var ref *core.DesignReference
	if strings.TrimSpace(rawURL) != "" {
		var err error
		if ref, err = work.ParseReference(rawURL, notes); err != nil {
			return nil, err
		}
	}
	analyze := false
	p, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if d := p.Design; d != nil && d.Status == core.DesignApproved {
			return ErrWrongStage
		}
		if ref == nil {
			if old := p.Reference; old != nil && old.ThemeID != "" && p.Theme == old.ThemeID {
				p.Theme = ""
			}
			p.Reference = nil
			p.Log("ui", "Referensi desain dihapus")
			return nil
		}
		p.Reference = ref
		analyze = p.Discovery != nil
		p.Log("ui", "Referensi desain: "+ref.URL)
		return nil
	})
	if err != nil || !analyze {
		return p, err
	}
	return p, s.startAI(work.KindReference)
}

// prepareReference takes the screenshots of the reference when a browser
// is available. Without one the analysis runs on the URL alone.
func (s *Service) prepareReference(p *core.Project) (*core.Project, error) {
	if p.Reference == nil {
		return nil, errors.New("tidak ada referensi desain")
	}
	var shots []core.QAShot
	if c, ok := s.Render.(Capturer); ok {
		for _, dev := range []string{"desktop", "mobile"} {
			img := filepath.Join(p.Dir, core.StateDir, "references", p.Reference.Name+"-"+dev+".png")
			if err := c.Capture(s.context(), p.Reference.URL, dev, img); err == nil {
				shots = append(shots, core.QAShot{Device: dev, Image: img})
			}
		}
	}
	return s.Store.Update(func(p *core.Project) error {
		if r := p.Reference; r != nil {
			r.Status, r.Error, r.Shots = "analyzing", "", shots
		}
		return nil
	})
}

// applyReference lands the Theme DNA; a draft design follows the new theme
// at once (tokens and style; its pages stay).
func applyReference(p *core.Project, raw string) error {
	t, err := work.ApplyReference(p, raw)
	if err != nil || t == nil {
		return err
	}
	if d := p.Design; d != nil && len(d.Pages) > 0 && d.Status != core.DesignApproved && d.Revision == nil {
		th, ctx := work.ResolveTheme(p)
		work.ApplyTheme(d, th, ctx)
		measureQuality(p)
	}
	return nil
}
