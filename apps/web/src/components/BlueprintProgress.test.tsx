import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it } from "vitest";
import { mergeTimeline, timelineOf } from "../bp-event";
import { useProject } from "../store/project";
import type { BlueprintEvent, BlueprintProgress as Run, BlueprintSection, BlueprintSummary, Project } from "../types";
import { BlueprintProgress, buildingInitialBlueprint, countLine, etaText, executionLine, runBanner, Timeline } from "./BlueprintProgress";

const proj = (o: Record<string, unknown> = {}) => ({ ...o } as unknown as Project);
const job = (kind: string) => ({ kind, status: "running" });

const summary = (o: Partial<BlueprintSummary> = {}): BlueprintSummary => ({
  total: 12, applicable: 10, completed: 3, active: 1, pending: 4, blocked: 1, failed: 1, skipped: 2, percent: 30,
  tasks_total: 9, tasks_completed: 7, tasks_active: 1, tasks_failed: 1, executions: 5, retries: 2, ...o,
});
const run = (o: Partial<Run> = {}): Run => ({
  run_id: "run-1", status: "running", started_at: "2026-11-07T10:00:00Z", seq: 9, plan_version: 1,
  summary: summary(), sections: [], ...o,
});
const ev = (seq: number, o: Partial<BlueprintEvent> = {}): BlueprintEvent => ({
  event_id: `run-1-${seq}`, run_id: "run-1", sequence_number: seq, timestamp: "2026-11-07T10:00:00Z",
  activity_type: "section.started", message: `m${seq}`, ...o,
});

afterEach(() => useProject.setState({ project: null, timeline: [], stream: "open" }));

describe("run numbers", () => {
  it("states every status apart and never adds skipped to the denominator", () => {
    expect(countLine(run())).toBe("3/10 bagian selesai · 30% · 1 berjalan · 4 menunggu · 1 terhambat · 1 gagal · 2 dilewati");
  });
  it("keeps execution progress separate from section progress", () => {
    expect(executionLine(run())).toBe("7/9 task · 5 percobaan AI · 2 perbaikan/pengulangan · 1 task gagal");
  });
  it("shows an estimate only with a basis", () => {
    expect(etaText(run())).toMatch(/belum tersedia/);
    expect(etaText(run({ eta: { remaining_ms: 90000, samples: 4 } }))).toBe("Sisa ±1:30, dari 4 pekerjaan sebelumnya");
    expect(etaText(run({ eta: { remaining_ms: 90000, samples: 4, unknown_sections: 2 } }))).toMatch(/minimal ±1:30, 2 bagian/);
    expect(etaText(run({ status: "failed", eta: { remaining_ms: 1, samples: 2 } }))).toBe("");
  });
  it("closes with what really happened", () => {
    const done = summary({ completed: 10, percent: 100 });
    expect(runBanner(run({ status: "completed", summary: done }))).toBe("Blueprint selesai · 10/10 bagian tervalidasi");
    expect(runBanner(run({ status: "incomplete" }))).toBe("Blueprint belum lengkap · 3/10 bagian tervalidasi");
    expect(runBanner(run({ status: "stopped" }))).toBe("Perancangan ditutup · 3/10 bagian tervalidasi");
    expect(runBanner(run({ status: "running" }))).toBe("");
    expect(runBanner(undefined)).toBe("");
  });
});

describe("mergeTimeline", () => {
  it("ignores a replayed frame", () => {
    const cur = [ev(1), ev(2)];
    expect(mergeTimeline(cur, "run-1", [ev(2)])).toBe(cur);
  });
  it("orders by sequence, not by arrival", () => {
    const out = mergeTimeline([ev(3)], "run-1", [ev(1), ev(2)]);
    expect(out.map((e) => e.sequence_number)).toEqual([1, 2, 3]);
  });
  it("keeps equal text from different executions", () => {
    const out = mergeTimeline([ev(1, { message: "Mengulang", execution_id: "exec-1" })], "run-1",
      [ev(2, { message: "Mengulang", execution_id: "exec-2" })]);
    expect(out).toHaveLength(2);
  });
  it("starts over for another run", () => {
    const other = ev(5, { event_id: "run-2-5", run_id: "run-2" });
    expect(mergeTimeline([ev(1), ev(2)], "run-2", [other])).toEqual([other]);
  });
});

describe("timelineOf", () => {
  it("keeps what the stream delivered before the first load", () => {
    const cur = [ev(1), ev(2)];
    expect(timelineOf(cur, "run-1")).toBe(cur);
  });
  it("drops another run's events", () => {
    expect(timelineOf([ev(1)], "run-2")).toEqual([]);
    expect(timelineOf([ev(1)], undefined)).toEqual([]);
  });
});

describe("applyProgress", () => {
  it("replaces state with the backend frame", () => {
    useProject.setState({ project: proj({ blueprint_progress: run({ seq: 3 }) }) });
    useProject.getState().applyProgress({ progress: run({ seq: 4, summary: summary({ completed: 4 }) }), events: [ev(4)] });
    expect(useProject.getState().project?.blueprint_progress?.summary.completed).toBe(4);
    expect(useProject.getState().timeline).toHaveLength(1);
  });
  it("does not let an old frame overwrite a newer status, but keeps its events", () => {
    useProject.setState({ project: proj({ blueprint_progress: run({ seq: 9, status: "failed" }) }) });
    useProject.getState().applyProgress({ progress: run({ seq: 4 }), events: [ev(4)] });
    expect(useProject.getState().project?.blueprint_progress?.status).toBe("failed");
    expect(useProject.getState().timeline).toHaveLength(1);
  });
  it("is unchanged by a duplicate frame", () => {
    useProject.setState({ project: proj({ blueprint_progress: run({ seq: 4 }) }) });
    const frame = { progress: run({ seq: 4 }), events: [ev(4)] };
    useProject.getState().applyProgress(frame);
    useProject.getState().applyProgress(frame);
    expect(useProject.getState().timeline).toHaveLength(1);
  });
});

describe("BlueprintProgress view", () => {
  const section = (o: Partial<BlueprintSection> & { id: string }): BlueprintSection =>
    ({ name: o.id.toUpperCase(), status: "pending", ...o });

  it("numbers only applicable sections, so the list matches the denominator", () => {
    const sections = [
      section({ id: "a", status: "completed" }),
      section({ id: "b", status: "skipped", skip_kind: "not_applicable", reason: "tidak ada integrasi" }),
      section({ id: "c" }),
    ];
    const html = renderToStaticMarkup(<BlueprintProgress project={proj({
      blueprint_progress: run({ sections, summary: summary({ total: 3, applicable: 2, completed: 1, skipped: 1, percent: 50 }) }),
    })} />);
    expect(html).toContain("1. A");
    expect(html).toContain("2. C");
    expect(html).not.toContain("3. ");
    expect(html).toContain("Dilewati (1)");
    expect(html).toContain("Tidak relevan");
    expect(html).toContain("tidak ada integrasi");
  });

  it("shows attempt, task, retry history and validation of a section", () => {
    const sections = [section({
      id: "arch", status: "repairing", attempt: 2, validation: "failed",
      activity: "Memperbaiki output ARCH (percobaan 2)",
      validation_issues: ["relasi tidak valid"],
      tasks: [{ id: "arch.repair.2", kind: "repair", title: "Memperbaiki output: relasi tidak valid", status: "running", attempt: 2 }],
      executions: [{ id: "exec-4", task_id: "t", attempt: 1, trigger: "initial", status: "rejected", failure_kind: "validation", started_at: "2026-11-07T10:00:00Z" }],
    })];
    const html = renderToStaticMarkup(<BlueprintProgress project={proj({ blueprint_progress: run({ sections }) })} />);
    for (const want of ["percobaan 2", "Validasi gagal", "Memperbaiki output: relasi tidak valid", "Ditolak validasi", "exec-4", "relasi tidak valid"]) {
      expect(html).toContain(want);
    }
  });

  it("gives a blocked section its reason and names parallel work", () => {
    const sections = [
      section({ id: "a", status: "in_progress" }),
      section({ id: "b", status: "validating" }),
      section({ id: "c", status: "blocked", reason: "Menunggu A" }),
    ];
    const html = renderToStaticMarkup(<BlueprintProgress project={proj({ blueprint_progress: run({ sections }) })} />);
    expect(html).toContain("2 bagian paralel: A, B");
    expect(html).toContain("Menunggu A");
    expect(html).toContain("Terhambat");
  });

  it("renders the timeline newest first with its time", () => {
    const html = renderToStaticMarkup(<Timeline events={[ev(1, { message: "pertama" }), ev(2, { message: "kedua", attempt_number: 2 })]} />);
    expect(html.indexOf("kedua")).toBeLessThan(html.indexOf("pertama"));
    expect(html).toContain("Aktivitas AI (2)");
    expect(html).toContain("<time");
  });

  it("offers retry and close on a failed run", () => {
    const html = renderToStaticMarkup(<BlueprintProgress project={proj({ blueprint_progress: run({ status: "failed", error: "gagal total" }) })} />);
    expect(html).toContain("Perancangan Blueprint gagal");
    expect(html).toContain("gagal total");
    expect(html).toContain("Coba lagi");
    expect(html).toContain("Tutup");
  });

  it("survives a job that has no run yet", () => {
    const html = renderToStaticMarkup(<BlueprintProgress project={proj({ job: { kind: "discovery", status: "running", message: "Merancang Blueprint" } })} />);
    expect(html).toContain("Merancang Blueprint");
  });
});

describe("buildingInitialBlueprint", () => {
  const q = ["plan"];
  it("covers discovery, design and plan until all plan sections landed", () => {
    expect(buildingInitialBlueprint(proj({ job: job("discovery") }))).toBe(true);
    expect(buildingInitialBlueprint(proj({ discovery: {}, job: job("design"), blueprint_queue: ["design", "plan"] }))).toBe(true);
    expect(buildingInitialBlueprint(proj({ discovery: {}, job: job("plan"), blueprint_queue: q, stack: { plan: {} }, structure: {} }))).toBe(true);
  });
  it("ends when tech and repo landed, or when nothing is queued", () => {
    expect(buildingInitialBlueprint(proj({ discovery: {}, job: job("plan"), blueprint_queue: q, stack: { plan: {} }, structure: { repo: {} } }))).toBe(false);
    expect(buildingInitialBlueprint(proj({ discovery: {}, job: job("artifacts") }))).toBe(false);
    expect(buildingInitialBlueprint(proj({ discovery: {}, blueprint_queue: q }))).toBe(false);
  });
  it("stays on the tracker while backend progress is running or failed", () => {
    expect(buildingInitialBlueprint(proj({ discovery: {}, blueprint_progress: { status: "running", sections: [] } }))).toBe(true);
    expect(buildingInitialBlueprint(proj({
      discovery: {}, job: { kind: "plan", status: "failed" },
      blueprint_progress: { status: "failed", sections: [] },
    }))).toBe(true);
    expect(buildingInitialBlueprint(proj({ discovery: {}, blueprint_progress: { status: "completed", sections: [] } }))).toBe(false);
    expect(buildingInitialBlueprint(proj({ discovery: {}, blueprint_progress: { status: "stopped", sections: [] } }))).toBe(false);
    expect(buildingInitialBlueprint(proj({ discovery: {}, blueprint_progress: { status: "incomplete", sections: [] } }))).toBe(false);
  });
});
