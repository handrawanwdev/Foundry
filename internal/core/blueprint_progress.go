package core

import "time"

// Blueprint generation task and section statuses. Distinct from JobStatus
// (one AI process) and from Progress (external coding-agent workflow).
const (
	BPTaskRunning   = "running"
	BPTaskCompleted = "completed"
	BPTaskFailed    = "failed"

	BPPending    = "pending"
	BPInProgress = "in_progress"
	BPCompleted  = "completed"
	BPFailed     = "failed"
	BPRetrying   = "retrying"
	BPSkipped    = "skipped"

	EvTaskStarted       = "task.started"
	EvSectionStarted    = "section.started"
	EvActivityUpdated   = "activity.updated"
	EvSectionCompleted  = "section.completed"
	EvSectionFailed     = "section.failed"
	EvSectionRetrying   = "section.retrying"
	EvTaskCompleted     = "task.completed"
	EvTaskFailed        = "task.failed"
)

const (
	maxBPEvents     = 64
	maxBPActivities = 40
)

// BlueprintProgress is the live tracker of one blueprint generation run.
// Written only at real pipeline transitions; the UI never invents it.
type BlueprintProgress struct {
	TaskID     string              `json:"task_id"`
	Status     string              `json:"status"` // running | completed | failed
	Activity   string              `json:"activity,omitempty"`
	StartedAt  time.Time           `json:"started_at"`
	EndedAt    *time.Time          `json:"ended_at,omitempty"`
	Seq        int64               `json:"seq"`
	LastEvent  string              `json:"last_event,omitempty"`
	Error      string              `json:"error,omitempty"`
	Sections   []BlueprintSection  `json:"sections"`
	Activities []BlueprintActivity `json:"activities,omitempty"`
	Events     []BlueprintEvent    `json:"events,omitempty"`
}

// BlueprintSection is one part of the blueprint this run is producing.
type BlueprintSection struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Kind      string     `json:"kind,omitempty"`
	Status    string     `json:"status"` // pending | in_progress | completed | failed | retrying | skipped
	Activity  string     `json:"activity,omitempty"`
	Error     string     `json:"error,omitempty"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// BlueprintActivity is one user-visible technical summary of work that
// actually happened. Not chain-of-thought.
type BlueprintActivity struct {
	Seq       int64     `json:"seq"`
	At        time.Time `json:"at"`
	SectionID string    `json:"section_id,omitempty"`
	Text      string    `json:"text"`
}

// BlueprintEvent is the SSE/reconnect payload for one transition.
type BlueprintEvent struct {
	Seq               int64     `json:"seq"`
	TaskID            string    `json:"taskId"`
	Event             string    `json:"event"`
	SectionID         string    `json:"sectionId,omitempty"`
	SectionName       string    `json:"sectionName,omitempty"`
	Activity          string    `json:"activity,omitempty"`
	Status            string    `json:"status,omitempty"`
	Error             string    `json:"error,omitempty"`
	CompletedSections int       `json:"completedSections"`
	TotalSections     int       `json:"totalSections"`
	Timestamp         time.Time `json:"timestamp"`
}

// NewBlueprintProgress starts a run with the given sections already stamped
// from current project truth (already-landed ones are completed).
func NewBlueprintProgress(id string, sections []BlueprintSection, at time.Time) *BlueprintProgress {
	t := &BlueprintProgress{
		TaskID:    id,
		Status:    BPTaskRunning,
		StartedAt: at,
		Sections:  sections,
	}
	t.emit(EvTaskStarted, "", "Merancang Blueprint", BPTaskRunning, "", at)
	return t
}

// Counts is completed/total, skipping sections left out on purpose.
func (t *BlueprintProgress) Counts() (done, total int) {
	if t == nil {
		return 0, 0
	}
	for _, s := range t.Sections {
		if s.Status == BPSkipped {
			continue
		}
		total++
		if s.Status == BPCompleted {
			done++
		}
	}
	return
}

// After returns events with seq greater than after, in order.
func (t *BlueprintProgress) After(after int64) []BlueprintEvent {
	if t == nil {
		return nil
	}
	var out []BlueprintEvent
	for _, e := range t.Events {
		if e.Seq > after {
			out = append(out, e)
		}
	}
	return out
}

// Section returns the named section, or nil.
func (t *BlueprintProgress) Section(id string) *BlueprintSection {
	return t.section(id)
}

func (t *BlueprintProgress) section(id string) *BlueprintSection {
	if t == nil || id == "" {
		return nil
	}
	for i := range t.Sections {
		if t.Sections[i].ID == id {
			return &t.Sections[i]
		}
	}
	return nil
}

// ResumeRunning reopens a failed run without dropping completed sections.
func (t *BlueprintProgress) ResumeRunning() {
	if t == nil || t.Status != BPTaskFailed {
		return
	}
	t.Status = BPTaskRunning
	t.Error = ""
	t.EndedAt = nil
}

// StartSection marks the section in progress. Completed/skipped stay put.
func (t *BlueprintProgress) StartSection(id, activity string, at time.Time) {
	s := t.section(id)
	if s == nil || s.Status == BPCompleted || s.Status == BPSkipped {
		return
	}
	first := s.Status == BPPending || s.Status == BPFailed
	if s.Status != BPRetrying {
		s.Status = BPInProgress
	}
	if s.StartedAt == nil {
		t0 := at
		s.StartedAt = &t0
	}
	s.EndedAt = nil
	s.Error = ""
	ev, st := EvSectionStarted, s.Status
	if s.Status == BPRetrying {
		ev = EvSectionRetrying
	} else if !first {
		ev = EvActivityUpdated
	}
	t.emit(ev, id, activity, st, "", at)
}

// RetrySection keeps prior completed work and shows the section retrying.
func (t *BlueprintProgress) RetrySection(id, activity string, at time.Time) {
	s := t.section(id)
	if s == nil || s.Status == BPCompleted || s.Status == BPSkipped {
		return
	}
	s.Status = BPRetrying
	s.EndedAt = nil
	if s.StartedAt == nil {
		t0 := at
		s.StartedAt = &t0
	}
	t.emit(EvSectionRetrying, id, activity, BPRetrying, s.Error, at)
}

// SetActivity records a real phase change (validate, wait, repair).
func (t *BlueprintProgress) SetActivity(id, activity string, at time.Time) {
	if t == nil || activity == "" {
		return
	}
	s := t.section(id)
	if t.Activity == activity && (s == nil || s.Activity == activity) {
		return
	}
	st := t.Status
	if s != nil {
		st = s.Status
	}
	t.emit(EvActivityUpdated, id, activity, st, "", at)
}

// CompleteSection is called only after the output landed and passed validation.
func (t *BlueprintProgress) CompleteSection(id string, at time.Time) {
	s := t.section(id)
	if s == nil || s.Status == BPCompleted || s.Status == BPSkipped {
		return
	}
	s.Status = BPCompleted
	s.Error = ""
	end := at
	s.EndedAt = &end
	if s.StartedAt == nil {
		s.StartedAt = &end
	}
	t.emit(EvSectionCompleted, id, "", BPCompleted, "", at)
}

// SkipSection leaves an area out (not applicable, or skipped with a reason).
func (t *BlueprintProgress) SkipSection(id string, at time.Time) {
	s := t.section(id)
	if s == nil || s.Status == BPCompleted || s.Status == BPSkipped {
		return
	}
	s.Status = BPSkipped
	s.Error = ""
	end := at
	s.EndedAt = &end
	t.emit(EvSectionCompleted, id, "", BPSkipped, "", at)
}

// FailSection records a safe failure reason. Completed siblings stay completed.
func (t *BlueprintProgress) FailSection(id, errMsg string, at time.Time) {
	s := t.section(id)
	if s == nil || s.Status == BPCompleted || s.Status == BPSkipped {
		return
	}
	s.Status = BPFailed
	s.Error = errMsg
	end := at
	s.EndedAt = &end
	t.emit(EvSectionFailed, id, "", BPFailed, errMsg, at)
}

// CompleteTask ends the run. Pending sections that were not in this run stay pending.
func (t *BlueprintProgress) CompleteTask(at time.Time) {
	if t == nil || t.Status != BPTaskRunning {
		return
	}
	t.Status = BPTaskCompleted
	end := at
	t.EndedAt = &end
	t.emit(EvTaskCompleted, "", t.Activity, BPTaskCompleted, "", at)
}

// FailTask ends the run as failed. Completed sections are kept.
func (t *BlueprintProgress) FailTask(errMsg string, at time.Time) {
	if t == nil || t.Status != BPTaskRunning {
		return
	}
	t.Status = BPTaskFailed
	t.Error = errMsg
	end := at
	t.EndedAt = &end
	t.emit(EvTaskFailed, "", t.Activity, BPTaskFailed, errMsg, at)
}

func (t *BlueprintProgress) emit(event, sectionID, activity, status, errMsg string, at time.Time) {
	if t == nil {
		return
	}
	t.Seq++
	name := ""
	if s := t.section(sectionID); s != nil {
		name = s.Name
		if activity != "" {
			s.Activity = activity
		}
	}
	if activity != "" {
		t.Activity = activity
		t.Activities = append(t.Activities, BlueprintActivity{
			Seq: t.Seq, At: at, SectionID: sectionID, Text: activity,
		})
		if len(t.Activities) > maxBPActivities {
			t.Activities = t.Activities[len(t.Activities)-maxBPActivities:]
		}
	}
	done, total := t.Counts()
	t.LastEvent = event
	t.Events = append(t.Events, BlueprintEvent{
		Seq: t.Seq, TaskID: t.TaskID, Event: event,
		SectionID: sectionID, SectionName: name, Activity: activity,
		Status: status, Error: errMsg,
		CompletedSections: done, TotalSections: total, Timestamp: at,
	})
	if len(t.Events) > maxBPEvents {
		t.Events = t.Events[len(t.Events)-maxBPEvents:]
	}
}
