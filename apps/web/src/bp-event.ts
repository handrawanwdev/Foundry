import type { BlueprintEvent } from "./types";

// Merges a stream frame into the timeline. Identity is event_id, never text:
// a replayed frame adds nothing, and two attempts that say the same thing stay
// two entries. Order is the backend's sequence. Another run starts over.
export function mergeTimeline(cur: BlueprintEvent[], runId: string, events: BlueprintEvent[]): BlueprintEvent[] {
  const same = cur.filter((e) => e.run_id === runId);
  const seen = new Set(same.map((e) => e.event_id));
  const fresh = events.filter((e) => !seen.has(e.event_id));
  if (!fresh.length && same.length === cur.length) return cur;
  return [...same, ...fresh].sort((a, b) => a.sequence_number - b.sequence_number);
}

// The part of the timeline that belongs to this run.
export function timelineOf(cur: BlueprintEvent[], runId: string | undefined): BlueprintEvent[] {
  return cur.every((e) => e.run_id === runId) ? cur : cur.filter((e) => e.run_id === runId);
}
