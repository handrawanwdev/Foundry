package uikit

import (
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// sectionKinds maps a section pattern onto the slot kind it renders as.
var sectionKinds = map[string]string{
	"table": "table", "list": "list", "feed": "list", "cards": "cards", "form": "form", "settings": "form",
	"auth": "form", "detail": "detail", "dashboard": "metrics", "report": "chart", "tabs": "tabs",
	"search": "toolbar", "wizard": "steps", "calendar": "chart", "map": "chart",
}

// AssignSlots places every section of a page or shell of pattern in a slot
// of its template. A slot the designer named stays when the template has
// it; otherwise the section goes to the slot whose id, words or kind fit
// it best, else to the first required slot still empty, else to the main
// slot. Sections of a pattern without a template keep no slot. The input
// is not changed.
func (k *Kit) AssignSlots(pattern string, secs []core.Section) []core.Section {
	out := append([]core.Section(nil), secs...)
	pat, ok := k.Pattern(pattern)
	if !ok {
		for i := range out {
			out[i].Slot = ""
		}
		return out
	}
	filled := map[string]bool{}
	for i := range out {
		s := strings.ToLower(strings.TrimSpace(out[i].Slot))
		if _, ok := pat.Slot(s); ok {
			out[i].Slot = s
			filled[s] = true
		} else {
			out[i].Slot = ""
		}
	}
	for i := range out {
		if out[i].Slot != "" {
			continue
		}
		best, score := "", 0
		for _, sl := range pat.Slots {
			if sc := slotScore(sl, out[i], filled[sl.ID]); sc > score {
				best, score = sl.ID, sc
			}
		}
		if best == "" {
			for _, sl := range pat.Slots {
				if sl.Required && !filled[sl.ID] {
					best = sl.ID
					break
				}
			}
		}
		if best == "" {
			best = mainSlot(pat).ID
		}
		out[i].Slot = best
		filled[best] = true
	}
	fillRequired(pat, secs, out)
	return out
}

// fillRequired moves a section into every required slot left empty: the
// best-fitting section of a slot that is optional or holds several
// sections, preferring sections the designer did not place.
func fillRequired(pat *Pattern, in, out []core.Section) {
	for _, sl := range pat.Slots {
		if !sl.Required || slices.ContainsFunc(out, func(s core.Section) bool { return s.Slot == sl.ID }) {
			continue
		}
		count := map[string]int{}
		for _, s := range out {
			count[s.Slot]++
		}
		best, score := -1, -1
		for i, s := range out {
			from, _ := pat.Slot(s.Slot)
			if from.Required && count[s.Slot] < 2 {
				continue
			}
			sc := slotScore(sl, s, false) * 4
			if in[i].Slot == "" {
				sc += 2 // not the designer's choice
			}
			switch core.ContentKind(s) {
			case "table", "chart", "cards":
				sc++ // a large block carries a page's main slot best
			}
			if sc > score {
				best, score = i, sc
			}
		}
		if best >= 0 {
			out[best].Slot = sl.ID
		}
	}
}

// mainSlot is the slot that carries the page's content: the first
// required one, else the last.
func mainSlot(p *Pattern) Slot {
	for _, s := range p.Slots {
		if s.Required {
			return s
		}
	}
	return p.Slots[len(p.Slots)-1]
}

func slotScore(sl Slot, s core.Section, taken bool) int {
	id, name := strings.ToLower(s.ID), strings.ToLower(s.Name)
	content := strings.ToLower(strings.Join(s.Content, " "))
	score := 0
	switch {
	case id == sl.ID:
		score += 12
	case strings.Contains(id, sl.ID) || strings.Contains(name, strings.ToLower(sl.Name)):
		score += 6
	}
	for _, w := range sl.Match {
		switch {
		case strings.Contains(id, w) || strings.Contains(name, w):
			score += 5
		case strings.Contains(content, w):
			score++
		}
	}
	if kind := sectionKinds[s.Pattern]; kind != "" && kind == sl.Kind {
		score += 4
	} else if kind := core.ContentKind(s); kind != "" && (kind == sl.Kind || kind == "list" && sl.Kind == "activity") {
		score += 4
	}
	// Prefer an empty slot when the fit is otherwise equal.
	if score > 0 && !taken {
		score++
	}
	return score
}
