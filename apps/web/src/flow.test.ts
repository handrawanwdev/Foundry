import { describe, expect, it } from "vitest";
import { COLUMNS, COLUMN_EMPTY, isWorking, queueItem, queueLabel, reachable, stepFor, taskCounts } from "./flow";
import type { Project } from "./types";

describe("flow", () => {
  it("maps internal stages to the four user steps", () => {
    expect(stepFor("TELL").label).toBe("Ceritakan");
    expect(stepFor("DISCOVERY").label).toBe("Ceritakan");
    expect(stepFor("DESIGN").label).toBe("Rancang");
    expect(stepFor("PLAN").label).toBe("Rancang");
    expect(stepFor("BUILD").label).toBe("Bangun");
    expect(stepFor("VERIFY").label).toBe("Bangun");
    expect(stepFor("READY").label).toBe("Siap");
  });

  it("locks steps the project has not reached", () => {
    expect(reachable("DESIGN", 1)).toBe(true);
    expect(reachable("DESIGN", 2)).toBe(false);
  });

  it("names columns in the product language", () => {
    expect(COLUMNS.map((c) => c.label)).toEqual(["Antrian", "Dikerjakan", "Review", "Selesai", "Terblokir"]);
    expect(COLUMN_EMPTY.IN_PROGRESS).toMatch(/dikerjakan/);
  });

  it("counts tasks per column and detects work in progress", () => {
    const p = {
      tasks: [{ status: "TODO" }, { status: "DONE" }, { status: "DONE" }],
      job: { status: "waiting_host" },
    } as unknown as Project;
    expect(taskCounts(p)).toMatchObject({ TODO: 1, DONE: 2, BLOCKED: 0 });
    expect(isWorking(p)).toBe(true);
    expect(isWorking(null)).toBe(false);
  });

  it("labels independent vs waiting vs locked tasks", () => {
    const p = {
      schedule: {
        independent: ["T2", "T4"],
        items: [
          { id: "T2", state: "independent" },
          { id: "T3", state: "queued", blocked_by: "T2", lock: "file:packages/ui" },
          { id: "T6", state: "waiting_deps", waiting: ["T4"] },
        ],
      },
    } as unknown as Project;
    expect(queueLabel(queueItem(p, "T2"))).toBe("siap paralel");
    expect(queueLabel(queueItem(p, "T3"))).toBe("dipakai T2 (file packages/ui)");
    expect(queueLabel(queueItem(p, "T6"))).toBe("menunggu T4");
  });
});
