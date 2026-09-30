package work

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/handrawanw/foundry/internal/core"
)

const KindSupport = "support"

// Support limits: the conversation is kept in the project state.
const (
	maxSupportMessages = 100
	maxQuestion        = 2000
	supportHistory     = 6 // earlier messages shown to the AI for follow-ups
)

var (
	ErrSupportBusy    = errors.New("asisten masih menjawab pertanyaan sebelumnya")
	ErrNoSupportIntel = errors.New("sistem belum dipahami: selesaikan tahap Ceritakan dulu")
)

// Ask records a question and a pending answer. status is SupportRunning
// (agent) or SupportWaiting (MCP host).
func Ask(p *core.Project, question, status string) error {
	question = strings.TrimSpace(question)
	switch {
	case question == "":
		return errors.New("tuliskan pertanyaan")
	case len(question) > maxQuestion:
		return fmt.Errorf("pertanyaan terlalu panjang (maks. %d karakter)", maxQuestion)
	case p.Discovery == nil:
		return ErrNoSupportIntel
	case PendingSupport(p) != nil:
		return ErrSupportBusy
	}
	if p.Support == nil {
		p.Support = &core.Support{}
	}
	now := time.Now().UTC()
	id := strconv.FormatInt(now.UnixNano(), 36)
	s := p.Support
	s.Messages = append(s.Messages,
		core.SupportMessage{ID: "q" + id, Role: "user", Text: question, At: now},
		core.SupportMessage{ID: "a" + id, Role: "assistant", Status: status, At: now})
	if n := len(s.Messages); n > maxSupportMessages {
		s.Messages = s.Messages[n-maxSupportMessages:]
	}
	return nil
}

// RetrySupport puts the last failed answer back to pending.
func RetrySupport(p *core.Project, status string) error {
	m := lastAnswer(p)
	if m == nil || m.Status != core.SupportFailed {
		return errors.New("tidak ada jawaban gagal untuk diulang")
	}
	m.Status, m.Error, m.At = status, "", time.Now().UTC()
	return nil
}

// FailSupport marks the pending answer failed.
func FailSupport(p *core.Project, err error) {
	if m := PendingSupport(p); m != nil {
		m.Status, m.Error = core.SupportFailed, err.Error()
	}
}

// PendingSupport is the answer being worked on, or nil.
func PendingSupport(p *core.Project) *core.SupportMessage {
	m := lastAnswer(p)
	if m != nil && (m.Status == core.SupportRunning || m.Status == core.SupportWaiting) {
		return m
	}
	return nil
}

func lastAnswer(p *core.Project) *core.SupportMessage {
	if p.Support == nil || len(p.Support.Messages) == 0 {
		return nil
	}
	m := &p.Support.Messages[len(p.Support.Messages)-1]
	if m.Role != "assistant" {
		return nil
	}
	return m
}

// question returns the user message the last answer belongs to.
func question(p *core.Project) string {
	ms := p.Support.Messages
	if n := len(ms); n >= 2 && ms[n-2].Role == "user" {
		return ms[n-2].Text
	}
	return ""
}

// ClearSupport starts a new conversation; escalations are kept.
func ClearSupport(p *core.Project) error {
	if PendingSupport(p) != nil {
		return ErrSupportBusy
	}
	if p.Support != nil {
		p.Support.Messages = nil
	}
	return nil
}

// Escalate stores the summary the user confirmed for the technical team.
func Escalate(p *core.Project, e core.Escalation) (*core.Escalation, error) {
	e.Issue = strings.TrimSpace(e.Issue)
	if e.Issue == "" {
		return nil, errors.New("tuliskan masalah yang dieskalasi")
	}
	e.Module, e.Flow, e.Status = strings.TrimSpace(e.Module), strings.TrimSpace(e.Flow), strings.TrimSpace(e.Status)
	e.Evidence, e.Causes, e.Investigation = nonEmpty(e.Evidence), nonEmpty(e.Causes), nonEmpty(e.Investigation)
	if p.Support == nil {
		p.Support = &core.Support{}
	}
	e.ID = fmt.Sprintf("ESC-%03d", len(p.Support.Escalations)+1)
	e.At = time.Now().UTC()
	p.Support.Escalations = append(p.Support.Escalations, e)
	return &p.Support.Escalations[len(p.Support.Escalations)-1], nil
}

// ---------- prompt ----------

// knowledge is what the assistant may know about the system: everything
// Foundry holds about the project, compacted.
func knowledge(p *core.Project) map[string]any {
	k := map[string]any{"name": p.Name, "stage": p.Stage, "discovery": p.Discovery,
		"requirements": p.Requirements, "features": p.Features, "flows": p.Flows}
	if p.Platform != nil {
		k["platforms"] = p.Platform.Targets
	}
	if s := p.Structure; s != nil {
		k["architecture"] = map[string]any{"mode": s.Mode, "modules": s.Components, "template": s.Template, "layout": s.Layout}
	}
	if p.Design != nil {
		var pages []map[string]any
		for _, pg := range p.Design.Pages {
			x := map[string]any{"id": pg.ID, "name": pg.Name, "role": pg.Role, "platform": pg.Platform,
				"feature_id": pg.FeatureID, "route": pg.Route, "access": pg.Access, "purpose": pg.Purpose}
			if pg.PrimaryAction != nil && pg.PrimaryAction.Label != "" {
				x["primary_action"] = pg.PrimaryAction.Label
			}
			if len(pg.SecondaryActions) > 0 {
				x["secondary_actions"] = pg.SecondaryActions
			}
			if pg.Data != nil && len(pg.Data.API) > 0 {
				x["api"] = pg.Data.API
			}
			if pg.States != nil {
				x["states"] = pg.States
			}
			var rules []string
			for _, c := range pg.Components {
				for _, f := range c.Fields {
					switch {
					case f.Required && f.Validation != "":
						rules = append(rules, f.Label+": required, "+f.Validation)
					case f.Required:
						rules = append(rules, f.Label+": required")
					case f.Validation != "":
						rules = append(rules, f.Label+": "+f.Validation)
					}
				}
			}
			if len(rules) > 0 {
				x["field_rules"] = rules
			}
			pages = append(pages, x)
		}
		k["pages"] = pages
	}
	var tasks []map[string]any
	for _, t := range p.Tasks {
		x := map[string]any{"id": t.ID, "title": t.Title, "area": t.Area, "status": t.Status, "feature_id": t.FeatureID, "files": t.Files}
		if n := len(t.Notes); n > 0 {
			x["last_note"] = t.Notes[n-1]
		}
		tasks = append(tasks, x)
	}
	if len(tasks) > 0 {
		k["tasks"] = tasks
	}
	var failed []map[string]any
	for i := len(p.Builds) - 1; i >= 0 && len(failed) < 3; i-- {
		if b := p.Builds[i]; !b.Passed {
			failed = append(failed, map[string]any{"id": b.ID, "task_id": b.TaskID, "reason": b.Reason, "at": b.At})
		}
	}
	if len(failed) > 0 {
		k["failed_builds"] = failed
	}
	if v := p.Verification; v != nil {
		k["verification"] = map[string]any{"ready": v.Ready, "issues": v.Issues}
	}
	return k
}

const supportRules = `RULES:
- Answer ONLY from SYSTEM KNOWLEDGE below and from the files in the current directory (you may read them; never modify anything).
- Never invent features, flows, roles, permissions, business rules, modules, APIs, statuses or data. When the system does not define something, say so plainly in "summary" and list it in "missing".
- You have NO access to live data, logs or a running app. For "why is my X pending/failed" questions: "status" says what the flow defines for that state, or that the live status is unknown; causes come from the flow, rules and code, and are "confirmed": true only with concrete evidence (file:line, build output). Otherwise they are possibilities.
- Write for a non-technical user, in the language of the question: what happens → where in the process → which flow → who is involved → why → what to do next. Technical names only in module/api fields.
- If the question is about a process, fill "flow" with its steps in order (use the flow steps of the system; enrich each step with detail, role, module, page_id, api when known). For a problem, mark the step where it is stuck with "current": true.
- For "who can ..." questions fill "roles" with each role and what it may do, citing the source (flow, page access, requirement). If no rule says who may do it, say so and list it in "missing".
- "sources": ids from the knowledge (REQ-, FEAT-, FLOW-, PAGE-, TASK-) and relative file paths you actually used.
- "escalate": true when you cannot resolve it (needs live data, a bug in code, or missing information).
- "follow_ups": 2-3 short next questions the user may ask.`

// SupportPrompt asks the AI to answer the pending question.
func SupportPrompt(p *core.Project) string {
	var b strings.Builder
	b.WriteString("You are the Technical Support Assistant of the system described below. You understand this system — its features, flows, roles, rules, modules and APIs — and help users understand processes and locate problems. You are not an FAQ bot.\n\n")
	b.WriteString(supportRules)
	b.WriteString("\n\nSYSTEM KNOWLEDGE:\n")
	b.WriteString(mustJSON(knowledge(p)))
	if p.Support != nil {
		ms := p.Support.Messages
		end := len(ms) - 2 // the pending question and answer
		start := max(0, end-supportHistory)
		if start < end {
			b.WriteString("\n\nEARLIER IN THIS CONVERSATION:\n")
			for _, m := range ms[start:end] {
				switch {
				case m.Role == "user":
					b.WriteString("User: " + m.Text + "\n")
				case m.Answer != nil:
					b.WriteString("Assistant: " + m.Answer.Summary + "\n")
				}
			}
		}
		b.WriteString("\nQUESTION:\n" + question(p) + "\n")
	}
	b.WriteString("\nReply with ONLY one JSON object in a ```json block, shape:\n" + `{
  "kind": "explain|flow|troubleshoot|roles|unknown",
  "summary": "plain answer, 1-4 short paragraphs",
  "flow": {"flow_id": "FLOW-001", "name": "", "steps": [{"name": "", "detail": "", "role": "", "module": "", "page_id": "PAGE-001", "api": "METHOD /path", "current": false}]},
  "trouble": {"problem": "", "status": "", "module": "", "step": "", "causes": [{"text": "", "evidence": "", "confirmed": false}], "actions": [""]},
  "roles": [{"role": "", "access": "what it may do", "source": "FLOW-001"}],
  "missing": [""],
  "sources": ["FLOW-001"],
  "follow_ups": [""],
  "escalate": false
}
Omit "flow", "trouble" and "roles" when they do not apply.`)
	return b.String()
}

// ---------- apply ----------

var supportKinds = map[string]bool{"explain": true, "flow": true, "troubleshoot": true, "roles": true, "unknown": true}

// ApplySupport lands the answer on the pending message. References the
// system does not have are dropped, so the UI never shows invented ids.
func ApplySupport(p *core.Project, raw string) error {
	m := PendingSupport(p)
	if m == nil {
		return errors.New("tidak ada pertanyaan yang menunggu jawaban")
	}
	var a core.SupportAnswer
	if err := ExtractJSON(raw, &a); err != nil {
		return err
	}
	a.Summary = strings.TrimSpace(a.Summary)
	if a.Summary == "" {
		return errors.New("jawaban AI kosong")
	}
	ids := knownIDs(p)
	a.Flow = normFlow(a.Flow, ids)
	a.Trouble = normTrouble(a.Trouble)
	a.Roles = normRoles(p, a.Roles)
	a.Missing, a.FollowUps = nonEmpty(a.Missing), nonEmpty(a.FollowUps)
	var src []string
	for _, s := range nonEmpty(a.Sources) {
		if (ids[s] || projectFile(p.Dir, s)) && !slices.Contains(src, s) {
			src = append(src, s)
		}
	}
	a.Sources = src
	a.Kind = strings.ToLower(strings.TrimSpace(a.Kind))
	if !supportKinds[a.Kind] {
		switch {
		case a.Trouble != nil:
			a.Kind = "troubleshoot"
		case a.Flow != nil:
			a.Kind = "flow"
		case len(a.Roles) > 0:
			a.Kind = "roles"
		default:
			a.Kind = "explain"
		}
	}
	m.Answer, m.Status, m.Error, m.At = &a, core.SupportDone, "", time.Now().UTC()
	return nil
}

func knownIDs(p *core.Project) map[string]bool {
	ids := map[string]bool{}
	for _, r := range p.Requirements {
		ids[r.ID] = true
	}
	for _, f := range p.Features {
		ids[f.ID] = true
		for _, s := range f.SubFeatures {
			ids[s.ID] = true
		}
	}
	for _, f := range p.Flows {
		ids[f.ID] = true
	}
	if p.Design != nil {
		for _, pg := range p.Design.Pages {
			ids[pg.ID] = true
		}
	}
	for _, t := range p.Tasks {
		ids[t.ID] = true
	}
	return ids
}

// projectFile reports whether s names a file inside the project; a
// ":line" suffix is allowed.
func projectFile(dir, s string) bool {
	if i := strings.LastIndexByte(s, ':'); i > 0 {
		if _, err := strconv.Atoi(strings.SplitN(s[i+1:], "-", 2)[0]); err == nil {
			s = s[:i]
		}
	}
	s = filepath.Clean(filepath.FromSlash(s))
	if s == "." || filepath.IsAbs(s) || s == ".." || strings.HasPrefix(s, ".."+string(filepath.Separator)) {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, s))
	return err == nil && !st.IsDir()
}

func normFlow(f *core.SupportFlow, ids map[string]bool) *core.SupportFlow {
	if f == nil {
		return nil
	}
	var steps []core.FlowStep
	current := false
	for _, s := range f.Steps {
		s.Name = strings.TrimSpace(s.Name)
		if s.Name == "" {
			continue
		}
		s.Detail, s.Role, s.Module, s.API = strings.TrimSpace(s.Detail), strings.TrimSpace(s.Role), strings.TrimSpace(s.Module), strings.TrimSpace(s.API)
		if !ids[s.PageID] {
			s.PageID = ""
		}
		s.Current = s.Current && !current // one current step at most
		current = current || s.Current
		steps = append(steps, s)
	}
	if len(steps) == 0 {
		return nil
	}
	if !ids[f.FlowID] {
		f.FlowID = ""
	}
	f.Name, f.Steps = strings.TrimSpace(f.Name), steps
	return f
}

func normTrouble(t *core.Troubleshoot) *core.Troubleshoot {
	if t == nil || strings.TrimSpace(t.Problem) == "" {
		return nil
	}
	t.Problem, t.Status, t.Module, t.Step = strings.TrimSpace(t.Problem), strings.TrimSpace(t.Status), strings.TrimSpace(t.Module), strings.TrimSpace(t.Step)
	var causes []core.Cause
	for _, c := range t.Causes {
		c.Text, c.Evidence = strings.TrimSpace(c.Text), strings.TrimSpace(c.Evidence)
		if c.Text == "" {
			continue
		}
		c.Confirmed = c.Confirmed && c.Evidence != "" // no evidence: a possibility
		causes = append(causes, c)
	}
	t.Causes, t.Actions = causes, nonEmpty(t.Actions)
	return t
}

// normRoles keeps roles the system defines, under their own names.
func normRoles(p *core.Project, in []core.RoleAccess) []core.RoleAccess {
	known := map[string]string{}
	add := func(n string) {
		if n = strings.TrimSpace(n); n != "" {
			known[strings.ToLower(n)] = n
		}
	}
	if d := p.Discovery; d != nil {
		for _, r := range d.Roles {
			add(r.Name)
		}
		for _, u := range d.Users {
			add(u)
		}
	}
	for _, f := range p.Flows {
		add(f.Role)
	}
	if p.Design != nil {
		for _, pg := range p.Design.Pages {
			add(pg.Role)
		}
	}
	var out []core.RoleAccess
	for _, r := range in {
		name, ok := known[strings.ToLower(strings.TrimSpace(r.Role))]
		if !ok || strings.TrimSpace(r.Access) == "" {
			continue
		}
		out = append(out, core.RoleAccess{Role: name, Access: strings.TrimSpace(r.Access), Source: strings.TrimSpace(r.Source)})
	}
	return out
}

