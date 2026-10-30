import { describe, expect, it } from "vitest";
import { isWorking, reachable, STEPS, stepFor, stepOfPath } from "./flow";
import type { Project } from "./types";

describe("flow", () => {
  it("maps internal stages to the user steps", () => {
    expect(stepFor("TELL").label).toBe("Prompt");
    expect(stepFor("DISCOVERY").label).toBe("Fitur");
    expect(stepFor("DESIGN").label).toBe("Blueprint");
    expect(stepFor("READY").label).toBe("Export");
  });

  it("locks steps until a blueprint exists", () => {
    expect(reachable("TELL", 0)).toBe(true);
    expect(reachable("TELL", 1)).toBe(false);
    expect(reachable("TELL", 2)).toBe(false);
    expect(reachable("DISCOVERY", 1)).toBe(true);
    expect(reachable("DISCOVERY", 2)).toBe(false);
    expect(reachable("DESIGN", 2)).toBe(true);
    expect(reachable("DESIGN", 3)).toBe(true);
    expect(reachable("READY", 1)).toBe(true);
  });

  it("detects work in progress", () => {
    expect(isWorking({ job: { status: "waiting_host" } } as unknown as Project)).toBe(true);
    expect(isWorking({ job: { status: "done" } } as unknown as Project)).toBe(false);
    expect(isWorking(null)).toBe(false);
  });

  it("has the prompt, features, blueprint, workflow and export steps", () => {
    expect(STEPS.map((s) => s.path)).toEqual(["/simple/tell", "/features", "/blueprint", "/workflow", "/export"]);
    expect(stepOfPath("/features")).toBe(1);
    expect(stepOfPath("/blueprint/ui")).toBe(2);
    expect(stepOfPath("/export")).toBe(4);
  });
});
