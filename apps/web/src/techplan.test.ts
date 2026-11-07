import { describe, expect, it } from "vitest";
import { allCompat, allFindings, groupItems, isCoreGroup, issueCounts, planGate, planNotes } from "./techplan";
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
  it("groups items and keeps only the core groups always visible", () => {
    const groups = groupItems(plan());
    expect(groups.map((g) => g.id)).toEqual(["stack", "data", "app_infra", "architecture", "operations", "standards"]);
    expect(groups.filter((g) => isCoreGroup(g.id)).map((g) => g.id)).toEqual(["stack", "data", "architecture"]);
  });

  it("counts issues, findings, checks and computed compatibility", () => {
    const p = plan();
    expect(issueCounts(p)).toEqual({ errors: 1, warnings: 1 });
    expect(allFindings(p).map((f) => f.message)).toEqual(["ai", "computed"]);
    expect(allCompat(p).some((c) => c.source === "computed")).toBe(true);
  });

  it("lists only what needs attention, errors first", () => {
    const p = plan({
      evaluation: { criteria: [], score: 66, findings: [
        { category: "other", severity: "info", message: "hidden" },
        { category: "other", severity: "warning", message: "w" },
        { category: "other", severity: "warning", message: "risk", resolution: "fix" },
        { category: "other", severity: "error", message: "e" },
      ] },
      analysis: { items: 6, outside_catalog: 3, unverified: 1, diverged: ["data_migration"], issues: [{ level: "warning", message: "w" }, { level: "error", message: "e" }] },
    });
    expect(planNotes(p).map((n) => n.text)).toEqual(["e", "w", "risk Solusi: fix", "Evaluasi belum mencerminkan pilihan user: data migration."]);
  });

  it("gates approval on a missing, stale or conflicting plan", () => {
    expect(planGate({} as Project)).toBeNull();
    expect(planGate({ stack: { plan: plan() } } as unknown as Project)).toMatch(/masalah/);
    expect(planGate({ stack: { plan: plan({ stale: true, analysis: { items: 1, outside_catalog: 0, unverified: 0 } }) } } as unknown as Project)).toMatch(/Sumber/);
    expect(planGate({ stack: { plan: plan({ analysis: { items: 1, outside_catalog: 0, unverified: 0 } }) } } as unknown as Project)).toBeNull();
  });
});
