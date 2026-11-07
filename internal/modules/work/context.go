package work

import (
	"fmt"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

const adaptationRules = `EXISTING PROJECT ADAPTATION:
- treat the repository as the system that already exists. Do not design it as a greenfield project.
- decision order: (1) existing implementation and conventions that still hold, (2) existing architecture and domain boundaries, (3) the new need, (4) architecture repair only when the current shape blocks the need, (5) a new technology only with a measured reason.
- do not recreate a feature, module, service or component the context already lists.
- do not replace a technology without a reason the story or the user gave.
- do not invent a generic folder tree; new files follow the observed layout and naming.
- do not refactor unless the change requires it. Prefer reuse and the smallest correct edit.
- keep backward compatibility unless the change plan says otherwise.
- separate fact (context evidence), documentation (confidence documented), assumption, and recommendation.
- every important claim about the current system names its file or evidence when the context has one.
- if the current architecture is a problem, say the impact and give an alternative; do not silently replace it.
`

const changeBlueprintRules = `FEATURE CHANGE BLUEPRINT:
- this job is a change to the existing system, not a full rewrite. Set change.scope to "change".
- keep modules, data, APIs, auth and UI the change does not need to alter.
- the architecture artifact still describes the system as it is, plus this change; change is the delta and must stay consistent with that architecture.
- classify every item as add, modify, reuse, refactor or remove. Do not remove or replace a component without a dependency analysis in impact.
- files.add / files.modify / files.remove follow the existing layout. Do not emit a template tree.
- impact covers modules that change, dependents, database/migration, API compatibility, permissions, UI, regression, performance, security and rollback when they apply.
- mark impact.source observed only with repository evidence; otherwise estimated. Never claim a dependency the context did not list as observed.
- name what is reused. A feature the context already has is reuse or modify, never a second copy.
- do not silently replace a module, entity, auth method or architecture pattern the main blueprint already decided. Record a conflict and keep the old decision.
`

const fullExistingRules = `FULL BLUEPRINT OF AN EXISTING PROJECT:
- describe the system as the repository is, then the requested work.
- never treat the folder as empty. Components, context evidence and user corrections bind the design.
- set change.scope to "full". Still record reuse of what is already there; do not duplicate it.
`

const existingCommandRules = `EXISTING PROJECT COMMANDS:
- inspect the named files first; do not invent paths, tables or endpoints the context did not list.
- reuse existing modules, services and components; do not recreate them.
- split work when dependencies differ: implement (new behaviour), migrate (schema/data), integrate (wire into what exists), test (verification).
- name the files to add, modify or reuse from the change plan when present.
- keep backward compatibility unless the change plan says otherwise.
- do not repeat work the repository already satisfies.
- follow the observed folder layout; never invent a different top-level tree.
`

func writeWorkIntent(b *strings.Builder, p *core.Project) {
	intent := p.IntentOrDefault()
	fmt.Fprintf(b, "WORK INTENT: %s (%s).\n", intent, core.ChoiceLabel(core.WorkIntents, string(intent)))
	switch intent {
	case core.IntentNewFeature:
		b.WriteString("Plan a new feature on the existing system.\n")
	case core.IntentEnhancement:
		b.WriteString("Plan an enhancement of behaviour that already exists.\n")
	case core.IntentRefactoring:
		b.WriteString("Plan a structural cleanup; do not change behaviour unless the story asks.\n")
	case core.IntentBugFix:
		b.WriteString("Plan a fix for a defect; keep the smallest change that corrects it.\n")
	case core.IntentArchitecture:
		b.WriteString("Plan an architecture improvement; record impact, trade-off and a smaller alternative.\n")
	case core.IntentExisting:
		b.WriteString("Understand the existing project and plan the requested work against it.\n")
	default:
		b.WriteString("Greenfield: there is no existing codebase to follow.\n")
	}
	b.WriteByte('\n')
}

func writeExistingContext(b *strings.Builder, p *core.Project) {
	if !existingProject(p) {
		return
	}
	b.WriteString(adaptationRules)
	b.WriteByte('\n')
	if p.IntentOrDefault().ChangeScope() {
		b.WriteString(changeBlueprintRules)
	} else {
		b.WriteString(fullExistingRules)
	}
	b.WriteByte('\n')
	c := p.Structure.Context
	if c == nil || c.Empty() {
		if c != nil && strings.TrimSpace(c.Corrections) != "" {
			fmt.Fprintf(b, "USER CORRECTIONS ON THE SCAN:\n%s\n\n", indent(c.Corrections))
		}
		return
	}
	if c.Stale {
		b.WriteString("CONTEXT STALE: the repository changed after this scan")
		if len(c.StaleAreas) > 0 {
			fmt.Fprintf(b, " (areas: %s)", strings.Join(c.StaleAreas, ", "))
		}
		b.WriteString(". Do not treat these findings as current fact until the context is refreshed.\n\n")
	}
	b.WriteString("EXISTING PROJECT CONTEXT (evidence from the repository. confidence observed = read from code or a manifest; documented = read from docs and not verified in code; estimated = AI inference, never a scanner fact. Do not treat a path as a responsibility unless evidence says so):\n")
	writeFindings(b, "Layout", c.Layout)
	writeFindings(b, "Languages", c.Languages)
	writeFindings(b, "Frameworks", c.Frameworks)
	if len(c.Modules) > 0 {
		b.WriteString("- Modules:\n")
		for _, m := range c.Modules {
			fmt.Fprintf(b, "  - `%s` evidence: %s", m.Path, m.Evidence)
			if len(m.Depends) > 0 {
				fmt.Fprintf(b, " depends_on: %s", strings.Join(m.Depends, ", "))
			}
			b.WriteByte('\n')
		}
	}
	if len(c.Deps) > 0 {
		b.WriteString("- Observed dependencies (not estimated):\n")
		n := 0
		for _, d := range c.Deps {
			fmt.Fprintf(b, "  - %s → %s [%s] at `%s` evidence: %s\n", d.From, d.To, d.Kind, d.Path, d.Evidence)
			n++
			if n >= 40 {
				break
			}
		}
	}
	if len(c.Changes) > 0 {
		b.WriteString("- Repository changes since previous scan:\n")
		for _, ch := range c.Changes {
			fmt.Fprintf(b, "  - %s %s `%s`\n", ch.Action, ch.Area, ch.Path)
		}
	}
	if len(c.Data) > 0 {
		b.WriteString("- Data model:\n")
		for _, e := range c.Data {
			fields := ""
			if len(e.Fields) > 0 {
				fields = " fields: " + strings.Join(e.Fields, ", ")
			}
			fmt.Fprintf(b, "  - %s%s at `%s` (%s)\n", e.Name, fields, e.Path, e.Evidence)
		}
	}
	if len(c.APIs) > 0 {
		b.WriteString("- APIs and routes:\n")
		for _, a := range c.APIs {
			detail := a.Detail
			if detail == "" {
				detail = a.Kind
			}
			fmt.Fprintf(b, "  - %s at `%s` (%s)\n", detail, a.Path, a.Evidence)
		}
	}
	writeFindings(b, "Auth", c.Auth)
	writeFindings(b, "Tests", c.Tests)
	writeFindings(b, "Docs", c.Docs)
	writeFindings(b, "Conventions", c.Conventions)
	writeFindings(b, "Documented features", c.Features)
	if strings.TrimSpace(c.Corrections) != "" {
		fmt.Fprintf(b, "USER CORRECTIONS (bind; they override a scan guess):\n%s\n", indent(c.Corrections))
	}
	b.WriteByte('\n')
}

func writeFindings(b *strings.Builder, title string, fs []core.ContextFinding) {
	if len(fs) == 0 {
		return
	}
	fmt.Fprintf(b, "- %s:\n", title)
	for _, f := range fs {
		path := ""
		if f.Path != "" {
			path = " `" + f.Path + "`"
		}
		conf := f.Confidence
		if conf == "" {
			conf = core.ConfidenceObserved
		}
		fmt.Fprintf(b, "  - %s%s [%s] evidence: %s\n", f.Text, path, conf, f.Evidence)
	}
}

func writeChangePlan(b *strings.Builder, p *core.Project) {
	c := p.Change
	if c == nil || c.Empty() {
		return
	}
	b.WriteString("CHANGE PLAN (delta against the current system; commands and new files follow this):\n")
	if c.Scope != "" {
		fmt.Fprintf(b, "Scope: %s\n", c.Scope)
	}
	if c.Summary != "" {
		fmt.Fprintf(b, "Summary: %s\n", c.Summary)
	}
	if len(c.Existing) > 0 {
		b.WriteString("Reuse:\n")
		for _, s := range c.Existing {
			fmt.Fprintf(b, "- %s\n", s)
		}
	}
	if len(c.Items) > 0 {
		b.WriteString("Items:\n")
		for _, it := range c.Items {
			fmt.Fprintf(b, "- %s %s", it.Action, it.Target)
			if it.Module != "" {
				fmt.Fprintf(b, " (module %s)", it.Module)
			}
			if it.Reason != "" {
				fmt.Fprintf(b, ": %s", it.Reason)
			}
			if len(it.Files) > 0 {
				fmt.Fprintf(b, " files: %s", strings.Join(it.Files, ", "))
			}
			b.WriteByte('\n')
		}
	}
	if len(c.Impact) > 0 {
		b.WriteString("Impact:\n")
		for _, im := range c.Impact {
			risk := im.Risk
			if risk == "" {
				risk = "unspecified"
			}
			src := im.Source
			if src == "" {
				src = core.ConfidenceEstimated
			}
			fmt.Fprintf(b, "- [%s/%s/%s] %s", im.Area, risk, src, im.Message)
			if im.Evidence != "" {
				fmt.Fprintf(b, " evidence: %s", im.Evidence)
			}
			b.WriteByte('\n')
		}
	}
	if len(c.Kept) > 0 {
		b.WriteString("Kept from main blueprint (do not overwrite):\n")
		for _, s := range c.Kept {
			fmt.Fprintf(b, "- %s\n", s)
		}
	}
	if len(c.Conflicts) > 0 {
		b.WriteString("Conflicts with main blueprint:\n")
		for _, cf := range c.Conflicts {
			fmt.Fprintf(b, "- [%s] %s", cf.Kind, cf.Message)
			if cf.Keep != "" {
				fmt.Fprintf(b, " keep: %s", cf.Keep)
			}
			b.WriteByte('\n')
		}
	}
	writeChangeFiles(b, "Files to add", c.Files.Add)
	writeChangeFiles(b, "Files to modify", c.Files.Modify)
	writeChangeFiles(b, "Files to remove", c.Files.Remove)
	b.WriteByte('\n')
}

func writeChangeFiles(b *strings.Builder, title string, files []core.ChangeFile) {
	if len(files) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", title)
	for _, f := range files {
		fmt.Fprintf(b, "- `%s`", f.Path)
		if f.Responsibility != "" {
			fmt.Fprintf(b, " — %s", f.Responsibility)
		}
		if f.Reason != "" {
			fmt.Fprintf(b, " (%s)", f.Reason)
		}
		b.WriteByte('\n')
	}
}

func writeCodebaseBrief(b *strings.Builder, p *core.Project) {
	if existingProject(p) {
		b.WriteString("EXISTING CODEBASE: yes. Commands change it; the agent inspects named files first.\n")
		writeWorkIntent(b, p)
		writeExistingContext(b, p)
		writeChangePlan(b, p)
		b.WriteString(existingCommandRules)
		b.WriteByte('\n')
		return
	}
	b.WriteString("EXISTING CODEBASE: no (new project).\n")
	writeWorkIntent(b, p)
}

const changeJSONShape = `"change": {
    "scope": "full|change",
    "summary": "",
    "existing": ["module or path reused"],
    "items": [{"action": "add|modify|reuse|refactor|remove", "target": "", "module": "", "reason": "", "files": [""]}],
    "impact": [{"area": "module|database|api|permission|ui|performance|security|rollback|regression", "message": "", "risk": "low|medium|high", "kind": "direct|indirect|breaking|migration|api|security|regression|rollback", "source": "observed|estimated", "evidence": ""}],
    "files": {
      "add": [{"path": "", "responsibility": ""}],
      "modify": [{"path": "", "responsibility": ""}],
      "remove": [{"path": "", "reason": ""}]
    }
  }`
