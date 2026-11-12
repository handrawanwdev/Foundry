package techplan

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// The contract of a technology plan: what every decision and every
// alternative must carry, and which areas the platform makes mandatory. The
// validator checks exactly this, and the prompts render it from the same
// values, so the AI is told every requirement the validator enforces.

// Issue codes of the technology plan. They group the problems for a targeted
// repair and for the user's diagnosis.
const (
	CodeEmptyPlan        = "PLAN_EMPTY"
	CodePlanMeta         = "PLAN_META_MISSING"
	CodeContext          = "CONTEXT_MISSING"
	CodePlatformMissing  = "PLATFORM_TECH_MISSING"
	CodePlatformDeclined = "PLATFORM_DECLINED"
	CodeStructureValue   = "STRUCTURE_VALUE_INVALID"
	CodeDuplicateArea    = "DUPLICATE_AREA"
	CodeTechnologyEmpty  = "TECHNOLOGY_EMPTY"
	CodeInvalidGroup     = "INVALID_GROUP"
	CodeRequiredField    = "REQUIRED_FIELD_MISSING"
	CodeUnknownDepend    = "UNKNOWN_DEPENDENCY"
	CodeAltIncomplete    = "ALTERNATIVE_INCOMPLETE"
	CodeEvaluation       = "EVALUATION_INCOMPLETE"
	CodeConsistency      = "CONSISTENCY_INCOMPLETE"
	CodeCompatibility    = "COMPATIBILITY_MISSING"
	CodeBinding          = "BINDING_CHOICE_CHANGED"
	CodeConflict         = "CONFLICT"
	CodeWarning          = "WARNING"
)

// codeLabel is the user-facing name of a code, for the diagnosis.
var codeLabel = map[string]string{
	CodeEmptyPlan:        "rencana kosong",
	CodePlanMeta:         "approach/summary kosong",
	CodeContext:          "context teknis kosong",
	CodePlatformMissing:  "teknologi platform belum ditentukan",
	CodePlatformDeclined: "platform dinilai tidak perlu",
	CodeStructureValue:   "nilai architecture/repository tidak valid",
	CodeDuplicateArea:    "area ganda",
	CodeTechnologyEmpty:  "technology kosong",
	CodeInvalidGroup:     "group tidak valid",
	CodeRequiredField:    "field wajib keputusan kosong",
	CodeUnknownDepend:    "depends_on ke area yang tidak ada",
	CodeAltIncomplete:    "alternatif belum lengkap",
	CodeEvaluation:       "evaluasi belum lengkap",
	CodeConsistency:      "pemeriksaan konsistensi belum lengkap",
	CodeCompatibility:    "pemeriksaan kompatibilitas belum ada",
	CodeBinding:          "pilihan mengikat diganti",
	CodeConflict:         "konflik",
	CodeWarning:          "peringatan",
}

// CodeLabel names a code for the user; the code itself when unknown.
func CodeLabel(code string) string {
	if l := codeLabel[code]; l != "" {
		return l
	}
	return code
}

// AlternativeFields are the fields every alternative must fill.
var AlternativeFields = []string{"why", "difference", "pros", "cons"}

// MissingAlternative lists the required fields a is missing.
func MissingAlternative(a core.TechAlternative) []string {
	var out []string
	for _, f := range AlternativeFields {
		var ok bool
		switch f {
		case "why":
			ok = strings.TrimSpace(a.Why) != ""
		case "difference":
			ok = strings.TrimSpace(a.Difference) != ""
		case "pros":
			ok = len(a.Pros) > 0
		case "cons":
			ok = len(a.Cons) > 0
		}
		if !ok {
			out = append(out, f)
		}
	}
	return out
}

// ItemFields are the fields every decision must fill; ChosenFields also, when
// the area has a technology (not "Tidak perlu").
var (
	ItemFields   = []string{"reasoning", "driver", "confidence"}
	ChosenFields = []string{"purpose", "fulfills", "pros", "cons"}
)

// MissingItem lists the required fields it is missing, each as its field
// name; driver and confidence carry their allowed values.
func MissingItem(it core.TechItem) []string {
	var out []string
	need := func(ok bool, field string) {
		if !ok {
			out = append(out, field)
		}
	}
	need(strings.TrimSpace(it.Reasoning) != "", "reasoning")
	need(slices.Contains(core.TechDrivers, it.Driver), "driver ("+strings.Join(core.TechDrivers, "|")+")")
	need(slices.Contains(core.TechConfidence, it.Confidence), "confidence (high|medium|low)")
	if !it.None() {
		need(strings.TrimSpace(it.Purpose) != "", "purpose")
		need(len(it.Fulfills) > 0, "fulfills")
		need(len(it.Pros) > 0, "pros")
		need(len(it.Cons) > 0, "cons")
	}
	return out
}

// PlatformArea is an area a selected platform makes mandatory.
type PlatformArea struct {
	Area     string
	Platform string // the platform's name for the user
	Targets  string // the target kinds that select it, for the prompt
}

// PlatformAreas are the areas the platform requires, in a fixed order.
func PlatformAreas(pl core.TechPlatform) []PlatformArea {
	var out []PlatformArea
	mobile := []string{}
	if pl.Android {
		mobile = append(mobile, core.PlatformAndroid)
	}
	if pl.IOS {
		mobile = append(mobile, core.PlatformIOS)
	}
	if len(mobile) == 0 {
		mobile = append(mobile, "mobile")
	}
	for _, x := range []struct {
		on       bool
		area     string
		name     string
		selected string
	}{
		{pl.Web, core.TechFrontend, "web", core.PlatformWeb},
		{pl.Mobile, core.TechMobile, "mobile", strings.Join(mobile, ", ")},
		{pl.Desktop, core.TechDesktop, "desktop", core.PlatformDesktop},
		{pl.Backend, core.TechBackend, "backend", core.PlatformBackend},
		{pl.CLI, core.TechCLI, "CLI", core.PlatformCLI},
	} {
		if x.on {
			out = append(out, PlatformArea{Area: x.area, Platform: x.name, Targets: x.selected})
		}
	}
	return out
}

// StructureAreas are the areas every plan decides with a closed value.
var StructureAreas = []string{core.TechArchitecture, core.TechRepository}

// Contract renders the plan contract for a prompt: the mandatory areas of
// this project, the fields of a decision and of an alternative, and the
// review sections, all from the values the validator checks.
func Contract(st *core.TechStack) string {
	var b strings.Builder
	b.WriteString("CONTRACT (Foundry's validator refuses the answer when any line below is not met; compact never means leaving one out):\n")
	var pl core.TechPlatform
	if st != nil {
		pl = st.Platform
	}
	if areas := PlatformAreas(pl); len(areas) > 0 {
		b.WriteString("- MANDATORY AREAS from the selected target platform (the user's choice, binding here): ")
		var parts []string
		for _, a := range areas {
			parts = append(parts, fmt.Sprintf("%s (target %s)", a.Area, a.Targets))
		}
		b.WriteString(strings.Join(parts, "; "))
		b.WriteString(". Each one is an item with a real implementation technology; when the role of a platform is still unclear, still decide, with confidence low and the assumptions it rests on. Write \"" + core.TechNone + "\" for one only when the requirements clearly contradict the target, with the reason in reasoning: Foundry then shows it to the user as a platform conflict that blocks approval. Never leave a mandatory area out.\n")
	}
	if st != nil && st.Mode == core.StructureExisting {
		b.WriteString("- MANDATORY STRUCTURE AREAS: architecture and repository, as the existing code has them.\n")
	} else {
		fmt.Fprintf(&b, "- MANDATORY STRUCTURE AREAS: architecture (exactly %q, %q or %q) and repository (exactly %q or %q).\n",
			core.ArchSimple, core.ArchModular, core.ArchMicroservices, core.RepoSimple, core.RepoMonorepo)
	}
	fmt.Fprintf(&b, "- every item: area (unique), group (%s), technology, %s. With a technology (not %q) also: %s (at least one id each for fulfills, pros, cons).\n",
		strings.Join(core.TechPlanGroups, "|"), strings.Join([]string{"reasoning", "driver (" + strings.Join(core.TechDrivers, "|") + ")", "confidence (high|medium|low)"}, ", "),
		core.TechNone, strings.Join(ChosenFields, ", "))
	fmt.Fprintf(&b, "- every alternative object: technology, %s; pros and cons are non-empty lists specific to THIS system. An alternative you cannot explain this way is left out (alternatives are optional; an incomplete one is refused).\n", strings.Join(AlternativeFields, ", "))
	b.WriteString("- depends_on: only areas that are items of this plan.\n")
	fmt.Fprintf(&b, "- plan: approach, summary, context.summary and at least one context.facts entry {category, statement, source}.\n")
	fmt.Fprintf(&b, "- compatibility: at least one entry. consistency: one entry for EACH of %s.\n", strings.Join(core.TechConsistencyAreas, ", "))
	fmt.Fprintf(&b, "- evaluation.criteria: one entry for EACH of %s; score 1-5, reason of at least 20 characters, evidence naming item areas, requirement ids, feature ids or module ids.\n", strings.Join(core.TechCriteria, ", "))
	return b.String()
}
