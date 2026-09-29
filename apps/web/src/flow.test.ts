import { describe, expect, it } from "vitest";
import { isWorking, reachable, stepFor, taskCounts } from "./flow";
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

  it("counts tasks per column and detects work in progress", () => {
    const p = {
      tasks: [{ status: "TODO" }, { status: "DONE" }, { status: "DONE" }],
      job: { status: "waiting_host" },
    } as unknown as Project;
    expect(taskCounts(p)).toMatchObject({ TODO: 1, DONE: 2, BLOCKED: 0 });
    expect(isWorking(p)).toBe(true);
    expect(isWorking(null)).toBe(false);
  });
});
