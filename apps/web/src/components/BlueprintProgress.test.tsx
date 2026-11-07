import { describe, expect, it } from "vitest";
import type { AreaStatus, BlueprintView, Project } from "../types";
import { blueprintStages, buildingInitialBlueprint } from "./BlueprintProgress";

const area = (id: string, status: string, topics: string[] = []): AreaStatus =>
  ({ id, title: id.toUpperCase(), status, topics } as unknown as AreaStatus);
const view = (...areas: AreaStatus[]) => ({ areas } as unknown as BlueprintView);
const proj = (o: Record<string, unknown> = {}) => ({ ...o } as unknown as Project);
const job = (kind: string) => ({ kind, status: "running" });
const ids = (s: ReturnType<typeof blueprintStages>) => s.map((x) => x.id);
const stateOf = (s: ReturnType<typeof blueprintStages>, id: string) => s.find((x) => x.id === id)?.state;

describe("blueprintStages", () => {
  it("follows the project's areas, ui last, hiding what does not apply", () => {
    const v = view(area("ui", "missing"), area("security", "missing", ["a", "b", "c", "d", "e"]), area("migration", "not_applicable"));
    const s = blueprintStages(proj({ discovery: {} }), v);
    expect(ids(s)).toEqual(["discovery", "security", "ui", "tech", "arch", "repo"]);
    expect(s.find((x) => x.id === "security")?.hint).toBe("a, b, c, d");
  });

  it("keeps data and integration pending until discovery decides", () => {
    const v = view(area("data", "not_applicable"), area("integration", "not_applicable"));
    expect(ids(blueprintStages(proj(), v))).toContain("data");
    expect(ids(blueprintStages(proj({ discovery: {} }), v))).not.toContain("data");
  });

  it("marks covered done and skipped apart", () => {
    const s = blueprintStages(proj({ discovery: {} }), view(area("security", "covered"), area("operations", "skipped")));
    expect(stateOf(s, "security")).toBe("done");
    expect(stateOf(s, "operations")).toBe("skipped");
  });

  it("marks artifact areas running during the artifacts job, ui during design", () => {
    const v = view(area("security", "missing"), area("ui", "missing"), area("use_cases", "missing"));
    let s = blueprintStages(proj({ discovery: {}, job: job("artifacts") }), v);
    expect(stateOf(s, "security")).toBe("running");
    expect(stateOf(s, "ui")).toBe("pending");
    s = blueprintStages(proj({ discovery: {}, job: job("design") }), v);
    expect(stateOf(s, "ui")).toBe("running");
    expect(stateOf(s, "security")).toBe("pending");
  });

  it("runs the first open plan section only", () => {
    const base = { discovery: {}, job: job("plan"), stack: { plan_request: {} }, structure: { plan_request: {}, repo_request: {} } };
    let s = blueprintStages(proj(base), null);
    expect([stateOf(s, "tech"), stateOf(s, "arch"), stateOf(s, "repo")]).toEqual(["running", "pending", "pending"]);
    s = blueprintStages(proj({ ...base, stack: { plan: {} }, structure: { plan_request: {}, repo_request: {} } }), null);
    expect([stateOf(s, "tech"), stateOf(s, "arch"), stateOf(s, "repo")]).toEqual(["done", "running", "pending"]);
  });

  it("drops architecture and repository for existing projects", () => {
    const s = blueprintStages(proj({ discovery: {}, structure: { mode: "existing" } }), null);
    expect(ids(s)).toEqual(["discovery", "tech"]);
  });

  it("runs the areas the discovery job writes together with it", () => {
    const v = view(area("data", "missing"), area("security", "missing"), area("ui", "missing"));
    const s = blueprintStages(proj({ job: job("discovery") }), v);
    expect([stateOf(s, "discovery"), stateOf(s, "data"), stateOf(s, "security"), stateOf(s, "ui")]).toEqual(["running", "running", "running", "pending"]);
  });

  it("marks discovery running during the discovery job", () => {
    expect(stateOf(blueprintStages(proj({ job: job("discovery") }), null), "discovery")).toBe("running");
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
});
