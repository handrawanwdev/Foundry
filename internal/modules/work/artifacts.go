package work

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

// KindArtifacts revises the blueprint artifacts for a change request.
const KindArtifacts = "blueprint"

// MaxArtifacts bounds the artifacts one project keeps besides the
// built-in ones.
const MaxArtifacts = 30

// promptBody is how much of an artifact body a prompt carries.
const promptBody = 4000

// bindingDocs are carried whole: they hold the user's choices (decided
// decisions, approved technology, survey, requirement ids), and a cut
// one lets an assumption elsewhere win over a decision.
var bindingDocs = map[string]bool{
	"requirements": true, "technology": true, core.ArtifactSurvey: true, core.ArtifactDecisions: true,
}

type artifactsOut struct {
	Artifacts []core.Artifact `json:"artifacts"`
	Remove    []string        `json:"remove"`
	scopeOut
}

// artifactRules is shared by the discovery and the revision prompt.
const artifactRules = `Artifact rules:
- id: lowercase-kebab, unique; category: requirements | domain | architecture | ui | data | platform | quality | other.
- body is Markdown: a short opening paragraph, ## headings, short paragraphs, lists, tables for comparisons, code blocks for technical specs (schemas, contracts, protocols), a mermaid code block only when a diagram really helps.
- write only what this project needs; no document just to fill a template, no repeat of another artifact.
- mark assumptions as "Asumsi:" and details an existing codebase must confirm as "Perlu verifikasi:"; never invent file paths, endpoints, tables or runtime facts.
- area: the blueprint area the artifact covers (data, security, integration, operations, migration), empty for others.
- refs lists related artifact ids; a change of a referenced artifact marks this one for review.
`

// ArtifactsPrompt asks the AI to revise the blueprint for the pending
// request. docs is the whole blueprint (built-in artifacts included).
func ArtifactsPrompt(p *core.Project, docs []core.Artifact) string {
	var b strings.Builder
	b.WriteString(`You are the blueprint editor of Foundry. Foundry designs software and writes instructions for external coding agents; it never implements. Revise the blueprint artifacts of this project for the user's request. Do not write application code. Do not modify files.
Write every human-readable text in the same language as the story.

PROJECT: ` + p.Name + "\nSTORY:\n" + p.Story + "\n\n")
	writeBinding(&b, p)
	cut := promptBody
	if len(p.DecisionSync) > 0 {
		cut = 0 // the AI must see the full text to find what still contradicts the decisions
	}
	writeDocs(&b, docs, cut)
	b.WriteString("AREA STATUS:\n")
	for _, a := range core.Coverage(p, docs) {
		fmt.Fprintf(&b, "- %s: %s", a.ID, a.Status)
		if a.Reason != "" {
			fmt.Fprintf(&b, " (%s)", a.Reason)
		}
		if a.Errors > 0 {
			fmt.Fprintf(&b, ", %d errors", a.Errors)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if issues := scopeIssues(p); len(issues) > 0 {
		b.WriteString("OPEN PROBLEMS of use cases and acceptance criteria:\n")
		for _, is := range issues {
			fmt.Fprintf(&b, "- %s %s: %s\n", is.Level, is.Where, is.Message)
		}
		b.WriteString("\n")
	}
	if changes := decisionChanges(p, p.DecisionSync); changes != "" {
		b.WriteString("DECISIONS CHANGED by the user after the artifacts were written (as they stand now):\n" + changes +
			"Bring the blueprint in line in this answer: return every artifact, use case and acceptance entry whose content (assumptions, recommendations, business rules, permissions, flows) still contradicts these decisions, rewritten to follow them; a reopened decision is written as still open, not as decided. Leave unaffected ones out; when nothing contradicts them, an empty JSON object is a valid answer.\n\n")
	}
	r := p.ArtifactRequest
	if r != nil {
		b.WriteString("USER REQUEST:\n" + r.Text + "\n\n")
		if len(r.Focus) > 0 {
			b.WriteString("The user is looking at: " + strings.Join(r.Focus, ", ") + ".\n\n")
		}
		if r.Rejected != "" {
			b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + r.Rejected + "\n\n")
		}
	}
	b.WriteString(artifactRules)
	areaGuide(&b, p)
	b.WriteString(scopeRules)
	b.WriteString(`Revision rules:
- change only what the request needs. Return only the artifacts, use cases, acceptance entries and decisions you add or change, each in full; the others stay as they are. Omit definition_of_done to keep it.
- use-cases, acceptance and decisions are built-in artifacts: change them through use_cases, acceptance, definition_of_done and decisions. requirements, survey-decisions, architecture, technology, design-rules, layout-spec and system-overview cannot be rewritten here. A passage there that contradicts a decided decision does not win: name the contradiction and follow the decision. Do not create a new artifact whose only job is to restate a decided decision (that document would hang on its own and other agents would miss it). For a technology change also add an open decision asking the user to change it in Foundry's technology step.
- a new use case or decision gets an empty id; keep the id of a changed one.
- when you decide, change or reopen a decision, also return in this answer every artifact, use case and acceptance entry that contradicts its new state, rewritten to follow it. The commands are brought in line after this answer.
- do not drop content that still applies.

Reply with ONLY one JSON object in a ` + "```json" + ` block:
{"artifacts": [{"id": "threat-model", "title": "", "category": "quality", "area": "security", "summary": "", "body": "", "refs": []}], "remove": ["artifact-id"],
 "use_cases": [` + useCaseShape + `], "remove_use_cases": ["FLOW-009"],
 "acceptance": [{"requirement": "REQ-001", "criteria": [` + criterionShape + `], "untestable": ""}],
 "definition_of_done": [""], "decisions": [` + decisionShape + `],
 "skip": [{"area": "migration", "reason": ""}], "unskip": ["area"]}`)
	return b.String()
}

const (
	useCaseShape   = `{"id": "FLOW-001", "name": "", "feature_id": "FEAT-001", "role": "", "goal": "", "preconditions": [""], "steps": [""], "postconditions": [""], "alternatives": [""], "failures": [""], "business_rules": [""], "permissions": [""], "data_effects": [""], "integrations": [""]}`
	criterionShape = `{"id": "REQ-001-AC1", "kind": "behavior|negative|edge|security|performance|compatibility|accessibility", "text": "", "evidence": ""}`
	decisionShape  = `{"id": "DEC-001", "kind": "assumption|question|conflict|risk", "text": "", "area": "", "critical": false, "resolution": ""}`
)

// scopeIssues are the use case and acceptance problems.
func scopeIssues(p *core.Project) []core.Issue {
	return append(p.AreaIssues(core.AreaUseCases), p.AreaIssues(core.AreaAcceptance)...)
}

// writeBinding puts the decided decisions at the top of every agent prompt
// so no document is read as if it stood alone.
func writeBinding(b *strings.Builder, p *core.Project) {
	text := core.BindingText(p.Decisions)
	if text == "" {
		return
	}
	b.WriteString("BINDING DECISIONS — these override every other artifact, use case, acceptance criterion, design rule, layout and command. Never present an alternative to them as still open, and never follow a contradicting passage in another document.\n")
	b.WriteString(text)
	b.WriteString("\n")
}

// writeDocs lists the blueprint for a prompt; long bodies are cut unless
// cut is 0. Binding documents (decisions, requirements, technology, survey)
// are never cut.
func writeDocs(b *strings.Builder, docs []core.Artifact, cut int) {
	b.WriteString("BLUEPRINT ARTIFACTS:\n")
	for _, d := range docs {
		body := d.Body
		if cut > 0 && len(body) > cut && !bindingDocs[d.ID] {
			body = strings.ToValidUTF8(body[:cut], "") + "\n…(dipotong)"
		}
		fmt.Fprintf(b, "=== %s (%s, %s%s) %s\n%s\n", d.ID, d.Title, d.Category, map[bool]string{true: ", builtin"}[d.Builtin], d.Summary, body)
	}
	b.WriteString("\n")
	b.WriteString(precedenceRules)
}

// precedenceRules settles which blueprint source wins when they disagree.
const precedenceRules = `Precedence when blueprint sources disagree (highest first):
1. decided decisions: the "Keputusan" of each DEC under "Diputuskan" in the decisions artifact. Binding; use the choice as written, never present it as open or pick an alternative to it.
2. the user's mandatory constraints and the approved technology artifact.
3. every other artifact. Its recommendations, "Asumsi" and temporary choices apply only where 1 and 2 are silent; an artifact that contradicts 1 or 2 does not override them, so name the contradiction instead of following it.

`

// ApplyArtifacts lands a revision: artifacts are added or replaced by id
// and removed ones dropped. Built-in artifacts cannot be touched.
func ApplyArtifacts(p *core.Project, raw string) error {
	var out artifactsOut
	if err := ExtractJSON(raw, &out); err != nil {
		return err
	}
	sync := p.ArtifactRequest != nil && p.ArtifactRequest.Sync
	if len(out.Artifacts) == 0 && len(out.Remove) == 0 && out.scopeOut.empty() && !sync {
		return errors.New("AI tidak menemukan perubahan untuk permintaan ini; tulis lebih spesifik")
	}
	before := p.Decisions
	list := slices.Clone(p.Artifacts)
	for _, id := range out.Remove {
		i := slices.IndexFunc(list, func(a core.Artifact) bool { return a.ID == id })
		switch {
		case core.BuiltinArtifacts[id]:
			return fmt.Errorf("artefak bawaan %q tidak bisa dihapus", id)
		case i < 0:
			return fmt.Errorf("artefak %q tidak ada", id)
		}
		list = slices.Delete(list, i, i+1)
	}
	now := time.Now().UTC()
	for _, a := range out.Artifacts {
		a, err := normArtifact(a)
		if err != nil {
			return err
		}
		if i := slices.IndexFunc(list, func(x core.Artifact) bool { return x.ID == a.ID }); i >= 0 {
			if list[i].Hash() == a.Hash() && slices.Equal(list[i].Refs, a.Refs) && list[i].Category == a.Category && list[i].Area == a.Area {
				continue
			}
			a.Version = list[i].Version + 1
			a.UpdatedAt = now
			list[i] = a
			continue
		}
		a.Version, a.UpdatedAt = 1, now
		list = append(list, a)
	}
	if len(list) > MaxArtifacts {
		return fmt.Errorf("terlalu banyak artefak (%d, maks %d)", len(list), MaxArtifacts)
	}
	if _, err := applyScope(p, out.scopeOut); err != nil {
		return err
	}
	p.Artifacts = dropDanglingRefs(list)
	p.ArtifactRequest = nil
	// The answer brought the artifacts in line with every changed decision;
	// the commands follow in the workflow job.
	requestCommandSync(p, slices.Concat(p.DecisionSync, changedDecisions(before, p.Decisions)))
	p.DecisionSync = nil
	return nil
}

// SetArtifacts replaces the written artifacts (a new discovery). Invalid
// entries are skipped; duplicate ids get a suffix.
func SetArtifacts(p *core.Project, in []core.Artifact) {
	now := time.Now().UTC()
	var list []core.Artifact
	seen := map[string]bool{}
	for _, a := range in {
		a, err := normArtifact(a)
		if err != nil || len(list) == MaxArtifacts {
			continue
		}
		base := a.ID
		for n := 2; seen[a.ID]; n++ {
			a.ID = fmt.Sprintf("%s-%d", base, n)
		}
		seen[a.ID] = true
		a.Version, a.UpdatedAt = 1, now
		list = append(list, a)
	}
	p.Artifacts = dropDanglingRefs(list)
}

// EditArtifact saves a user's edit of a written artifact.
func EditArtifact(p *core.Project, id string, in core.Artifact) error {
	i := slices.IndexFunc(p.Artifacts, func(a core.Artifact) bool { return a.ID == id })
	if i < 0 {
		if core.BuiltinArtifacts[id] {
			return fmt.Errorf("artefak bawaan %q berubah lewat datanya sendiri (requirement, arsitektur, desain), bukan diedit langsung", id)
		}
		return fmt.Errorf("artefak %q tidak ada", id)
	}
	in.ID = id
	a, err := normArtifact(in)
	if err != nil {
		return err
	}
	old := p.Artifacts[i]
	if old.Hash() == a.Hash() && old.Category == a.Category && slices.Equal(old.Refs, a.Refs) && old.Area == a.Area {
		return nil
	}
	a.Version, a.UpdatedAt = old.Version+1, time.Now().UTC()
	p.Artifacts[i] = a
	p.Artifacts = dropDanglingRefs(p.Artifacts)
	return nil
}

// RemoveArtifact drops a written artifact. Commands that referenced it
// are reported for review by the workflow check.
func RemoveArtifact(p *core.Project, id string) error {
	i := slices.IndexFunc(p.Artifacts, func(a core.Artifact) bool { return a.ID == id })
	if i < 0 {
		return fmt.Errorf("artefak %q tidak ada atau bawaan", id)
	}
	p.Artifacts = dropDanglingRefs(slices.Delete(p.Artifacts, i, i+1))
	return nil
}

func normArtifact(a core.Artifact) (core.Artifact, error) {
	a.ID = slugID(a.ID)
	if a.ID == "" {
		a.ID = slugID(a.Title)
	}
	a.Title = strings.TrimSpace(a.Title)
	a.Summary = strings.TrimSpace(a.Summary)
	a.Body = strings.TrimSpace(a.Body)
	switch {
	case a.ID == "":
		return a, errors.New("artefak tanpa id dan judul")
	case core.BuiltinArtifacts[a.ID]:
		return a, fmt.Errorf("artefak bawaan %q tidak bisa ditulis ulang; pakai id lain", a.ID)
	case a.Title == "":
		return a, fmt.Errorf("artefak %q tanpa judul", a.ID)
	case a.Body == "":
		return a, fmt.Errorf("artefak %q tanpa isi", a.ID)
	case len(a.Body) > 60_000:
		return a, fmt.Errorf("artefak %q terlalu panjang", a.ID)
	}
	if !slices.Contains(core.ArtifactCategories, a.Category) {
		a.Category = core.ArtifactOther
	}
	a.Area = strings.TrimSpace(a.Area)
	if area := core.AreaByID(a.Area); area == nil || area.Builtin {
		a.Area = ""
	}
	for _, area := range core.Areas {
		if a.Area == "" && !area.Builtin && area.Artifacts[0] == a.ID {
			a.Area = area.ID
		}
	}
	a.Seen = nil // recorded again by StampRefs
	var refs []string
	for _, r := range a.Refs {
		if r = slugID(r); r != "" && r != a.ID && !slices.Contains(refs, r) {
			refs = append(refs, r)
		}
	}
	a.Refs, a.Builtin = refs, false
	return a, nil
}

// dropDanglingRefs keeps refs to built-in or listed artifacts only.
func dropDanglingRefs(list []core.Artifact) []core.Artifact {
	ids := map[string]bool{}
	for _, a := range list {
		ids[a.ID] = true
	}
	for i := range list {
		list[i].Refs = slices.DeleteFunc(list[i].Refs, func(r string) bool { return !ids[r] && !core.BuiltinArtifacts[r] })
		if len(list[i].Refs) == 0 {
			list[i].Refs = nil
		}
	}
	return list
}
