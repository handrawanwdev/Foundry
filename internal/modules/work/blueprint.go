package work

import (
	"strings"
	"unicode"

	"github.com/handrawanwdev/foundry/internal/core"
)

// NormalizeBlueprint cleans AI output: slug module ids, drop empty rows,
// drop unknown or cyclic depends_on. Nil stays nil.
func NormalizeBlueprint(b *core.Blueprint) *core.Blueprint {
	if b == nil {
		return nil
	}
	b.Modules = normModules(b.Modules)
	ids := map[string]bool{}
	for _, m := range b.Modules {
		ids[m.ID] = true
	}
	var ents []core.Entity
	for _, e := range b.Database.Entities {
		e.Name = strings.TrimSpace(e.Name)
		if e.Name == "" {
			continue
		}
		e.Fields = nonEmpty(e.Fields)
		e.Relates = nonEmpty(e.Relates)
		if e.Module != "" && !ids[slugID(e.Module)] {
			e.Module = slugID(e.Module)
		}
		ents = append(ents, e)
	}
	b.Database.Entities = ents
	var workers []core.Worker
	for _, w := range b.Workers {
		w.Name = strings.TrimSpace(w.Name)
		if w.Name == "" || strings.TrimSpace(w.Job) == "" {
			continue
		}
		switch strings.ToLower(w.Trigger) {
		case "event", "cron", "queue":
			w.Trigger = strings.ToLower(w.Trigger)
		default:
			w.Trigger = "queue"
		}
		workers = append(workers, w)
	}
	b.Workers = workers
	b.Overview.Users = nonEmpty(b.Overview.Users)
	b.Overview.Capabilities = nonEmpty(b.Overview.Capabilities)
	b.Authz.Permissions = nonEmpty(b.Authz.Permissions)
	b.Cache.Store = nonEmpty(b.Cache.Store)
	b.Cache.Never = nonEmpty(b.Cache.Never)
	return b
}

// MergeBlueprint overlays a change-scope design onto the main blueprint.
// Untouched modules, entities and architecture stay; silent replacement is
// recorded in kept so the user can review it.
func MergeBlueprint(prev, next *core.Blueprint, change *core.ChangePlan, intent core.WorkIntent) (*core.Blueprint, []string) {
	next = NormalizeBlueprint(next)
	if prev == nil {
		return next, nil
	}
	if next == nil {
		return prev, nil
	}
	if intent == core.IntentNewProject || (change != nil && change.Scope == core.BlueprintScopeFull) {
		return next, nil
	}
	added, modified, removed := changeModuleSets(change)
	var kept []string
	out := *next
	out.Modules = mergeModules(prev.Modules, next.Modules, added, modified, removed, &kept)
	out.Database.Entities = mergeEntities(prev.Database.Entities, next.Database.Entities, added, modified, removed, &kept)
	if intent != core.IntentArchitecture && strings.TrimSpace(prev.Architecture.Pattern) != "" {
		if next.Architecture.Pattern != "" && next.Architecture.Pattern != prev.Architecture.Pattern {
			kept = append(kept, "architecture.pattern="+prev.Architecture.Pattern)
		}
		out.Architecture = prev.Architecture
	}
	if !touchesAuth(change) && (prev.Auth != next.Auth || prev.Authz.Model != next.Authz.Model) {
		if prev.Auth.Method != "" && next.Auth.Method != "" && prev.Auth.Method != next.Auth.Method {
			kept = append(kept, "auth.method="+prev.Auth.Method)
		}
		out.Auth = prev.Auth
		out.Authz.Model = prev.Authz.Model
		out.Authz.Convention = prev.Authz.Convention
	}
	out.Authz.Permissions = unionStr(prev.Authz.Permissions, next.Authz.Permissions)
	out.Events.Events = mergeEvents(prev.Events.Events, next.Events.Events)
	out.Workers = mergeWorkers(prev.Workers, next.Workers, added, modified)
	return NormalizeBlueprint(&out), kept
}

func changeModuleSets(change *core.ChangePlan) (added, modified, removed map[string]bool) {
	added, modified, removed = map[string]bool{}, map[string]bool{}, map[string]bool{}
	if change == nil {
		return
	}
	mark := func(dst map[string]bool, it core.ChangeItem) {
		for _, s := range []string{it.Module, it.Target} {
			if s = slugID(s); s != "" {
				dst[s] = true
			}
		}
	}
	for _, it := range change.Items {
		switch it.Action {
		case core.ChangeAdd:
			mark(added, it)
		case core.ChangeModify, core.ChangeRefactor:
			mark(modified, it)
		case core.ChangeRemove:
			mark(removed, it)
		}
	}
	return
}

func mergeModules(prev, next []core.Module, added, modified, removed map[string]bool, kept *[]string) []core.Module {
	byID := map[string]core.Module{}
	order := []string{}
	for _, m := range prev {
		if removed[m.ID] {
			continue
		}
		byID[m.ID] = m
		order = append(order, m.ID)
	}
	for _, m := range next {
		if removed[m.ID] {
			continue
		}
		old, ok := byID[m.ID]
		if !ok {
			byID[m.ID] = m
			order = append(order, m.ID)
			continue
		}
		if added[m.ID] || modified[m.ID] {
			byID[m.ID] = m
			continue
		}
		if m.Responsibility != old.Responsibility || strings.Join(m.Interface, ",") != strings.Join(old.Interface, ",") {
			*kept = append(*kept, "module:"+old.ID)
		}
	}
	var out []core.Module
	seen := map[string]bool{}
	for _, id := range order {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, byID[id])
	}
	return out
}

func mergeEntities(prev, next []core.Entity, added, modified, removed map[string]bool, kept *[]string) []core.Entity {
	byName := map[string]core.Entity{}
	order := []string{}
	key := func(n string) string { return strings.ToLower(strings.TrimSpace(n)) }
	for _, e := range prev {
		k := key(e.Name)
		if removed[slugID(e.Name)] || removed[k] {
			continue
		}
		byName[k] = e
		order = append(order, k)
	}
	for _, e := range next {
		k := key(e.Name)
		if removed[slugID(e.Name)] || removed[k] {
			continue
		}
		old, ok := byName[k]
		if !ok {
			byName[k] = e
			order = append(order, k)
			continue
		}
		if added[slugID(e.Name)] || modified[slugID(e.Name)] || added[k] || modified[k] {
			byName[k] = e
			continue
		}
		if strings.Join(e.Fields, ",") != strings.Join(old.Fields, ",") {
			*kept = append(*kept, "entity:"+old.Name)
		}
	}
	var out []core.Entity
	for _, k := range order {
		out = append(out, byName[k])
	}
	return out
}

func mergeEvents(prev, next []core.EventSpec) []core.EventSpec {
	byType := map[string]core.EventSpec{}
	order := []string{}
	for _, e := range append(append([]core.EventSpec{}, prev...), next...) {
		t := strings.TrimSpace(e.Type)
		if t == "" {
			continue
		}
		if _, ok := byType[t]; !ok {
			order = append(order, t)
		}
		byType[t] = e
	}
	var out []core.EventSpec
	for _, t := range order {
		out = append(out, byType[t])
	}
	return out
}

func mergeWorkers(prev, next []core.Worker, added, modified map[string]bool) []core.Worker {
	byName := map[string]core.Worker{}
	order := []string{}
	for _, w := range prev {
		k := strings.ToLower(w.Name)
		byName[k] = w
		order = append(order, k)
	}
	for _, w := range next {
		k := strings.ToLower(w.Name)
		if _, ok := byName[k]; !ok {
			byName[k] = w
			order = append(order, k)
			continue
		}
		if added[slugID(w.Name)] || modified[slugID(w.Name)] {
			byName[k] = w
		}
	}
	var out []core.Worker
	for _, k := range order {
		out = append(out, byName[k])
	}
	return out
}

func touchesAuth(change *core.ChangePlan) bool {
	if change == nil {
		return false
	}
	hit := func(s string) bool {
		s = strings.ToLower(s)
		return strings.Contains(s, "auth") || strings.Contains(s, "jwt") || strings.Contains(s, "rbac") || strings.Contains(s, "permission")
	}
	for _, it := range change.Items {
		if hit(it.Target) || hit(it.Module) || hit(strings.Join(it.Files, " ")) {
			return true
		}
	}
	for _, f := range append(append(append([]core.ChangeFile{}, change.Files.Add...), change.Files.Modify...), change.Files.Remove...) {
		if hit(f.Path) {
			return true
		}
	}
	return false
}

func unionStr(a, b []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range append(append([]string{}, a...), b...) {
		s = strings.TrimSpace(s)
		if s == "" || seen[strings.ToLower(s)] {
			continue
		}
		seen[strings.ToLower(s)] = true
		out = append(out, s)
	}
	return out
}

func normModules(in []core.Module) []core.Module {
	var out []core.Module
	seen := map[string]int{}
	for _, m := range in {
		id := slugID(m.ID)
		if id == "" {
			id = slugID(m.Responsibility)
		}
		if id == "" || seen[id] > 0 {
			continue
		}
		seen[id] = 1
		m.ID = id
		m.Responsibility = strings.TrimSpace(m.Responsibility)
		m.Data = nonEmpty(m.Data)
		m.Interface = nonEmpty(m.Interface)
		m.DependsOn = nil
		out = append(out, m)
		if len(out) == 16 {
			break
		}
	}
	valid := map[string]bool{}
	for _, m := range out {
		valid[m.ID] = true
	}
	// Second pass: depends_on that exist and do not form a cycle.
	for _, m := range in {
		id := slugID(m.ID)
		if id == "" {
			id = slugID(m.Responsibility)
		}
		idx := -1
		for j := range out {
			if out[j].ID == id {
				idx = j
				break
			}
		}
		if idx < 0 {
			continue
		}
		var deps []string
		for _, d := range m.DependsOn {
			d = slugID(d)
			if d == "" || d == id || !valid[d] || containsStr(deps, d) {
				continue
			}
			if wouldCycle(out, id, d) {
				continue
			}
			out[idx].DependsOn = append(out[idx].DependsOn, d)
			deps = append(deps, d)
		}
	}
	return out
}

func wouldCycle(mods []core.Module, from, to string) bool {
	graph := map[string][]string{}
	for _, m := range mods {
		graph[m.ID] = append([]string{}, m.DependsOn...)
	}
	graph[from] = append(graph[from], to)
	seen := map[string]bool{}
	var walk func(string) bool
	walk = func(n string) bool {
		if n == from && seen[n] {
			return true
		}
		if seen[n] {
			return false
		}
		seen[n] = true
		for _, d := range graph[n] {
			if d == from || walk(d) {
				return true
			}
		}
		return false
	}
	return walk(to)
}

func slugID(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return unicode.ToLower(r)
		}
		return '-'
	}, strings.TrimSpace(s))
	s = strings.Trim(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	if len(s) > 32 {
		s = strings.TrimRight(s[:32], "-")
	}
	return s
}

func containsStr(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func fill(dst *string, v string) {
	if strings.TrimSpace(*dst) == "" {
		*dst = v
	}
}

func fillSlice(dst *[]string, v []string) {
	if len(*dst) == 0 {
		*dst = v
	}
}

// CompleteBlueprint fills empty overview and platform fields from the
// discovery and the platform, and mirrors the decided technology into the
// blueprint fields that name it. It designs nothing: modules, entities,
// workers, auth, data conventions, coding rules and every other
// architecture detail come from the AI analyst, never from a default here.
func CompleteBlueprint(p *core.Project) {
	if p == nil {
		return
	}
	b := p.Blueprint
	if b == nil {
		b = &core.Blueprint{}
	}
	d := p.Discovery
	if b.Overview.Goal == "" && d != nil {
		b.Overview.Goal = firstNonEmpty(d.Business, d.Summary)
	}
	if b.Overview.CoreDomain == "" && d != nil {
		b.Overview.CoreDomain = firstNonEmpty(d.Summary, p.Name)
	}
	if len(b.Overview.Users) == 0 && d != nil {
		b.Overview.Users = append([]string{}, d.Users...)
	}
	if len(b.Overview.Capabilities) == 0 {
		for _, f := range p.Features {
			b.Overview.Capabilities = append(b.Overview.Capabilities, f.Name)
		}
	}
	if b.Platform.Reason == "" && p.Platform != nil {
		b.Platform.Reason = p.Platform.Reason
	}
	if len(b.Platform.Devices) == 0 && p.Platform != nil {
		for _, t := range p.Platform.Targets {
			b.Platform.Devices = append(b.Platform.Devices, t.Kind)
		}
	}
	mirrorStack(b, p.Stack)
	mirrorSurvey(b, p.Survey)
	fillSlice(&b.Docs, defaultDocs)
	fillSlice(&b.AIGuidelines, defaultAIGuidelines)
	p.Blueprint = NormalizeBlueprint(b)
}

// mirrorStack writes the decided technology into the blueprint fields that
// name a technology, so the two never disagree. Only decided areas are
// written; nothing is guessed from a technology's name.
func mirrorStack(b *core.Blueprint, st *core.TechStack) {
	if st == nil {
		return
	}
	if v := st.Choice(core.TechArchitecture); v != "" {
		b.Architecture.Pattern = v
		if b.Architecture.Reason == "" {
			if d := st.Decision(core.TechArchitecture); d != nil {
				b.Architecture.Reason = d.Reason
			}
		}
	}
	if v := st.Choice(core.TechDatabase); v != "" {
		b.Database.Engine = v
	}
	if v := st.Choice(core.TechDeployment); v != "" {
		b.Platform.Deployment = v
	}
	switch v := st.Choice(core.TechCache); v {
	case "":
	case core.TechNone:
		b.Cache.Use = "none"
	default:
		b.Cache.Use = v
	}
	if v := st.Choice(core.TechMessaging); v != "" && v != core.TechNone {
		b.Realtime.Broker = v
	}
	switch v := st.Choice(core.TechStorage); v {
	case "":
	case core.TechNone:
		b.Storage.Strategy, b.Storage.Provider = "none", ""
	default:
		b.Storage.Provider = v
	}
	if v := st.Choice(core.TechAPI); v != "" && b.Coding.API.Style == "" {
		b.Coding.API.Style = v
	}
}

// mirrorSurvey writes the latency target the user gave in the survey; no
// target is invented without one.
func mirrorSurvey(b *core.Blueprint, sv *core.Survey) {
	if sv == nil || b.Performance.Budget != "" {
		return
	}
	switch sv.Latency {
	case core.SurveyLatency200:
		b.Performance.Budget = "< 200 ms p95"
	case core.SurveyLatency100:
		b.Performance.Budget = "< 100 ms p95"
	}
}

// AppLike reports whether the product has an app or server part (web,
// mobile, desktop or backend). No platform yet counts as one.
func AppLike(p *core.Project) bool {
	if p.Platform == nil || len(p.Platform.Targets) == 0 {
		return true
	}
	for _, t := range p.Platform.Targets {
		switch t.Kind {
		case core.PlatformWeb, core.PlatformAndroid, core.PlatformIOS, core.PlatformDesktop, core.PlatformBackend:
			return true
		}
	}
	return false
}

// ResetBlueprintTech clears the blueprint details written for the
// technology of the changed areas: they described the old choice. The AI
// analyst writes them again for the new one; nothing else changes.
func ResetBlueprintTech(p *core.Project, changes []core.TechChange) {
	b := p.Blueprint
	if b == nil {
		return
	}
	for _, c := range changes {
		switch c.Area {
		case core.TechFrontend:
			b.Frontend = core.FrontendArch{}
		case core.TechBackend:
			b.Backend = core.BackendArch{}
			b.Database.ORM = ""
		case core.TechDatabase:
			b.Database.ORM = ""
		case core.TechCache:
			b.Cache = core.CacheArch{}
			b.Auth.Session = ""
			b.Performance.Health, b.Performance.Caching = "", ""
			b.Realtime.Broker = ""
			b.Events.Ephemeral = ""
		case core.TechArchitecture:
			b.Architecture.Reason, b.Architecture.Tradeoff = "", ""
		case core.TechMessaging:
			b.Realtime.Broker = ""
			b.Events.Durable, b.Events.Ephemeral, b.Events.Outbox = "", "", ""
		case core.TechStorage:
			b.Storage = core.StorageArch{}
		case core.TechWorkers:
			b.Backend.Background = ""
		case core.TechAPI:
			b.Coding.API.Style = ""
		}
	}
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

// defaultDocs is the export package an agent reads; the order of work
// comes from the command workflow, not from fixed phases.
var defaultDocs = []string{
	"AGENTS.md", "docs/foundry/README.md", "docs/foundry/manifest.json", "docs/foundry/blueprint/",
	"docs/foundry/workflow/", "docs/foundry/adr/",
}

var defaultAIGuidelines = []string{
	"Ikuti docs/foundry/README.md dan kerjakan command di docs/foundry/workflow/ sesuai gelombang dan dependency",
	"Blueprint dan stack adalah source of truth; jangan ganti arsitektur, framework, atau database tanpa keputusan Foundry",
	"Ikuti batas module dan aturan struktur di blueprint; data dan logic milik module tidak diakses langsung dari module lain",
	"Ikuti struktur folder yang diputuskan; jangan membuat folder di luar rencana struktur repository",
	"Jangan menduplikasi logic; perluas yang sudah ada",
	"Laporkan perubahan kontrak API, skema, atau alur agar blueprint di Foundry diperbarui",
	"Naming konsisten dengan konvensi yang ditulis di blueprint",
	"Perubahan struktur, framework, atau migration lama: tulis ADR lalu berhenti menunggu persetujuan",
}
