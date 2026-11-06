import { describe, expect, it } from "vitest";
import { allChecks, allCompat, allFindings, groupItems, issueCounts, itemsOf, planGate, planSummary } from "./techplan";
import type { Project, TechItem, TechPlan } from "./types";

const item = (area: string, group: string, extra: Partial<TechItem> = {}): TechItem => ({
  area, group, technology: extra.technology ?? area, purpose: "p", driver: "requirement", reasoning: "r", confidence: "high", ...extra,
});

const plan = (extra: Partial<TechPlan> = {}): TechPlan => ({
  approach: "A", summary: "S", context: { summary: "c" },
  items: [
    item("backend", "stack", { technology: "Elixir + Phoenix", in_catalog: false }),
    item("database", "data", { technology: "TimescaleDB" }),
    item("authentication", "app_infra", { technology: "Phoenix.Session" }),
    item("deployment", "operations", { technology: "Fly.io Machines" }),
    item("architecture", "architecture", { technology: "Modular Monolith" }),
    item("testing", "standards", { technology: "ExUnit" }),
  ],
  evaluation: { criteria: [], findings: [{ category: "unverified", severity: "info", message: "ai", source: "ai" }], score: 66 },
  analysis: {
    items: 6, outside_catalog: 3, unverified: 1,
    issues: [{ level: "warning", message: "w" }, { level: "error", message: "e" }],
    findings: [{ category: "unverified", severity: "info", message: "computed", source: "computed" }],
    checks: [{ area: "requirements", status: "consistent", source: "computed" }],
    compat: [{ area: "language", status: "conflict", source: "computed" }],
  },
  ...extra,
});

describe("techplan helpers", () => {
  it("groups items and lists them per workspace tab", () => {
    const p = plan();
    const groups = groupItems(p);
    expect(groups.map((g) => g.id)).toEqual(["stack", "data", "app_infra", "architecture", "operations", "standards"]);
    expect(itemsOf(p, "stack").map((i) => i.area)).toEqual(["backend"]);
    expect(itemsOf(p, "infra").map((i) => i.area)).toEqual(["authentication", "deployment"]);
    expect(itemsOf(p, "data").map((i) => i.area)).toEqual(["database"]);
  });

  it("counts issues, findings, checks and computed compatibility", () => {
    const p = plan();
    expect(issueCounts(p)).toEqual({ errors: 1, warnings: 1 });
    expect(allFindings(p).map((f) => f.message)).toEqual(["ai", "computed"]);
    expect(allChecks(p).map((c) => c.source)).toEqual(["computed"]);
    expect(allCompat(p).some((c) => c.source === "computed")).toBe(true);
    expect(planSummary(p)).toEqual({ items: 6, outside: 3, unverified: 1, score: 66 });
  });

  it("gates approval on a missing, stale or conflicting plan", () => {
    expect(planGate({} as Project)).toBeNull();
    expect(planGate({ stack: { plan: plan() } } as unknown as Project)).toMatch(/masalah/);
    expect(planGate({ stack: { plan: plan({ stale: true, analysis: { items: 1, outside_catalog: 0, unverified: 0 } }) } } as unknown as Project)).toMatch(/Sumber/);
    expect(planGate({ stack: { plan: plan({ analysis: { items: 1, outside_catalog: 0, unverified: 0 } }) } } as unknown as Project)).toBeNull();
  });
});
