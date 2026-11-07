import type { BlueprintActivity, BlueprintEvent, BlueprintProgress, BlueprintSectionStatus } from "./types";

export function foldProgressEvent(bp: BlueprintProgress | undefined, ev: BlueprintEvent): BlueprintProgress {
  const base: BlueprintProgress = bp ?? {
    task_id: ev.taskId, status: "running", started_at: ev.timestamp, seq: 0, sections: [], activities: [], events: [],
  };
  if (ev.seq <= (base.seq ?? 0)) return base;
  const sections = base.sections.map((s) => {
    if (s.id !== ev.sectionId) return s;
    const raw = ev.status as BlueprintSectionStatus;
    const ok = raw === "skipped" || raw === "pending" || raw === "in_progress" || raw === "completed" || raw === "failed" || raw === "retrying";
    const status = ok ? raw : s.status;
    return {
      ...s,
      status,
      activity: ev.activity ?? s.activity,
      error: ev.event === "section.failed" ? ev.error : (status === "completed" ? undefined : s.error),
    };
  });
  const activities: BlueprintActivity[] = ev.activity
    ? [...(base.activities ?? []), { seq: ev.seq, at: ev.timestamp, section_id: ev.sectionId, text: ev.activity }].slice(-40)
    : (base.activities ?? []);
  const events = [...(base.events ?? []), ev].slice(-64);
  let status = base.status;
  if (ev.event === "task.completed") status = "completed";
  if (ev.event === "task.failed") status = "failed";
  if (ev.event === "task.started") status = "running";
  return {
    ...base,
    seq: ev.seq,
    activity: ev.activity || base.activity,
    status,
    error: ev.event === "task.failed" ? ev.error : base.error,
    sections,
    activities,
    events,
  };
}
