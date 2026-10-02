import { describe, expect, it } from "vitest";
import { techGate, techReview, techState } from "./tech";
import type { Project, TechStack, TechVersion } from "./types";

const version = (n: number, design: boolean, area: "backend" | "deployment" = "backend"): TechVersion => ({
  version: n, at: "2026-10-02T00:00:00Z", source: "user", status: "impact_checked", decisions: [], design,
  changes: n > 1 ? [{ area, from: "NestJS + TypeScript", to: "Rust + Axum" }] : [],
});
const stack = (over: Partial<TechStack>): TechStack => ({
  mode: "new", platform: { web: true, mobile: false, desktop: false, backend: true }, decisions: [], approved: false, ...over,
});
const project = (s: TechStack, techVersion = 1) => ({
  stack: s, design: { pages: [{ id: "P1" }], status: "draft", feedback: null, tech_version: techVersion },
}) as unknown as Project;

describe("tech", () => {
  it("derives the status of stacks saved before the lifecycle", () => {
    expect(techState(stack({ approved: true }))).toBe("approved");
    expect(techState(stack({}))).toBe("recommended");
    expect(techState(stack({ status: "changed" }))).toBe("changed");
  });

  it("asks for a design review only for later changes that affect the design", () => {
    const s = stack({ status: "approved", approved: true, history: [version(1, false), version(2, true), version(3, false, "deployment")] });
    expect(techReview(project(s))).toHaveLength(1);
    expect(techReview(project(s, 2))).toHaveLength(0);
    expect(techGate(project(s))).toMatch(/periksa dampaknya/);
    expect(techGate(project(s, 3))).toBeNull();
  });

  it("blocks the design approval until the technology is approved", () => {
    expect(techGate(project(stack({ status: "impact_checked" })))).toBe("Setujui keputusan teknologi dulu");
    expect(techGate(project(stack({ mode: "existing" })))).toBeNull();
  });
});
