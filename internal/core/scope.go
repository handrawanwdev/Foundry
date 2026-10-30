package core

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode"
)

// The blueprint is complete when every area that applies to the project
// is covered by an artifact or left out on purpose with a reason. Areas
// that do not apply (migration of a new project, integration without
// external systems) need nothing.

// Blueprint areas.
const (
	AreaUI          = "ui"
	AreaUseCases    = "use-cases"
	AreaData        = "data"
	AreaSecurity    = "security"
	AreaIntegration = "integration"
	AreaOperations  = "operations"
	AreaMigration   = "migration"
	AreaAcceptance  = "acceptance"
)

// Area is one part of the blueprint. Artifact is the id the area's
// artifact gets by default; built-in areas are rendered from structured
// data, the others are Markdown artifacts the AI or the user writes.
type Area struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Artifacts []string `json:"artifacts"`
	Builtin   bool     `json:"builtin,omitempty"`
	// Skippable areas may be left out with a reason even when they apply.
	Skippable bool `json:"skippable,omitempty"`
	// Topics are what the area's artifact covers, for the AI and the UI.
	Topics []string `json:"topics"`
}

var Areas = []Area{
	{ID: AreaUI, Title: "Aturan Desain & Spesifikasi Layout", Artifacts: []string{ArtifactDesignRules, ArtifactLayoutSpec}, Builtin: true,
		Topics: []string{"prinsip UI/UX dan design tokens", "halaman, section, navigasi, pola layout", "state UI", "aksesibilitas dan aturan platform"}},
	{ID: AreaUseCases, Title: "Alur Bisnis & Use Case", Artifacts: []string{ArtifactUseCases}, Builtin: true, Skippable: true,
		Topics: []string{"aktor dan tujuan", "preconditions dan postconditions", "alur utama, alternatif, kegagalan", "aturan bisnis dan otorisasi", "efek data dan integrasi"}},
	{ID: AreaData, Title: "Arsitektur & Siklus Hidup Data", Artifacts: []string{"data-lifecycle"}, Skippable: true,
		Topics: []string{"kepemilikan data dan sumber kebenaran", "constraint, integritas, transaksi, konsistensi", "strategi penyimpanan dan akses",
			"schema migration dan evolusi data", "retention, archival, penghapusan", "backup, restore, pemulihan", "data sensitif dan perlindungannya"}},
	{ID: AreaSecurity, Title: "Keamanan & Threat Model", Artifacts: []string{"threat-model"}, Skippable: true,
		Topics: []string{"trust boundary dan aset", "threat, abuse case, attack surface", "authentication, authorization, permission matrix",
			"validasi input dan perlindungan data", "secret dan konfigurasi sensitif", "audit trail", "mitigasi dan prioritasnya"}},
	{ID: AreaIntegration, Title: "Integrasi & Dependensi Eksternal", Artifacts: []string{"integrations"}, Skippable: true,
		Topics: []string{"tujuan, pemilik, kontrak", "format request/response dan skema event", "auth ke layanan", "timeout, retry, backoff, rate limit",
			"idempotency dan duplikasi", "fallback dan kegagalan dependency", "konsistensi data dan observabilitas integrasi"}},
	{ID: AreaOperations, Title: "Observability & Reliability", Artifacts: []string{"operations"}, Skippable: true,
		Topics: []string{"structured logging, metrics, tracing", "health check dan readiness", "alerting dan indikator operasional", "SLO/SLI bila relevan",
			"timeout, cancellation, graceful shutdown", "recovery dan retry", "target availability/recovery dari kebutuhan bisnis"}},
	{ID: AreaMigration, Title: "Migrasi & Kompatibilitas", Artifacts: []string{"migration"}, Skippable: true,
		Topics: []string{"kondisi saat ini dan target", "dampak ke API, database, konfigurasi, konsumen", "backward compatibility dan versioning",
			"migrasi data dan schema", "rollout, rollback, pemulihan", "risiko dan dependensi migrasi"}},
	{ID: AreaAcceptance, Title: "Acceptance Criteria & Definition of Done", Artifacts: []string{ArtifactAcceptance}, Builtin: true,
		Topics: []string{"perilaku yang diharapkan", "negative dan edge case", "keamanan dan otorisasi", "performa, kompatibilitas, aksesibilitas bila relevan",
			"bukti verifikasi yang diharapkan", "Definition of Done"}},
}

// AreaByID returns the area with this id, or nil.
func AreaByID(id string) *Area {
	for i := range Areas {
		if Areas[i].ID == id {
			return &Areas[i]
		}
	}
	return nil
}

// AreaSkip leaves an area out of the blueprint on purpose.
type AreaSkip struct {
	Area   string `json:"area"`
	Reason string `json:"reason"`
}

// Skip returns why the area is left out, "" when it is not.
func (p *Project) Skip(area string) string {
	for _, s := range p.Skips {
		if s.Area == area {
			return s.Reason
		}
	}
	return ""
}

// AreaApplies says whether the project needs the area; when it does not,
// why.
func (p *Project) AreaApplies(id string) (bool, string) {
	switch id {
	case AreaUI:
		if !p.HasUI() {
			return false, "produk tanpa layar"
		}
	case AreaUseCases:
		if len(p.Features) == 0 {
			return false, "belum ada fitur"
		}
	case AreaData:
		if (p.Blueprint == nil || len(p.Blueprint.Database.Entities) == 0) && (p.Discovery == nil || len(p.Discovery.Data) == 0) {
			return false, "tidak ada data yang disimpan"
		}
	case AreaIntegration:
		if (p.Discovery == nil || len(p.Discovery.Integrations) == 0) && !p.Survey.HasWebhook() {
			return false, "tidak ada integrasi eksternal"
		}
	case AreaMigration:
		if p.Structure == nil || p.Structure.Mode != StructureExisting {
			return false, "project baru, tidak ada sistem lama"
		}
	case AreaAcceptance:
		if len(p.Requirements) == 0 {
			return false, "belum ada requirement"
		}
	}
	return true, ""
}

// Area statuses.
const (
	AreaCovered       = "covered"
	AreaInvalid       = "invalid"
	AreaMissing       = "missing"
	AreaSkipped       = "skipped"
	AreaNotApplicable = "not_applicable"
)

// AreaStatus is how far the blueprint covers one area.
type AreaStatus struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	Artifacts []string `json:"artifacts"`
	// Reason is why the area is skipped or does not apply.
	Reason string `json:"reason,omitempty"`
	// Errors counts what must be fixed before the area is covered.
	Errors    int  `json:"errors,omitempty"`
	Skippable bool `json:"skippable,omitempty"`
}

// Coverage reports every area against the blueprint docs (built-in and
// written artifacts).
func Coverage(p *Project, docs []Artifact) []AreaStatus {
	return coverageOf(p, docs, nil)
}

// BlueprintChecks is Coverage and ArtifactStates from one AreaIssues pass.
func BlueprintChecks(p *Project, docs []Artifact) ([]AreaStatus, map[string]ArtifactState) {
	issues := make(map[string][]Issue, len(Areas))
	errs := make(map[string]int, len(Areas))
	for _, a := range Areas {
		iss := p.AreaIssues(a.ID)
		issues[a.ID] = iss
		errs[a.ID] = len(Errors(iss))
	}
	return coverageOf(p, docs, errs), artifactStatesOf(p, docs, issues)
}

func coverageOf(p *Project, docs []Artifact, errByArea map[string]int) []AreaStatus {
	out := make([]AreaStatus, 0, len(Areas))
	for _, a := range Areas {
		st := AreaStatus{ID: a.ID, Title: a.Title, Artifacts: []string{}, Skippable: a.Skippable}
		for _, d := range docs {
			if d.Area == a.ID || slices.Contains(a.Artifacts, d.ID) {
				st.Artifacts = append(st.Artifacts, d.ID)
			}
		}
		if errByArea != nil {
			st.Errors = errByArea[a.ID]
		} else {
			st.Errors = len(Errors(p.AreaIssues(a.ID)))
		}
		applies, why := p.AreaApplies(a.ID)
		switch reason := p.Skip(a.ID); {
		case reason != "" && a.Skippable:
			st.Status, st.Reason = AreaSkipped, reason
		case !applies && len(st.Artifacts) == 0:
			st.Status, st.Reason = AreaNotApplicable, why
		case len(st.Artifacts) == 0 || a.ID == AreaUI && len(st.Artifacts) < 2:
			st.Status = AreaMissing
		case st.Errors > 0:
			st.Status = AreaInvalid
		default:
			st.Status = AreaCovered
		}
		out = append(out, st)
	}
	return out
}

// AreaIssues validates the structured data of a built-in area.
func (p *Project) AreaIssues(area string) []Issue {
	switch area {
	case AreaUI:
		if p.UI.Written() {
			return p.DesignIssues(nil)
		}
	case AreaUseCases:
		return p.UseCaseIssues(p.Flows)
	case AreaAcceptance:
		return AcceptanceIssues(p.Requirements, p.Done)
	}
	return nil
}

// ---------- Use cases ----------

// UseCaseIssues validates use cases against the project's features.
func (p *Project) UseCaseIssues(flows []Flow) []Issue {
	var out []Issue
	add := func(level, where, format string, a ...any) {
		out = append(out, Issue{Level: level, Where: where, Message: fmt.Sprintf(format, a...)})
	}
	seen := map[string]bool{}
	for _, f := range flows {
		w := f.ID
		switch {
		case strings.TrimSpace(f.ID) == "":
			add(IssueError, "use-cases", "use case %q tanpa id", f.Name)
			continue
		case seen[f.ID]:
			add(IssueError, w, "id dipakai lebih dari satu use case")
		}
		seen[f.ID] = true
		empty := func(s string) bool { return strings.TrimSpace(s) == "" }
		if empty(f.Name) {
			add(IssueError, w, "nama belum diisi")
		}
		if p.Feature(f.FeatureID) == nil {
			add(IssueError, w, "fitur %q tidak ada", f.FeatureID)
		}
		if empty(f.Role) {
			add(IssueError, w, "aktor belum diisi")
		}
		if empty(f.Goal) {
			add(IssueError, w, "tujuan belum diisi")
		}
		if len(f.Steps) == 0 {
			add(IssueError, w, "alur utama belum diisi")
		}
		if len(f.Post) == 0 {
			add(IssueError, w, "postcondition (kondisi berhasil) belum diisi")
		}
		if len(f.Alternatives) == 0 && len(f.Failures) == 0 {
			add(IssueError, w, "skenario alternatif atau kegagalan belum diisi")
		}
		if len(f.Steps) > MaxDesignListSize {
			add(IssueError, w, "alur utama terlalu panjang (maks %d langkah)", MaxDesignListSize)
		}
	}
	return out
}

// ---------- Acceptance criteria ----------

// Criterion kinds.
var CriterionKinds = []string{"behavior", "negative", "edge", "security", "performance", "compatibility", "accessibility"}

// AcceptanceIssues validates the acceptance criteria and the Definition of
// Done.
func AcceptanceIssues(reqs []Requirement, done []string) []Issue {
	var out []Issue
	add := func(level, where, format string, a ...any) {
		out = append(out, Issue{Level: level, Where: where, Message: fmt.Sprintf(format, a...)})
	}
	ids := map[string]bool{}
	for _, r := range reqs {
		if len(r.Acceptance) == 0 {
			if strings.TrimSpace(r.Untestable) == "" {
				add(IssueError, r.ID, "belum punya acceptance criteria (atau alasan tidak dapat diuji)")
			}
			continue
		}
		edge := false
		for _, c := range r.Acceptance {
			switch {
			case strings.TrimSpace(c.Text) == "":
				add(IssueError, r.ID, "kriteria %s kosong", c.ID)
			case !slices.Contains(CriterionKinds, c.Kind):
				add(IssueError, r.ID, "jenis kriteria %q tidak dikenal (%s)", c.Kind, strings.Join(CriterionKinds, ", "))
			}
			if c.ID == "" || ids[c.ID] {
				add(IssueError, r.ID, "id kriteria %q kosong atau ganda", c.ID)
			}
			ids[c.ID] = true
			edge = edge || c.Kind == "negative" || c.Kind == "edge"
		}
		if r.Priority == "MUST" && !edge {
			add(IssueWarning, r.ID, "belum ada negative atau edge case")
		}
	}
	if len(reqs) > 0 && len(done) == 0 {
		add(IssueError, ArtifactAcceptance, "Definition of Done belum diisi")
	}
	return out
}

// ---------- Requirement → feature traceability ----------

// RequirementCoverageIssues validates that every requirement is covered by
// at least one feature and that no feature references a requirement that
// does not exist. An orphaned requirement can never be built or accepted,
// so both are errors that block finalize.
func RequirementCoverageIssues(reqs []Requirement, feats []Feature) []Issue {
	var out []Issue
	covered := map[string]int{}
	for _, f := range feats {
		for _, r := range f.RequirementIDs {
			covered[r]++
		}
	}
	for _, r := range reqs {
		if covered[r.ID] == 0 {
			out = append(out, Issue{Level: IssueError, Where: r.ID, Message: "tidak terhubung ke feature mana pun"})
		}
	}
	ids := make(map[string]bool, len(reqs))
	for _, r := range reqs {
		ids[r.ID] = true
	}
	for _, f := range feats {
		seen := map[string]bool{}
		for _, r := range f.RequirementIDs {
			if !ids[r] && !seen[r] {
				out = append(out, Issue{Level: IssueError, Where: f.ID, Message: fmt.Sprintf("merujuk requirement %q yang tidak ada", r)})
				seen[r] = true
			}
		}
	}
	return out
}

// conceptTerm maps words that name the same system need onto one term, so a
// requirement is matched to a feature by meaning, not by identical words.
// "latency", "performa" and "cache" name the performance/caching need;
// "pasar", "market" and "quote" name market data.
var conceptTerm = map[string]string{
	"performance": "performa", "performansi": "performa", "latency": "performa", "latensi": "performa",
	"kecepatan": "performa", "throughput": "performa", "cache": "performa", "caching": "performa",
	"responsif": "performa", "cepat": "performa",
	"market": "pasar", "harga": "pasar", "price": "pasar", "quote": "pasar", "kutipan": "pasar",
	"saham": "pasar", "stock": "pasar", "bursa": "pasar", "trading": "pasar", "orderbook": "pasar",
}

// stopTerms are words that name no system need on their own; matching on
// them would tie every feature to every requirement.
var stopTerms = map[string]bool{
	"yang": true, "dan": true, "untuk": true, "dari": true, "dengan": true, "dalam": true, "pada": true,
	"atau": true, "tidak": true, "ini": true, "itu": true, "adalah": true, "akan": true, "bisa": true,
	"harus": true, "dapat": true, "mampu": true, "secara": true, "sebagai": true, "agar": true,
	"the": true, "a": true, "an": true, "and": true, "or": true, "of": true, "to": true, "in": true,
	"on": true, "for": true, "with": true, "is": true, "are": true, "be": true, "must": true,
	"should": true, "can": true, "will": true, "that": true, "this": true,
	"pengguna": true, "user": true, "sistem": true, "system": true, "aplikasi": true, "application": true,
	"data": true, "fitur": true, "feature": true, "tanpa": true, "ada": true,
}

// MapRequirementToFeature links a requirement to the feature that covers it
// by meaning: the feature whose name, description and sub-features share
// the most normalized meaning with the requirement text. It returns "" when
// no feature shares any meaning, so the caller can refuse instead of
// inventing a link.
func MapRequirementToFeature(req Requirement, feats []Feature) string {
	best, bestScore := "", 0.0
	for _, f := range feats {
		if s := requirementFeatureScore(req.Text, f); s > bestScore {
			best, bestScore = f.ID, s
		}
	}
	return best
}

func requirementFeatureScore(text string, f Feature) float64 {
	var b strings.Builder
	b.WriteString(f.Name)
	b.WriteString(" ")
	b.WriteString(f.Description)
	for _, s := range f.SubFeatures {
		b.WriteString(" ")
		b.WriteString(s.Name)
	}
	req := meaningfulTerms(text)
	feat := meaningfulTerms(b.String())
	if len(req) == 0 || len(feat) == 0 {
		return 0
	}
	featCount := map[string]int{}
	for _, t := range feat {
		featCount[t]++
	}
	var shared float64
	for _, t := range req {
		if n := featCount[t]; n > 0 {
			shared++
			featCount[t] = n - 1
		}
	}
	return shared / (math.Sqrt(float64(len(req))) * math.Sqrt(float64(len(feat))))
}

func meaningfulTerms(s string) []string {
	var out []string
	for _, t := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(t) < 2 || stopTerms[t] {
			continue
		}
		if c, ok := conceptTerm[t]; ok {
			t = c
		}
		out = append(out, t)
	}
	return out
}

// ---------- Decisions ----------

// Decision kinds.
const (
	DecisionAssumption = "assumption"
	DecisionQuestion   = "question"
	DecisionConflict   = "conflict"
	DecisionRisk       = "risk"
)

var DecisionKinds = []string{DecisionAssumption, DecisionQuestion, DecisionConflict, DecisionRisk}

// Decision is an assumption, open question, conflict or risk of the
// blueprint. Resolution is the user's decision (or the accepted
// mitigation); empty means still open. A critical open one blocks
// finalize: the user has to decide it.
type Decision struct {
	ID         string `json:"id"` // DEC-001
	Kind       string `json:"kind"`
	Text       string `json:"text"`
	Area       string `json:"area,omitempty"`
	Critical   bool   `json:"critical,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

func (d Decision) Open() bool { return strings.TrimSpace(d.Resolution) == "" }

// BindingDecisions are the decided register entries. They override every
// other artifact and every command; an agent reading one document still
// follows these.
func (p *Project) BindingDecisions() []Decision {
	var out []Decision
	for _, d := range p.Decisions {
		if !d.Open() {
			out = append(out, d)
		}
	}
	return out
}

// BindingText is the compact digest of decided decisions for prompts and
// exports. Empty when none is decided.
func BindingText(ds []Decision) string {
	var b strings.Builder
	for _, d := range ds {
		if d.Open() {
			continue
		}
		fmt.Fprintf(&b, "- %s: %s\n  Keputusan: %s\n", d.ID, d.Text, d.Resolution)
	}
	return b.String()
}

// DecisionIssues validates the decision register.
func DecisionIssues(ds []Decision) []Issue {
	var out []Issue
	seen := map[string]bool{}
	for _, d := range ds {
		add := func(format string, a ...any) {
			out = append(out, Issue{Level: IssueError, Where: d.ID, Message: fmt.Sprintf(format, a...)})
		}
		if d.ID == "" || seen[d.ID] {
			add("id keputusan kosong atau ganda")
		}
		seen[d.ID] = true
		if strings.TrimSpace(d.Text) == "" {
			add("isi keputusan kosong")
		}
		if !slices.Contains(DecisionKinds, d.Kind) {
			add("jenis %q tidak dikenal (%s)", d.Kind, strings.Join(DecisionKinds, ", "))
		}
		if d.Area != "" && AreaByID(d.Area) == nil {
			add("area %q tidak dikenal", d.Area)
		}
	}
	return out
}

// OpenCritical lists the critical decisions still waiting for the user.
func (p *Project) OpenCritical() []Decision {
	var out []Decision
	for _, d := range p.Decisions {
		if d.Critical && d.Open() {
			out = append(out, d)
		}
	}
	return out
}

// ---------- Artifact review ----------

// Artifact states, as the blueprint UI shows them.
const (
	StateDraft       = "draft"
	StateValid       = "valid"
	StateNeedsReview = "needs_review"
	StateFinalized   = "finalized"
)

// ArtifactState is one artifact's state; Reason says why it is not valid.
type ArtifactState struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// StaleRefs are the refs of a written artifact that changed or vanished
// since it recorded them. Refs it never recorded are not stale.
func StaleRefs(a Artifact, hashes map[string]string) []string {
	var out []string
	for _, r := range a.Refs {
		seen, ok := a.Seen[r]
		if !ok {
			continue
		}
		if h, exists := hashes[r]; !exists || h != seen {
			out = append(out, r)
		}
	}
	return out
}

// ArtifactStates reports the state of every doc.
func ArtifactStates(p *Project, docs []Artifact) map[string]ArtifactState {
	return artifactStatesOf(p, docs, nil)
}

func artifactStatesOf(p *Project, docs []Artifact, issuesByArea map[string][]Issue) map[string]ArtifactState {
	hashes := make(map[string]string, len(docs))
	for _, d := range docs {
		hashes[d.ID] = d.Hash()
	}
	if issuesByArea == nil {
		issuesByArea = make(map[string][]Issue)
		for _, a := range Areas {
			if a.Builtin {
				issuesByArea[a.ID] = p.AreaIssues(a.ID)
			}
		}
	}
	var decisionErrs int
	var openCrit int
	checkedDecisions := false
	out := make(map[string]ArtifactState, len(docs))
	for _, d := range docs {
		st := ArtifactState{Status: StateValid}
		errs := 0
		for _, a := range Areas {
			if a.Builtin && slices.Contains(a.Artifacts, d.ID) {
				issues := Errors(issuesByArea[a.ID])
				if a.ID == AreaUI {
					issues = slices.DeleteFunc(slices.Clone(issues), func(i Issue) bool { return (i.Where == "rules") != (d.ID == ArtifactDesignRules) })
				}
				errs = len(issues)
			}
		}
		if d.ID == ArtifactDecisions {
			if !checkedDecisions {
				decisionErrs = len(DecisionIssues(p.Decisions))
				openCrit = len(p.OpenCritical())
				checkedDecisions = true
			}
			errs = decisionErrs
		}
		switch stale := StaleRefs(d, hashes); {
		case p.Finalized():
			st.Status = StateFinalized
		case len(stale) > 0:
			st.Status, st.Reason = StateNeedsReview, "artefak acuan berubah: "+strings.Join(stale, ", ")
		case errs > 0:
			st.Status, st.Reason = StateDraft, fmt.Sprintf("%d hal wajib diperbaiki", errs)
		case d.ID == ArtifactDecisions && openCrit > 0:
			st.Status, st.Reason = StateDraft, fmt.Sprintf("%d keputusan kritis belum diputuskan", openCrit)
		}
		out[d.ID] = st
	}
	return out
}
