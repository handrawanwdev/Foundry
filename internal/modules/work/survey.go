package work

import (
	"fmt"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// The survey binds the technical decisions: an answer is the user's
// decision, an empty one is left to Foundry and never becomes a constraint.

func labels(cs []core.Choice, ids []string) string {
	var out []string
	for _, id := range ids {
		out = append(out, core.ChoiceLabel(cs, id))
	}
	return strings.Join(out, ", ")
}

func existingProject(p *core.Project) bool {
	return p.Structure != nil && p.Structure.Mode == core.StructureExisting
}

// SurveyConflicts are the contradictions Foundry finds in the answers
// themselves; the AI adds those against the story.
func SurveyConflicts(p *core.Project) []core.Decision {
	s := p.Survey
	if s == nil {
		return nil
	}
	var out []core.Decision
	if s.Architecture == core.SurveyArchExisting && !existingProject(p) {
		out = append(out, core.Decision{Kind: core.DecisionConflict, Critical: true,
			Text: "Arsitektur dipilih \"mengikuti project yang sudah ada\", tetapi folder project belum berisi kode yang bisa dibaca. " +
				"Pilihan: biarkan Foundry memilih arsitektur, pilih satu aplikasi modular, atau buka folder project yang sudah ada."})
	}
	if s.Architecture == core.SurveyArchServices && s.Delivery() == core.SurveyStagePrototype {
		out = append(out, core.Decision{Kind: core.DecisionConflict, Critical: true,
			Text: "Beberapa service terpisah dipilih untuk prototype: deployment, komunikasi antar-service, dan debugging jadi lebih mahal sebelum ide tervalidasi. " +
				"Pilihan: satu aplikasi modular yang modulnya bisa dipisah nanti, atau tetap service terpisah dengan alasan yang dicatat."})
	}
	if s.Delivery() == core.SurveyStagePrototype {
		var heavy []string
		for _, x := range []string{core.SurveyPriorityScalability, core.SurveyPriorityOperations} {
			if s.HasPriority(x) {
				heavy = append(heavy, core.ChoiceLabel(core.SurveyPriorities, x))
			}
		}
		if len(heavy) > 0 {
			out = append(out, core.Decision{Kind: core.DecisionConflict, Area: core.AreaOperations,
				Text: fmt.Sprintf("Target prototype tetapi prioritas %s dipilih. Prototype tidak menyiapkan infrastruktur produksi. "+
					"Pilihan: naikkan target ke MVP/Production-ready, atau catat prioritas itu sebagai arah desain saja.", strings.Join(heavy, " dan "))})
		}
	}
	if s.Users == core.SurveyUsersSystem {
		for _, k := range s.Kinds {
			if k == core.SurveyKindWeb || k == core.SurveyKindMobile || k == core.SurveyKindDesktop {
				out = append(out, core.Decision{Kind: core.DecisionConflict,
					Text: "Pengguna dipilih \"sistem atau layanan lain\", tetapi platform berlayar (web/mobile/desktop) juga dipilih. " +
						"Pilihan: tambahkan pengguna manusia (mis. admin) yang memakai layar itu, atau hapus platform berlayar."})
				break
			}
		}
	}
	return out
}

// surveyPrompt is the survey part of the discovery prompt.
func surveyPrompt(p *core.Project) string {
	s := p.Survey
	if s == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(`SURVEY (the user's technical answers). "user:" is the user's explicit decision: honor it and never replace it with a recommendation.
"open" is left to Foundry: decide it from the story and the real needs, give the reason, and record a decision kind assumption when the story does not settle it.
Never turn an open answer into a constraint, and do not ask again what the story already says.
`)
	answer := func(name, v string) {
		if v == "" {
			v = "open"
		} else {
			v = "user: " + v
		}
		fmt.Fprintf(&b, "- %s: %s\n", name, v)
	}
	answer("Platform", labels(core.SurveyKinds, s.Kinds))
	answer("Users", core.ChoiceLabel(core.SurveyUsers, s.Users))
	stage := core.ChoiceLabel(core.SurveyStages, s.Delivery())
	if s.Stage == "" {
		stage += " (default)"
	}
	fmt.Fprintf(&b, "- Delivery target: %s — %s\n", stage, core.DeliveryCriteria[s.Delivery()])
	answer("Architecture", core.ChoiceLabel(core.SurveyArchs, s.Architecture))
	answer("Quality priorities", labels(core.SurveyPriorities, s.Priorities))
	if s.Constraints != "" {
		fmt.Fprintf(&b, "- Mandatory constraints (user's words):\n%s\n", indent(s.Constraints))
	} else {
		b.WriteString("- Mandatory constraints: none given\n")
	}
	if s.Latency != "" {
		fmt.Fprintf(&b, "- Latency budget p95 (earlier answer): < %s ms\n", s.Latency)
	}
	if s.HasWebhook() {
		fmt.Fprintf(&b, "- Webhook (earlier answer): %s\n", s.Webhook)
	}
	if len(s.Kinds) > 0 {
		b.WriteString("Do not drop a chosen platform.\n")
	}
	b.WriteString(`How the answers shape the blueprint:
- the delivery target sizes the scope and the Definition of Done: prototype = the main flow demonstrable, no production hardening;
  MVP = core features usable by real users, tests on the main flows; production-ready = security, automated tests, deployment,
  observability and operations are requirements with acceptance criteria.
- each quality priority becomes at least one non-functional requirement with a checkable acceptance criterion sized to the story.
  The basic qualities (input validation, access control fitting the users, tests of the main flow) hold even when not chosen.
  Do not invent numbers (latency, load, uptime) the story does not give: record them as a decision kind question.
- the users decide authentication and authorization: public without account = no sign-in flow; registered users = sign-up and sign-in;
  internal = company sign-in, no public sign-up; users and admins = separate roles and permissions; other systems = machine clients
  (service credentials), no human screens unless the story asks.
- an architecture answer is binding; when open, choose the simplest that fits the needs and say why. Do not pick separate services,
  webhooks, a latency target or a technology without a reason in the story or the answers.
- mandatory constraints are binding: apply each where it belongs (technology, integrations, compatibility, infrastructure, cost, regulation).
  A constraint that cannot be met, or that contradicts another answer or the story, is a decision kind conflict.
- check the answers against each other and against the story. A contradiction is a decision kind conflict, critical when it changes
  the architecture, security or data, naming both sides and the options to resolve it.
`)
	if cs := SurveyConflicts(p); len(cs) > 0 {
		b.WriteString("Foundry already records these conflicts; do not repeat them:\n")
		for _, c := range cs {
			fmt.Fprintf(&b, "- %s\n", c.Text)
		}
	}
	b.WriteString("\n")
	return b.String()
}

// surveyWorkflowRules tells the workflow planner what the answers mean for
// the commands.
func surveyWorkflowRules(p *core.Project) string {
	s := p.Survey
	if s == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "DELIVERY TARGET: %s — %s\n", core.ChoiceLabel(core.SurveyStages, s.Delivery()), core.DeliveryCriteria[s.Delivery()])
	switch s.Delivery() {
	case core.SurveyStagePrototype:
		b.WriteString("- keep the commands to the main flow; no deployment, observability or hardening commands unless the blueprint requires them.\n")
	case core.SurveyStageMVP:
		b.WriteString("- core features first; every command tests the flow it builds; one command makes the product runnable and deployable simply.\n")
	case core.SurveyStageProd:
		b.WriteString("- include commands for security hardening, automated test suites, the deployment pipeline, observability (logs, metrics, health) and operations (configuration, backup), each with verifiable acceptance criteria and evidence.\n")
	}
	if len(s.Priorities) > 0 {
		fmt.Fprintf(&b, "QUALITY PRIORITIES (user): %s. The commands touching them carry explicit acceptance criteria and verification evidence for them.\n",
			labels(core.SurveyPriorities, s.Priorities))
	}
	if s.Architecture != "" {
		fmt.Fprintf(&b, "ARCHITECTURE (user): %s. Commands follow it.\n", core.ChoiceLabel(core.SurveyArchs, s.Architecture))
	}
	if s.Constraints != "" {
		fmt.Fprintf(&b, "MANDATORY CONSTRAINTS (user):\n%s\nPut each one in the constraints of every command it affects; never contradict them.\n", indent(s.Constraints))
	}
	b.WriteString("\n")
	return b.String()
}

func indent(s string) string {
	return "  " + strings.ReplaceAll(strings.TrimSpace(s), "\n", "\n  ")
}
