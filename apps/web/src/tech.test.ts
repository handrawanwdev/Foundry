import { describe, expect, it } from "vitest";
import { decisionState, needsApproval, techGate, techReview, techState } from "./tech";
import type { Project, TechStack, TechVersion } from "./types";

const version = (n: number, design: boolean, area: "backend" | "deployment" = "backend"): TechVersion => ({
  version: n, at: "2026-10-02T00:00:00Z", source: "user", status: "impact_checked", decisions: [], design,
  changes: n > 1 ? [{ area, from: "NestJS + TypeScript", to: "Rust + Axum" }] : [],
});
const stack = (over: Partial<TechStack>): TechStack => ({
  mode: "new", platform: { web: true, mobile: false, desktop: false, backend: true }, decisions: [], approved: false, ...over,
});
const project = (s: TechStack, techVersion = 1) => ({
  stage: "DESIGN", stack: s, ui: { layout: { shells: null, pages: [{ id: "p1" }] }, version: 1, tech_version: techVersion },
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

  it("asks an existing project to approve only what its code does not already use", () => {
    const code = { area: "backend" as const, recommended: "Go + Gin", reason: "Sudah dipakai di kode", source: "existing" as const };
    expect(needsApproval(stack({ mode: "existing", decisions: [code] }))).toBe(false);
    const added = stack({ mode: "existing", decisions: [code, { area: "mobile", recommended: "Flutter", reason: "x", source: "profile" }] });
    expect(needsApproval(added)).toBe(true);
    expect(techGate(project(added))).toBe("Setujui keputusan teknologi dulu");
  });

  it("tells a recommendation, a choice, an approval, a review and a conflict apart", () => {
    const d = { area: "ui" as const, recommended: "Tailwind CSS + shadcn/ui", reason: "x", source: "dependency" as const };
    const chosen = { ...d, override: { value: "MUI (Material UI)" } };
    expect(decisionState(stack({ decisions: [d] }), d)).toBe("recommended");
    expect(decisionState(stack({ decisions: [chosen] }), chosen)).toBe("selected");
    const approved = stack({ status: "approved", approved: true, decisions: [chosen],
      history: [{ ...version(1, false), status: "approved", decisions: [chosen] }] });
    expect(decisionState(approved, chosen)).toBe("approved");
    expect(decisionState(approved, { ...chosen, review: "Frontend berubah" })).toBe("needs_review");
    const conflict = stack({ decisions: [chosen], issues: [{ level: "error", area: "ui", message: "x" }] });
    expect(decisionState(conflict, chosen)).toBe("conflict");
    expect(decisionState(stack({ mode: "existing", decisions: [{ ...d, source: undefined }] }), { ...d, source: undefined })).toBe("existing");
  });
});
