package core

import (
	"sort"
	"strings"
	"time"
)

// Visual review is the quality loop after a coding agent implements the
// design. Foundry does not render pages and never claims a page looks right
// without evidence: a review carries browser snapshots (measured nodes and
// screenshot files) and is checked against the blueprint deterministically.
// Screenshots are required evidence; a review without them stays untested.

// Visual page and criterion statuses.
const (
	VisualPass     = "pass"
	VisualPartial  = "partial"
	VisualFail     = "fail"
	VisualUntested = "untested"
)

// Finding severities.
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
)

// VisualAudit is the latest measured state of every page.
type VisualAudit struct {
	Version   int                  `json:"version"`
	UpdatedAt time.Time            `json:"updated_at"`
	Source    string               `json:"source,omitempty"`
	Pages     map[string]PageAudit `json:"pages,omitempty"`
	Reviews   []VisualReviewLog    `json:"reviews,omitempty"`
}

// PageAudit is the measured state of one page.
type PageAudit struct {
	Page        string            `json:"page"`
	URL         string            `json:"url,omitempty"`
	Status      string            `json:"status"`
	Previous    string            `json:"previous_status,omitempty"`
	Resolved    []string          `json:"resolved,omitempty"`
	Source      string            `json:"source,omitempty"`
	CheckedAt   time.Time         `json:"checked_at"`
	Viewports   []string          `json:"viewports,omitempty"`
	Screenshots []string          `json:"screenshots,omitempty"`
	Findings    []VisualFinding   `json:"findings"`
	Criteria    []VisualCriterion `json:"criteria"`
}

// VisualFinding is one measured problem with evidence and a concrete fix.
type VisualFinding struct {
	Page      string `json:"page"`
	Criterion string `json:"criterion"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	Evidence  string `json:"evidence,omitempty"`
	Fix       string `json:"fix,omitempty"`
}

// VisualCriterion is the result of one check for a page.
type VisualCriterion struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"` // pass | fail | untested
	Severity string `json:"severity,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// VisualReviewLog is the audit trail of one submitted review.
type VisualReviewLog struct {
	At        time.Time `json:"at"`
	Page      string    `json:"page"`
	Source    string    `json:"source,omitempty"`
	Status    string    `json:"status"`
	Findings  int       `json:"findings"`
	Viewports []string  `json:"viewports,omitempty"`
}

// VisualSummary is what the progress view and the export read.
type VisualSummary struct {
	Pages      int       `json:"pages"`
	Pass       int       `json:"pass"`
	Partial    int       `json:"partial"`
	Fail       int       `json:"fail"`
	Untested   int       `json:"untested"`
	OpenAll    int       `json:"open_findings"`
	OpenHigh   int       `json:"open_high"`
	CheckedAt  time.Time `json:"checked_at,omitempty"`
	NotChecked []string  `json:"not_checked,omitempty"`
}

// VisualReviewIn is the contract a browser automation tool, the coding agent
// or a person submits. Nodes are the measured DOM (not a screenshot alone)
// so spacing, contrast, overflow and targets can be checked.
type VisualReviewIn struct {
	Page      string           `json:"page"`
	URL       string           `json:"url,omitempty"`
	Source    string           `json:"source,omitempty"`
	Snapshots []VisualSnapshot `json:"snapshots"`
}

type VisualSnapshot struct {
	Viewport   VisualViewport `json:"viewport"`
	State      string         `json:"state,omitempty"` // default | loading | empty | ... | ""
	Screenshot string         `json:"screenshot"`      // file inside the project
	Background string         `json:"background,omitempty"`
	Nodes      []VisualNode   `json:"nodes"`
}

type VisualViewport struct {
	Name   string  `json:"name,omitempty"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type VisualNode struct {
	Path   string            `json:"path"` // "main > section > h1"
	Tag    string            `json:"tag"`
	Role   string            `json:"role,omitempty"`
	Text   string            `json:"text,omitempty"`
	Bounds VisualBounds      `json:"bounds"`
	Style  VisualStyle       `json:"style"`
	Attrs  map[string]string `json:"attrs,omitempty"`
}

type VisualBounds struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// VisualStyle is the computed style that matters for the checks. Lengths are
// CSS pixels. Background is the computed background color of the element.
type VisualStyle struct {
	Color      string    `json:"color,omitempty"`
	Background string    `json:"background,omitempty"`
	FontSize   float64   `json:"font_size,omitempty"`
	FontWeight int       `json:"font_weight,omitempty"`
	LineHeight float64   `json:"line_height,omitempty"`
	Padding    []float64 `json:"padding,omitempty"` // top, right, bottom, left
	Margin     []float64 `json:"margin,omitempty"`  // top, right, bottom, left
	Gap        float64   `json:"gap,omitempty"`
	Display    string    `json:"display,omitempty"`
}

// Hidden reports styles that make a node invisible to the checks.
func (n VisualNode) Hidden() bool {
	return strings.EqualFold(strings.TrimSpace(n.Style.Display), "none") || n.Bounds.Width <= 0 || n.Bounds.Height <= 0
}

// Interactive reports elements a user activates or fills.
func (n VisualNode) Interactive() bool {
	switch strings.ToLower(strings.TrimSpace(n.Tag)) {
	case "a", "button", "input", "select", "textarea", "summary":
		return true
	}
	switch strings.ToLower(strings.TrimSpace(n.Role)) {
	case "button", "link", "checkbox", "radio", "switch", "tab", "menuitem", "option", "textbox", "combobox":
		return true
	}
	return false
}

// Summary counts the current page audits against the pages of the design.
func (v *VisualAudit) Summary(pages []LayoutPage) VisualSummary {
	s := VisualSummary{Pages: len(pages)}
	if v == nil {
		for _, pg := range pages {
			s.Untested++
			s.NotChecked = append(s.NotChecked, pg.ID)
		}
		return s
	}
	for _, pg := range pages {
		a, ok := v.Pages[pg.ID]
		if !ok {
			s.Untested++
			s.NotChecked = append(s.NotChecked, pg.ID)
			continue
		}
		switch a.Status {
		case VisualPass:
			s.Pass++
		case VisualPartial:
			s.Partial++
		case VisualFail:
			s.Fail++
		default:
			s.Untested++
			s.NotChecked = append(s.NotChecked, pg.ID)
		}
	}
	for _, a := range v.Pages {
		for _, f := range a.Findings {
			s.OpenAll++
			if f.Severity == SeverityHigh || f.Severity == SeverityCritical {
				s.OpenHigh++
			}
		}
		if a.CheckedAt.After(s.CheckedAt) {
			s.CheckedAt = a.CheckedAt
		}
	}
	sort.Strings(s.NotChecked)
	return s
}

// Page returns the audit of one page.
func (v *VisualAudit) Page(id string) *PageAudit {
	if v == nil {
		return nil
	}
	a, ok := v.Pages[id]
	if !ok {
		return nil
	}
	return &a
}

// StatusLabel is the human label of a visual status.
func StatusLabel(s string) string {
	switch s {
	case VisualPass:
		return "lulus"
	case VisualPartial:
		return "sebagian"
	case VisualFail:
		return "gagal"
	default:
		return "belum diuji"
	}
}
