package work

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/handrawanw/foundry/internal/core"
)

// Design quality: the rule findings on the Design IR plus the critic's
// findings, scored per dimension, with a gate on blocker findings and a
// before/after comparison of every critic patch. The screenshot is evidence;
// the Design IR is the source. Scores are an internal indicator, never a
// ranking between designs.

var QualityDimensions = []string{"hierarchy", "composition", "spacing", "density", "responsive", "navigation", "ai_slop"}

// dimensionOf maps a finding category onto a dimension; the rest count as
// composition.
var dimensionOf = map[string]string{"hierarchy": "hierarchy", "duplicate_action": "hierarchy", "spacing": "spacing", "density": "density",
	"responsive": "responsive", "overflow": "responsive", "navigation": "navigation", "ai_slop": "ai_slop"}

var penalty = map[string]int{"critical": 40, "major": 25, "minor": 10}

// blockerRules fail the gate: invalid hierarchy, duplicate primary action,
// unusable mobile layout, a layout that does not support the strategy.
var blockerRules = []string{"primary_hidden", "no_primary_focal", "duplicate_primary_action", "mobile_split", "table_not_swapped", "layout_mismatch"}

// IsBlocker reports whether a finding fails the quality gate. A critic
// finding blocks when critical, an overflow, or a major problem on mobile.
func IsBlocker(is core.QAIssue) bool {
	if is.Source == "lint" {
		return slices.Contains(blockerRules, is.Rule)
	}
	return is.Severity == "critical" || is.Severity == "major" && (is.Category == "overflow" || is.Category == "responsive" && is.Device == "mobile")
}

// PageMetrics counts what the quality rules measure on one page's IR.
func PageMetrics(pg core.Page) map[string]int {
	m := map[string]int{}
	primary := ""
	if a := pg.PrimaryAction; a != nil {
		primary = strings.ToLower(strings.TrimSpace(a.Label))
	}
	for _, c := range pg.Components {
		label := c.Label
		if c.Type == "form" {
			label = FormSubmit(pg)
		}
		if primary != "" && (c.Type == "button" || c.Type == "form") && strings.ToLower(strings.TrimSpace(label)) == primary {
			m["primary_actions"]++
		}
		if c.Type == "stat" {
			m["stats"]++
		}
	}
	for _, a := range duplicateActions(pg) {
		if strings.ToLower(a) != primary {
			m["duplicate_actions"]++
		}
	}
	if l := pg.Layout; l != nil {
		m["nested_surfaces"] = nestedSurfaces(l.Root, false)
		if core.BaseDevice(pg) == "desktop" && pg.Platform != core.PlatformDesktop {
			mob, _ := l.Resolve("mobile")
			mob.Root.Walk(func(n core.LayoutNode, _ *core.LayoutNode) {
				if n.Component == "" && (n.Type == "split" || n.Type == "sidebar") {
					m["mobile_split_remaining"]++
				}
			})
		}
	}
	return m
}

func nestedSurfaces(n core.LayoutNode, inside bool) int {
	count := 0
	if n.Component == "" && n.Surface != "" {
		if inside {
			count++
		}
		inside = true
	}
	for _, k := range n.Children {
		count += nestedSurfaces(k, inside)
	}
	return count
}

func scoreDims(issues []core.QAIssue) map[string]int {
	d := map[string]int{}
	for _, k := range QualityDimensions {
		d[k] = 100
	}
	for _, is := range issues {
		k := dimensionOf[is.Category]
		if k == "" {
			k = "composition"
		}
		d[k] = max(0, d[k]-penalty[is.Severity])
	}
	return d
}

func mean(sum, n int) int {
	if n == 0 {
		return 100
	}
	return int(math.Round(float64(sum) / float64(n)))
}

func overall(d map[string]int) int {
	sum := 0
	for _, k := range QualityDimensions {
		sum += d[k]
	}
	return mean(sum, len(QualityDimensions))
}

// measure scores the pages from their rule findings plus the critic's.
func measure(pages []core.Page, domain string, critic []core.QAIssue) *core.DesignQuality {
	q := &core.DesignQuality{Dimensions: map[string]int{}}
	sums := map[string]int{}
	total := 0
	for _, pg := range pages {
		issues := LintPage(pg, domain)
		for _, is := range critic {
			if is.PageID == pg.ID {
				issues = append(issues, is)
			}
		}
		d := scoreDims(issues)
		pq := core.PageQuality{PageID: pg.ID, Overall: overall(d), Dimensions: d, Metrics: PageMetrics(pg)}
		q.Pages = append(q.Pages, pq)
		total += pq.Overall
		for k, v := range d {
			sums[k] += v
		}
		q.Findings = append(q.Findings, issues...)
	}
	for _, is := range critic {
		if findPage(pages, is.PageID) == nil {
			q.Findings = append(q.Findings, is) // about the whole design: listed, not scored
		}
	}
	for _, k := range QualityDimensions {
		q.Dimensions[k] = mean(sums[k], len(pages))
	}
	q.Overall = mean(total, len(pages))
	for _, is := range q.Findings {
		if IsBlocker(is) {
			q.Reasons = append(q.Reasons, is)
		}
	}
	q.Decision = "pass"
	if len(q.Reasons) > 0 {
		q.Decision = "revision_required"
	}
	return q
}

// ScorePages is the rule score of one design version.
func ScorePages(pages []core.Page, domain string) core.QualityScore {
	q := measure(pages, domain, nil)
	return core.QualityScore{Overall: q.Overall, Dimensions: q.Dimensions, Blockers: len(q.Reasons)}
}

// currentQA returns the active check when it still describes the current
// pages: a revision's, or a new design's while no change was requested.
func currentQA(d *core.Design) *core.DesignQA {
	qa, initial := ActiveQA(d)
	if qa == nil || initial && len(d.Feedback) > 0 {
		return nil
	}
	return qa
}

// reviewedQA returns the current check when its critic round is finished:
// its findings describe the rendered pages as they are now.
func reviewedQA(d *core.Design) *core.DesignQA {
	qa := currentQA(d)
	if qa == nil || len(qa.Shots) == 0 || qa.Status == "running" || qa.Status == "skipped" {
		return nil
	}
	return qa
}

// MeasureDesign measures the current design: the rules on every page plus
// the findings of a finished critic round on the rendered pages.
func MeasureDesign(p *core.Project) *core.DesignQuality {
	d := p.Design
	if d == nil || len(d.Pages) == 0 {
		return nil
	}
	var critic []core.QAIssue
	qa := currentQA(d)
	reviewed := reviewedQA(d) != nil
	if reviewed {
		for _, is := range qa.Issues {
			if is.Source != "lint" {
				critic = append(critic, is)
			}
		}
	}
	q := measure(d.Pages, d.Domain, critic)
	planned := !slices.ContainsFunc(d.Pages, func(pg core.Page) bool { return pg.Intent == nil || pg.Plan == nil })
	switch {
	case reviewed && planned:
		q.Confidence = "high"
	case reviewed || planned:
		q.Confidence = "medium"
	default:
		q.Confidence = "low"
	}
	if qa != nil && len(qa.Compared) > 0 {
		c := qa.Compared[len(qa.Compared)-1]
		q.Compared = &c
	}
	switch {
	case len(q.Reasons) > 0:
		q.Recommendation = "Revisi dulu: " + q.Reasons[0].Problem
		if f := q.Reasons[0].Fix; f != "" {
			q.Recommendation += " " + f
		}
	case len(q.Findings) > 0:
		q.Recommendation = "Lolos gate; temuan minor bisa diperbaiki saat revisi berikutnya."
	default:
		q.Recommendation = "Lolos gate; tidak ada temuan."
	}
	q.MeasuredAt = time.Now().UTC()
	return q
}

func qualityVerdict(before, after core.QualityScore) string {
	switch {
	case after.Overall < before.Overall || after.Blockers > before.Blockers:
		return "degraded"
	case after.Overall > before.Overall || after.Blockers < before.Blockers:
		return "improved"
	}
	return "neutral"
}

// comparePatch scores the design before and after a critic patch and
// records it on the active check. A patch that lowers the score or adds a
// blocker is rejected.
func comparePatch(p *core.Project, after []core.Page) error {
	c := core.QualityComparison{Before: ScorePages(p.Design.Pages, p.Design.Domain), After: ScorePages(after, p.Design.Domain)}
	c.Verdict = qualityVerdict(c.Before, c.After)
	c.Rejected = c.Verdict == "degraded"
	if qa, _ := ActiveQA(p.Design); qa != nil {
		c.Round = qa.Round
		qa.Compared = append(qa.Compared, c)
	}
	if c.Rejected {
		return fmt.Errorf("patch memperburuk kualitas desain (skor %d → %d, blocker %d → %d)",
			c.Before.Overall, c.After.Overall, c.Before.Blockers, c.After.Blockers)
	}
	return nil
}

// QualityRecord is one line of the quality history: one page, one
// measurement.
type QualityRecord struct {
	Time       time.Time      `json:"time"`
	Revision   int            `json:"revision"` // change requests so far
	Round      int            `json:"round,omitempty"`
	PageID     string         `json:"page"`
	Devices    []string       `json:"devices,omitempty"`
	Overall    int            `json:"overall"`
	Dimensions map[string]int `json:"dimensions"`
	Decision   string         `json:"decision"`
	Findings   []core.QAIssue `json:"findings,omitempty"`
}

// QualityRecords turns the last measurement into history lines.
func QualityRecords(p *core.Project) []QualityRecord {
	d := p.Design
	if d == nil || d.Quality == nil {
		return nil
	}
	q := d.Quality
	qa := reviewedQA(d) // rules only: no round, no devices
	var out []QualityRecord
	for _, pq := range q.Pages {
		r := QualityRecord{Time: q.MeasuredAt, Revision: len(d.Feedback), PageID: pq.PageID, Overall: pq.Overall, Dimensions: pq.Dimensions, Decision: "pass"}
		if qa != nil {
			r.Round = qa.Round
			for _, s := range qa.Shots {
				if s.PageID == pq.PageID && !slices.Contains(r.Devices, s.Device) {
					r.Devices = append(r.Devices, s.Device)
				}
			}
		}
		for _, is := range q.Findings {
			if is.PageID == pq.PageID {
				r.Findings = append(r.Findings, is)
				if IsBlocker(is) {
					r.Decision = "revision_required"
				}
			}
		}
		out = append(out, r)
	}
	return out
}
