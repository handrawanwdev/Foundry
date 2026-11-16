package pipeline

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/uikit"
)

// Mockups: every designed page is rendered as an HTML mockup from the UI
// kit (token preset + page template). The user reviews and approves them;
// the approved mockups are the reference the coding agent builds from and
// the visual review measures against. A design change that alters a page's
// mockup leaves its approval stale.

// Mockups lists every page's mockup with its approval.
func (s *Service) Mockups() ([]uikit.MockupStatus, error) {
	p, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	return uikit.Default().Statuses(p), nil
}

// MockupWalks lists the use cases step by step with the page each step
// happens on: the walkthrough of the mockups.
func (s *Service) MockupWalks() ([]core.FlowWalk, error) {
	p, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	if p.UI == nil {
		return []core.FlowWalk{}, nil
	}
	return core.FlowWalks(p, p.UI.Layout), nil
}

// DesignGate is the quality gate report of the current design.
func (s *Service) DesignGate() (core.DesignGate, error) {
	p, err := s.Store.Load()
	if err != nil {
		return core.DesignGate{}, err
	}
	return p.DesignGate(), nil
}

// Mockup renders one page in a state; link makes the hrefs between pages.
func (s *Service) Mockup(page, state string, link func(string) string) (string, error) {
	p, err := s.Store.Load()
	if err != nil {
		return "", err
	}
	d := uikit.DocOf(p)
	d.Link = link
	return uikit.Default().RenderPage(d, strings.TrimSpace(page), state)
}

// ApproveMockups records the user's approval of the current mockup of the
// pages (every page when none is named).
func (s *Service) ApproveMockups(pages []string) ([]uikit.MockupStatus, error) {
	var out []uikit.MockupStatus
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Busy() {
			return ErrBusy
		}
		if p.UI == nil || p.UI.Layout == nil || len(p.UI.Layout.Pages) == 0 {
			return uikit.ErrNoDesign
		}
		if errs := core.Errors(p.DesignIssues(nil)); len(errs) > 0 {
			return fmt.Errorf("rancangan UI masih punya %d masalah; perbaiki dulu sebelum menyetujui mockup", len(errs))
		}
		// A design is not complete because its pages exist: a failing
		// quality gate check (a MUST use case without a screen) blocks too.
		if g := p.DesignGate(); !g.Ready {
			var failed []string
			for _, c := range g.Checks {
				if c.Status == core.GateFail {
					failed = append(failed, c.Name)
				}
			}
			return fmt.Errorf("quality gate desain belum lolos (%s); perbaiki dulu sebelum menyetujui mockup", strings.Join(failed, ", "))
		}
		k := uikit.Default()
		d := uikit.DocOf(p)
		if len(pages) == 0 {
			for _, pg := range p.UI.Layout.Pages {
				pages = append(pages, pg.ID)
			}
		}
		now := time.Now().UTC()
		for _, id := range pages {
			id = strings.TrimSpace(id)
			if !slices.ContainsFunc(p.UI.Layout.Pages, func(pg core.LayoutPage) bool { return pg.ID == id }) {
				return fmt.Errorf("%w: %q", uikit.ErrNoPage, id)
			}
			if p.UI.Mockups == nil {
				p.UI.Mockups = map[string]core.MockupApproval{}
			}
			p.UI.Mockups[id] = core.MockupApproval{Hash: k.PageHash(d, id), ApprovedAt: now}
		}
		// Approvals of pages that are gone are dropped.
		for id := range p.UI.Mockups {
			if !slices.ContainsFunc(p.UI.Layout.Pages, func(pg core.LayoutPage) bool { return pg.ID == id }) {
				delete(p.UI.Mockups, id)
			}
		}
		p.Log("ui", fmt.Sprintf("Mockup disetujui: %s", strings.Join(pages, ", ")))
		out = k.Statuses(p)
		return nil
	})
	return out, err
}

// IsMockupMissing reports a request for a page or design that has no mockup.
func IsMockupMissing(err error) bool {
	return errors.Is(err, uikit.ErrNoPage) || errors.Is(err, uikit.ErrNoDesign)
}
