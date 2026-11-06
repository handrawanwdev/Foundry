import { describe, expect, it } from "vitest";
import {
  ancestors, allChecks, allFindings, buildTree, circleLayout, drawnUnits, issueCounts, modulesAt, parentOf, repoGate, subtreeSize, usedBy, visibleRows,
} from "./repo";
import type { Project, RepoNode, RepoPlan } from "./types";

const node = (path: string, owner = "platform", extra: Partial<RepoNode> = {}): RepoNode => ({
  path, type: path.endsWith("/") ? "dir" : "file", responsibility: `tanggung jawab ${path}`, owner, reason: "r", ...extra,
});

const plan = (extra: Partial<RepoPlan> = {}): RepoPlan => ({
  approach: "A", topology: "T", summary: "S",
  nodes: [
    node("apps/"), node("apps/api/"), node("apps/api/modules/"), node("apps/api/modules/booking/", "booking"),
    node("apps/api/modules/booking/index.ts", "booking", { depends_on: ["packages/contracts/"] }),
    node("packages/"), node("packages/contracts/", "shared"),
  ],
  modules: [{ module: "booking", locations: [{ path: "apps/api/modules/booking/" }], frontend: false, backend: true }],
  dependency: { direction: "d" }, evaluation: { criteria: [], findings: [], score: 50 }, status: "recommended",
  analysis: { dirs: 6, files: 1, max_depth: 4 }, ...extra,
});

describe("repo tree", () => {
  it("nests nodes under their parent folder", () => {
    const roots = buildTree(plan().nodes);
    expect(roots.map((r) => r.node.path)).toEqual(["apps/", "packages/"]);
    const booking = roots[0].children[0].children[0].children[0];
    expect(booking.node.path).toBe("apps/api/modules/booking/");
    expect(booking.depth).toBe(3);
    expect(booking.children.map((c) => c.node.path)).toEqual(["apps/api/modules/booking/index.ts"]);
  });

  it("lists folders before files at each level", () => {
    const roots = buildTree([
      node("README.md"), node("apps/"), node("apps/api/"), node("apps/index.ts"),
    ]);
    expect(visibleRows(roots, new Set(["apps/"]), "").map((r) => r.node.path)).toEqual([
      "apps/", "apps/api/", "apps/index.ts", "README.md",
    ]);
  });

  it("collapses folders unless expanded, and a search opens the path to every match", () => {
    const roots = buildTree(plan().nodes);
    expect(visibleRows(roots, new Set(), "").map((r) => r.node.path)).toEqual(["apps/", "packages/"]);
    expect(visibleRows(roots, new Set(["apps/"]), "").map((r) => r.node.path)).toEqual(["apps/", "apps/api/", "packages/"]);
    expect(visibleRows(roots, new Set(), "index.ts").map((r) => r.node.path)).toEqual([
      "apps/", "apps/api/", "apps/api/modules/", "apps/api/modules/booking/", "apps/api/modules/booking/index.ts",
    ]);
    expect(visibleRows(roots, new Set(), "tidak ada")).toEqual([]);
  });

  it("finds parents and ancestors of a path", () => {
    expect(parentOf("a/b/c.ts")).toBe("a/b/");
    expect(parentOf("a/")).toBe("");
    expect(ancestors("a/b/c/")).toEqual(["a/", "a/b/"]);
  });
});

describe("repo relations", () => {
  it("tells who uses a node, which module is placed where, and how big a subtree is", () => {
    const p = plan();
    expect(usedBy(p, "packages/contracts/")).toEqual(["apps/api/modules/booking/index.ts"]);
    expect(modulesAt(p, "apps/api/modules/booking/")).toEqual(["booking"]);
    expect(subtreeSize(p, "apps/api/")).toBe(4);
    expect(subtreeSize(p, "apps/api/modules/booking/index.ts")).toBe(1);
  });
});

describe("dependency diagram", () => {
  it("places units on a circle and draws only units that matter", () => {
    const units = [{ name: "a", nodes: 2, fan_out: 1, fan_in: 0 }, { name: "b", nodes: 0, fan_out: 0, fan_in: 1 }, { name: "ghost", nodes: 0, fan_out: 0, fan_in: 0 }];
    const edges = [{ from: "a", to: "b", kind: "derived" as const }];
    expect(drawnUnits(units, edges).map((u) => u.name)).toEqual(["a", "b"]);
    const pos = circleLayout(drawnUnits(units, edges), 400);
    expect(pos.a).toEqual({ x: 200, y: 56 });
    expect(pos.b.y).toBe(344);
  });
});

describe("repo gate and summaries", () => {
  const project = (repo?: RepoPlan, mode: "new" | "existing" = "new") => ({ structure: { mode, repo } }) as unknown as Project;

  it("does not block a project without a plan, blocks an unapproved, stale or broken one", () => {
    expect(repoGate(project())).toBeNull();
    expect(repoGate(project(plan(), "existing"))).toBeNull();
    expect(repoGate(project(plan()))).toMatch(/Setujui/);
    expect(repoGate(project(plan({ status: "approved", stale: true })))).toMatch(/kedaluwarsa/);
    expect(repoGate(project(plan({ status: "approved", analysis: { dirs: 0, files: 0, max_depth: 0, issues: [{ level: "error", message: "x" }] } })))).toMatch(/masalah/);
    expect(repoGate(project(plan({ status: "approved" })))).toBeNull();
  });

  it("counts issues and lists findings worst first with the checks once", () => {
    const p = plan({
      evaluation: { criteria: [], score: 1, findings: [{ category: "other", severity: "info", message: "i", source: "ai" }] },
      analysis: {
        dirs: 1, files: 0, max_depth: 1, issues: [{ level: "error", message: "e" }, { level: "warning", message: "w" }],
        findings: [{ category: "circular-dependency", severity: "error", message: "c", source: "computed" }],
        checks: [{ area: "technology", status: "consistent", source: "ai" }, { area: "technology", status: "consistent", source: "ai" }, { area: "technology", status: "conflict", source: "computed" }],
      },
    });
    expect(issueCounts(p)).toEqual({ errors: 1, warnings: 1 });
    expect(allFindings(p).map((f) => f.severity)).toEqual(["error", "info"]);
    expect(allChecks(p)).toHaveLength(2);
  });
});
