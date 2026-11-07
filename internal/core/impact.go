package core

import (
	"fmt"
	"strings"
)

// EnrichChange fills observed impact and blueprint conflicts onto the
// change plan. AI-written impact stays; observed rows are added when they
// are not already present. Nil-safe.
func EnrichChange(p *Project) {
	if p == nil || p.Change == nil {
		return
	}
	ch := p.Change
	obs := ObservedImpact(p)
	for _, im := range obs {
		if !hasImpact(ch.Impact, im) {
			ch.Impact = append(ch.Impact, im)
		}
	}
	for _, cf := range ChangeConflicts(p) {
		if !hasConflict(ch.Conflicts, cf) {
			ch.Conflicts = append(ch.Conflicts, cf)
		}
	}
	p.Change = NormalizeChangePlan(ch)
	if p.Change == nil {
		p.Change = ch
	}
}

// ObservedImpact is the effect of the change on modules connected by
// observed dependencies. Nothing here is estimated.
func ObservedImpact(p *Project) []ChangeImpact {
	if p == nil || p.Change == nil || p.Change.Empty() {
		return nil
	}
	ctx := (*ProjectContext)(nil)
	if p.Structure != nil {
		ctx = p.Structure.Context
	}
	touched := expandTouched(p, touchedModules(p))
	if len(touched) == 0 && len(changePaths(p.Change)) == 0 {
		return nil
	}
	var out []ChangeImpact
	seen := map[string]bool{}
	add := func(im ChangeImpact) {
		im.Source = ConfidenceObserved
		key := im.Kind + "|" + im.Area + "|" + im.Message + "|" + im.Path
		if seen[key] || im.Message == "" {
			return
		}
		seen[key] = true
		out = append(out, im)
	}
	for mod := range touched {
		add(ChangeImpact{Area: "module", Kind: ImpactDirect, Path: mod,
			Message: fmt.Sprintf("modul %s terdampak langsung", mod), Risk: "medium",
			Evidence: "change plan"})
	}
	if ctx != nil {
		for _, d := range ctx.Deps {
			if d.Confidence != "" && d.Confidence != ConfidenceObserved {
				continue
			}
			if !touched[d.To] {
				continue
			}
			if touched[d.From] {
				continue
			}
			add(ChangeImpact{Area: "module", Kind: ImpactIndirect, Path: d.From,
				Message: fmt.Sprintf("modul %s bergantung pada %s", d.From, d.To), Risk: "medium",
				Evidence: d.Evidence})
		}
	}
	for _, path := range changePaths(p.Change) {
		lp := strings.ToLower(path)
		switch {
		case strings.Contains(lp, "schema.prisma") || strings.HasSuffix(lp, ".sql") || strings.Contains(lp, "migration"):
			add(ChangeImpact{Area: "database", Kind: ImpactMigration, Path: path,
				Message: "perubahan skema atau migrasi", Risk: "high", Evidence: path})
			add(ChangeImpact{Area: "rollback", Kind: ImpactRollback, Path: path,
				Message: "migrasi membutuhkan rencana rollback", Risk: "high", Evidence: path})
		case strings.Contains(lp, "openapi") || strings.Contains(lp, "swagger") || strings.HasSuffix(lp, ".proto") ||
			strings.Contains(lp, "router") || strings.Contains(lp, "route") || strings.Contains(lp, "handler"):
			add(ChangeImpact{Area: "api", Kind: ImpactAPI, Path: path,
				Message: "kontrak API atau rute berubah", Risk: "high", Evidence: path})
			if consumers := apiConsumers(ctx, path, touched); len(consumers) > 0 {
				add(ChangeImpact{Area: "api", Kind: ImpactBreaking, Path: path,
					Message: "perubahan API dengan konsumen: " + strings.Join(consumers, ", "), Risk: "high",
					Evidence: path})
			}
		case strings.Contains(lp, "auth") || strings.Contains(lp, "jwt") || strings.Contains(lp, "rbac") ||
			strings.Contains(lp, "permission") || strings.Contains(lp, "session"):
			add(ChangeImpact{Area: "security", Kind: ImpactSecurity, Path: path,
				Message: "berkas autentikasi atau otorisasi berubah", Risk: "high", Evidence: path})
		}
	}
	if len(p.Change.Files.Modify) > 0 || len(p.Change.Files.Remove) > 0 {
		add(ChangeImpact{Area: "regression", Kind: ImpactRegression, Message: "perubahan file existing berisiko regresi",
			Risk: "medium", Evidence: "files.modify/remove"})
	}
	if len(p.Change.Files.Remove) > 0 {
		add(ChangeImpact{Area: "rollback", Kind: ImpactRollback, Message: "penghapusan file membutuhkan rencana rollback",
			Risk: "high", Evidence: "files.remove"})
	}
	return out
}

// ChangeConflicts compares the change plan with the main blueprint and
// decided decisions. Duplicates and silent overwrites are recorded here.
func ChangeConflicts(p *Project) []ChangeConflict {
	if p == nil || p.Change == nil {
		return nil
	}
	var out []ChangeConflict
	seenFeat := map[string]bool{}
	if ctx := contextOf(p); ctx != nil {
		for _, f := range ctx.Features {
			seenFeat[strings.ToLower(strings.TrimSpace(f.Text))] = true
		}
	}
	bpMods := map[string]bool{}
	bpEnts := map[string]bool{}
	if p.Blueprint != nil {
		for _, m := range p.Blueprint.Modules {
			bpMods[strings.ToLower(m.ID)] = true
		}
		for _, e := range p.Blueprint.Database.Entities {
			bpEnts[strings.ToLower(e.Name)] = true
		}
	}
	dependents := reverseDeps(contextOf(p))
	aliases := moduleAliases(p)
	for _, it := range p.Change.Items {
		target := strings.ToLower(strings.TrimSpace(it.Target))
		mod := strings.ToLower(strings.TrimSpace(it.Module))
		switch it.Action {
		case ChangeAdd:
			if seenFeat[target] {
				out = append(out, ChangeConflict{Kind: "duplicate", Where: "item:" + it.Target,
					Message: fmt.Sprintf("fitur %q sudah tercatat di repository; jangan dibuat ulang", it.Target),
					Keep:    it.Target, Reason: "existing documented feature"})
			}
			if bpMods[target] && (mod == "" || mod == target) {
				out = append(out, ChangeConflict{Kind: "duplicate", Where: "item:" + it.Target,
					Message: fmt.Sprintf("modul %q sudah ada di blueprint utama", it.Target),
					Keep:    it.Target, Reason: "main blueprint module"})
			}
			if bpEnts[target] {
				out = append(out, ChangeConflict{Kind: "duplicate", Where: "item:" + it.Target,
					Message: fmt.Sprintf("entity %q sudah ada di model data", it.Target),
					Keep:    it.Target, Reason: "main blueprint entity"})
			}
		case ChangeRemove:
			key := mod
			if key == "" {
				key = target
			}
			var ds []string
			seenD := map[string]bool{}
			for _, k := range aliases[key] {
				for _, d := range dependents[k] {
					if seenD[d] {
						continue
					}
					seenD[d] = true
					ds = append(ds, d)
				}
			}
			if len(ds) > 0 {
				out = append(out, ChangeConflict{Kind: "dependents", Where: "item:" + it.Target,
					Message: fmt.Sprintf("menghapus %s memutus %s", it.Target, strings.Join(ds, ", ")),
					Keep:    it.Target, Reason: "observed dependents"})
			}
		}
	}
	for _, k := range p.Change.Kept {
		out = append(out, ChangeConflict{Kind: "decision", Where: "blueprint",
			Message: "keputusan blueprint utama dipertahankan: " + k, Keep: k,
			Reason: "change must not silently replace the main blueprint"})
	}
	return out
}

func contextOf(p *Project) *ProjectContext {
	if p == nil || p.Structure == nil {
		return nil
	}
	return p.Structure.Context
}

func touchedModules(p *Project) map[string]bool {
	out := map[string]bool{}
	if p.Change == nil {
		return out
	}
	for _, it := range p.Change.Items {
		if it.Action == ChangeReuse {
			continue
		}
		if m := strings.TrimSpace(it.Module); m != "" {
			out[m] = true
		}
		for _, f := range it.Files {
			if m := moduleOfPath(p, f); m != "" {
				out[m] = true
			}
		}
	}
	for _, f := range append(append(append([]ChangeFile{}, p.Change.Files.Add...), p.Change.Files.Modify...), p.Change.Files.Remove...) {
		if m := moduleOfPath(p, f.Path); m != "" {
			out[m] = true
		}
	}
	return out
}

func changePaths(c *ChangePlan) []string {
	if c == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, it := range c.Items {
		for _, f := range it.Files {
			add(f)
		}
	}
	for _, f := range c.Files.Add {
		add(f.Path)
	}
	for _, f := range c.Files.Modify {
		add(f.Path)
	}
	for _, f := range c.Files.Remove {
		add(f.Path)
	}
	return out
}

func apiConsumers(ctx *ProjectContext, path string, touched map[string]bool) []string {
	if ctx == nil {
		return nil
	}
	owner := ""
	lp := strings.ToLower(path)
	for _, m := range ctx.Modules {
		if strings.HasPrefix(lp, strings.ToLower(m.Path)) {
			if len(m.Path) >= len(owner) {
				owner = m.Path
			}
		}
	}
	if owner == "" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, d := range ctx.Deps {
		if d.To != owner && d.To != path {
			continue
		}
		if touched[d.From] || seen[d.From] {
			continue
		}
		seen[d.From] = true
		out = append(out, d.From)
	}
	return out
}

func expandTouched(p *Project, touched map[string]bool) map[string]bool {
	out := map[string]bool{}
	aliases := moduleAliases(p)
	for id := range touched {
		for _, k := range aliases[strings.ToLower(id)] {
			out[k] = true
		}
		out[id] = true
	}
	return out
}

func moduleAliases(p *Project) map[string][]string {
	out := map[string][]string{}
	add := func(keys ...string) {
		seen := map[string]bool{}
		var clean []string
		take := func(k string) {
			k = strings.TrimSpace(k)
			if k == "" || seen[strings.ToLower(k)] {
				return
			}
			seen[strings.ToLower(k)] = true
			clean = append(clean, k)
		}
		for _, k := range keys {
			for _, e := range out[strings.ToLower(k)] {
				take(e)
			}
			take(k)
		}
		for _, k := range clean {
			out[strings.ToLower(k)] = clean
			out[k] = clean
		}
	}
	if ctx := contextOf(p); ctx != nil {
		for _, m := range ctx.Modules {
			add(m.Path, m.Name)
			if i := strings.LastIndex(m.Path, "/"); i >= 0 {
				add(m.Path, m.Name, m.Path[i+1:])
			}
		}
	}
	if p != nil && p.Blueprint != nil {
		for _, m := range p.Blueprint.Modules {
			add(m.ID)
		}
	}
	return out
}

func reverseDeps(ctx *ProjectContext) map[string][]string {
	out := map[string][]string{}
	if ctx == nil {
		return out
	}
	for _, d := range ctx.Deps {
		if d.From == "" || d.To == "" {
			continue
		}
		out[d.To] = append(out[d.To], d.From)
		out[strings.ToLower(d.To)] = append(out[strings.ToLower(d.To)], d.From)
	}
	return out
}

func hasImpact(in []ChangeImpact, im ChangeImpact) bool {
	for _, x := range in {
		if x.Area == im.Area && x.Message == im.Message {
			return true
		}
		if im.Kind != "" && x.Kind == im.Kind && x.Path == im.Path && x.Path != "" {
			return true
		}
	}
	return false
}

func hasConflict(in []ChangeConflict, cf ChangeConflict) bool {
	for _, x := range in {
		if x.Kind == cf.Kind && x.Message == cf.Message {
			return true
		}
	}
	return false
}
