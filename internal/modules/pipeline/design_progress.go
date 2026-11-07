package pipeline

import (
	"fmt"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

const (
	secDiscovery = "discovery"
	secTech      = "tech"
	secArch      = "arch"
	secRepo      = "repo"
)

func tracksBlueprint(p *core.Project, kind string) bool {
	switch kind {
	case work.KindDiscovery, work.KindArtifacts, work.KindDesign, work.KindPlan, work.KindTech, work.KindStructure, work.KindRepo:
	default:
		return false
	}
	if p.BlueprintProgress != nil {
		switch p.BlueprintProgress.Status {
		case core.BPTaskRunning, core.BPTaskFailed:
			return true
		}
	}
	if p.BlueprintBatch > 0 {
		return true
	}
	return kind == work.KindDiscovery && p.BlueprintStarted != nil
}

func ensureBlueprintProgress(p *core.Project) {
	now := time.Now().UTC()
	started := now
	if p.BlueprintStarted != nil {
		started = *p.BlueprintStarted
	}
	t := p.BlueprintProgress
	if t != nil && t.Status == core.BPTaskFailed {
		t.ResumeRunning()
		mergeSectionList(p)
		return
	}
	if t != nil && t.Status == core.BPTaskRunning {
		mergeSectionList(p)
		return
	}
	id := fmt.Sprintf("blueprint-%d", started.Unix())
	p.BlueprintProgress = core.NewBlueprintProgress(id, buildSections(p), started)
}

func buildSections(p *core.Project) []core.BlueprintSection {
	out := []core.BlueprintSection{{
		ID: secDiscovery, Name: "Penjelasan Sistem", Kind: work.KindDiscovery, Status: core.BPPending,
	}}
	docs := spec.Artifacts(p)
	for _, a := range core.Coverage(p, docs) {
		unknown := p.Discovery == nil && (a.ID == core.AreaData || a.ID == core.AreaIntegration)
		if a.Status == core.AreaNotApplicable && !unknown {
			continue
		}
		kind := work.KindArtifacts
		if a.ID == core.AreaUI {
			kind = work.KindDesign
		}
		out = append(out, core.BlueprintSection{ID: a.ID, Name: a.Title, Kind: kind, Status: core.BPPending})
	}
	if work.AppLike(p) {
		out = append(out, core.BlueprintSection{ID: secTech, Name: "Teknologi", Kind: work.KindTech, Status: core.BPPending})
		existing := p.Structure != nil && p.Structure.Mode == core.StructureExisting
		if !existing {
			out = append(out, core.BlueprintSection{ID: secArch, Name: "Arsitektur", Kind: work.KindStructure, Status: core.BPPending})
			out = append(out, core.BlueprintSection{ID: secRepo, Name: "Struktur Repository", Kind: work.KindRepo, Status: core.BPPending})
		}
	}
	stampFromTruth(p, out)
	return out
}

func stampFromTruth(p *core.Project, sections []core.BlueprintSection) {
	for i := range sections {
		s := &sections[i]
		if sectionSkipped(p, s.ID) {
			s.Status = core.BPSkipped
			continue
		}
		if sectionLanded(p, s.ID) {
			s.Status = core.BPCompleted
		}
	}
}

func mergeSectionList(p *core.Project) {
	t := p.BlueprintProgress
	if t == nil {
		return
	}
	prev := make(map[string]core.BlueprintSection, len(t.Sections))
	for _, s := range t.Sections {
		prev[s.ID] = s
	}
	next := buildSections(p)
	now := time.Now().UTC()
	for i := range next {
		old, ok := prev[next[i].ID]
		if !ok {
			continue
		}
		// Truth wins for landed/skipped; otherwise keep the live status.
		if next[i].Status == core.BPCompleted || next[i].Status == core.BPSkipped {
			next[i].Activity, next[i].StartedAt, next[i].EndedAt = old.Activity, old.StartedAt, old.EndedAt
			if next[i].EndedAt == nil {
				end := now
				next[i].EndedAt = &end
			}
			continue
		}
		next[i].Status = old.Status
		next[i].Activity = old.Activity
		next[i].Error = old.Error
		next[i].StartedAt = old.StartedAt
		next[i].EndedAt = old.EndedAt
	}
	t.Sections = next
}

func sectionForJob(p *core.Project, kind string) string {
	switch kind {
	case work.KindDiscovery:
		return secDiscovery
	case work.KindDesign:
		return core.AreaUI
	case work.KindArtifacts:
		if p.ArtifactRequest != nil && p.ArtifactRequest.Area != "" {
			return p.ArtifactRequest.Area
		}
	case work.KindTech:
		return secTech
	case work.KindStructure:
		return secArch
	case work.KindRepo:
		return secRepo
	case work.KindPlan:
		switch work.CurrentPlanPart(p) {
		case work.KindTech:
			return secTech
		case work.KindStructure:
			return secArch
		case work.KindRepo:
			return secRepo
		}
	}
	return ""
}

func sectionLanded(p *core.Project, id string) bool {
	switch id {
	case secDiscovery:
		return p.Discovery != nil
	case core.AreaUI:
		return p.UI != nil && p.UI.Written()
	case secTech:
		return p.Stack != nil && p.Stack.Plan != nil && (p.Stack.PlanRequest == nil)
	case secArch:
		return p.Structure != nil && p.Structure.Plan != nil && p.Structure.PlanRequest == nil
	case secRepo:
		return p.Structure != nil && p.Structure.Repo != nil && p.Structure.RepoRequest == nil
	}
	for _, a := range core.Coverage(p, spec.Artifacts(p)) {
		if a.ID == id {
			return a.Status == core.AreaCovered
		}
	}
	return false
}

func sectionSkipped(p *core.Project, id string) bool {
	switch id {
	case secDiscovery, secTech, secArch, secRepo:
		return false
	}
	for _, a := range core.Coverage(p, spec.Artifacts(p)) {
		if a.ID == id {
			return a.Status == core.AreaSkipped
		}
	}
	return false
}

func generatingActivity(id string) string {
	switch id {
	case secDiscovery:
		return "Membaca kebutuhan sistem dan mengidentifikasi aktor"
	case core.AreaUseCases:
		return "Mengidentifikasi aktor, alur, dan hak akses"
	case core.AreaData:
		return "Menganalisis kepemilikan data dan sumber kebenaran"
	case core.AreaSecurity:
		return "Menganalisis ancaman dan model keamanan"
	case core.AreaIntegration:
		return "Menganalisis integrasi dan dependensi eksternal"
	case core.AreaOperations:
		return "Menganalisis kebutuhan observability dan reliability"
	case core.AreaMigration:
		return "Menganalisis migrasi dan kompatibilitas"
	case core.AreaAcceptance:
		return "Menyusun acceptance criteria dan definition of done"
	case core.AreaUI:
		return "Menyusun aturan desain dan spesifikasi layout"
	case secTech:
		return "Merancang teknologi"
	case secArch:
		return "Menganalisis arsitektur aplikasi"
	case secRepo:
		return "Merancang struktur repository"
	}
	if a := core.AreaByID(id); a != nil {
		return "Menyusun " + a.Title
	}
	return "Menghasilkan output blueprint"
}

func sectionTitle(p *core.Project, id string) string {
	if p != nil && p.BlueprintProgress != nil {
		for _, s := range p.BlueprintProgress.Sections {
			if s.ID == id {
				return s.Name
			}
		}
	}
	switch id {
	case secDiscovery:
		return "Penjelasan Sistem"
	case secTech:
		return "Teknologi"
	case secArch:
		return "Arsitektur"
	case secRepo:
		return "Struktur Repository"
	}
	if a := core.AreaByID(id); a != nil {
		return a.Title
	}
	return id
}

func repairingActivity(id string, p *core.Project) string {
	return "Memperbaiki output " + sectionTitle(p, id)
}

func validatingActivity(id string, p *core.Project) string {
	return "Memvalidasi output " + sectionTitle(p, id)
}

func sectionRepairing(p *core.Project, id string) bool {
	switch id {
	case core.AreaUI:
		return p.UI != nil && p.UI.Request != nil && p.UI.Request.Rejected != ""
	case secTech:
		return p.Stack != nil && p.Stack.PlanRequest != nil && p.Stack.PlanRequest.Rejected != ""
	case secArch:
		return p.Structure != nil && p.Structure.PlanRequest != nil && p.Structure.PlanRequest.Rejected != ""
	case secRepo:
		return p.Structure != nil && p.Structure.RepoRequest != nil && p.Structure.RepoRequest.Rejected != ""
	}
	if p.ArtifactRequest != nil && p.ArtifactRequest.Area == id {
		return p.ArtifactRequest.Rejected != ""
	}
	return false
}

func trackJobStart(p *core.Project, kind string, isMCP bool) {
	if !tracksBlueprint(p, kind) {
		return
	}
	ensureBlueprintProgress(p)
	id := sectionForJob(p, kind)
	if id == "" {
		return
	}
	now := time.Now().UTC()
	activity := generatingActivity(id)
	if isMCP {
		activity = "Menunggu AI Host (MCP)"
	}
	s := p.BlueprintProgress.Section(id)
	retry := sectionRepairing(p, id) || (s != nil && (s.Status == core.BPFailed || s.Status == core.BPRetrying))
	if retry {
		p.BlueprintProgress.RetrySection(id, repairingActivity(id, p), now)
		if isMCP {
			p.BlueprintProgress.SetActivity(id, activity, now)
		}
		return
	}
	p.BlueprintProgress.StartSection(id, activity, now)
}

func syncCompleted(p *core.Project) {
	if p.BlueprintProgress == nil || p.BlueprintProgress.Status != core.BPTaskRunning {
		return
	}
	mergeSectionList(p)
	now := time.Now().UTC()
	ids := make([]string, len(p.BlueprintProgress.Sections))
	for i, s := range p.BlueprintProgress.Sections {
		ids[i] = s.ID
	}
	for _, id := range ids {
		s := p.BlueprintProgress.Section(id)
		if s == nil || s.Status == core.BPCompleted || s.Status == core.BPSkipped {
			continue
		}
		if sectionSkipped(p, id) {
			p.BlueprintProgress.SkipSection(id, now)
			continue
		}
		if sectionLanded(p, id) {
			p.BlueprintProgress.CompleteSection(id, now)
		}
	}
}

func finishTaskIfDone(p *core.Project) {
	t := p.BlueprintProgress
	if t == nil || t.Status != core.BPTaskRunning {
		return
	}
	if p.BlueprintBatch > 0 {
		return
	}
	if p.Job != nil && (p.Job.Status == core.JobRunning || p.Job.Status == core.JobWaiting) {
		return
	}
	if p.Job != nil && p.Job.Status == core.JobFailed {
		return
	}
	for _, s := range t.Sections {
		if s.Status == core.BPInProgress || s.Status == core.BPRetrying {
			return
		}
	}
	t.CompleteTask(time.Now().UTC())
}

func trackFail(p *core.Project, kind string, err error) {
	if p.BlueprintProgress == nil || p.BlueprintProgress.Status != core.BPTaskRunning {
		return
	}
	now := time.Now().UTC()
	msg := safeProgressError(err)
	if id := sectionForJob(p, kind); id != "" {
		p.BlueprintProgress.FailSection(id, msg, now)
	}
	p.BlueprintProgress.FailTask(msg, now)
}

func trackRetry(p *core.Project, kind string) {
	if p.BlueprintProgress == nil || p.BlueprintProgress.Status != core.BPTaskRunning {
		return
	}
	id := sectionForJob(p, kind)
	if id == "" {
		return
	}
	p.BlueprintProgress.RetrySection(id, repairingActivity(id, p), time.Now().UTC())
}

func (s *Service) markValidating(kind string) {
	p, err := s.Store.Load()
	if err != nil || p.BlueprintProgress == nil || p.BlueprintProgress.Status != core.BPTaskRunning {
		return
	}
	id := sectionForJob(p, kind)
	if id == "" {
		return
	}
	s.Store.Update(func(p *core.Project) error {
		if p.BlueprintProgress == nil || p.BlueprintProgress.Status != core.BPTaskRunning {
			return nil
		}
		id := sectionForJob(p, kind)
		if id == "" {
			return nil
		}
		p.BlueprintProgress.SetActivity(id, validatingActivity(id, p), time.Now().UTC())
		return nil
	})
}

func (s *Service) markBusyWait(kind string) {
	s.Store.Update(func(p *core.Project) error {
		if p.BlueprintProgress == nil || p.BlueprintProgress.Status != core.BPTaskRunning {
			return nil
		}
		id := sectionForJob(p, kind)
		p.BlueprintProgress.SetActivity(id, "Server AI sibuk, menunggu untuk mencoba kembali", time.Now().UTC())
		return nil
	})
}

func safeProgressError(err error) string {
	if err == nil {
		return "Proses gagal"
	}
	msg := err.Error()
	if len(msg) > 400 {
		msg = msg[:400] + "…"
	}
	return msg
}
