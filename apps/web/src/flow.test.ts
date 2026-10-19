import { describe, expect, it } from "vitest";
import { isWorking, reachable, stepFor, viewOf } from "./flow";
import type { Project } from "./types";

describe("flow", () => {
  it("maps internal stages to the two user steps", () => {
    expect(stepFor("TELL").label).toBe("Prompt");
    expect(stepFor("DISCOVERY").label).toBe("Prompt");
    expect(stepFor("DESIGN").label).toBe("Rancang");
    expect(stepFor("READY").label).toBe("Rancang");
  });

  it("locks steps the project has not reached", () => {
    expect(reachable("TELL", 1)).toBe(false);
    expect(reachable("DESIGN", 1)).toBe(true);
    expect(reachable("READY", 1)).toBe(true);
  });

  it("detects work in progress", () => {
    expect(isWorking({ job: { status: "waiting_host" } } as unknown as Project)).toBe(true);
    expect(isWorking({ job: { status: "done" } } as unknown as Project)).toBe(false);
    expect(isWorking(null)).toBe(false);
  });

  it("has no build views", () => {
    expect(viewOf("/simple/rancang")?.label).toBe("Rancang");
    expect(viewOf("/simple/bangun")).toBeNull();
    expect(viewOf("/simple/siap")).toBeNull();
  });
});
