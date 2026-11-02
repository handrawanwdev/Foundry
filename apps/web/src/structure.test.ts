import { describe, expect, it } from "vitest";
import { otherAlternatives, recommendedAlternative, structureGate, tree } from "./structure";
import type { Project, Structure, StructureIR, StructurePlan } from "./types";

const ir: StructureIR = {
  type: "modular", docs: "docs/",
  apps: [{ name: "web", path: "apps/web/" }],
  services: [{ name: "backend", path: "backend/", modules: ["booking", "payment"] }],
};
const project = (s: Partial<Structure>) =>
  ({ structure: { mode: "new", components: null, reason: "", commands: null, approved: false, scaffolded: false, ir, ...s } }) as unknown as Project;

describe("structure", () => {
  it("draws the IR as a folder tree", () => {
    expect(tree(ir)).toBe("apps/\n└── web/\nbackend/\n└── modules/\n    ├── booking/\n    └── payment/\ndocs/");
  });

  it("gates the design approval on an approved structure without errors", () => {
    expect(structureGate(project({}))).toMatch(/Setujui struktur/);
    expect(structureGate(project({ status: "approved", approved: true }))).toBeNull();
    expect(structureGate(project({ status: "approved", issues: [{ level: "error", message: "x" }] }))).toMatch(/masalah/);
    expect(structureGate(project({ mode: "existing", ir: undefined }))).toBeNull();
  });

  it("puts the AI recommendation first and keeps the other alternatives separate", () => {
    const plan = {
      summary: "s", recommendation: "b", reason: "r",
      alternatives: [
        { id: "a", name: "A", summary: "", type: "modular", ir, evaluation: { maintainability: 3, simplicity: 3, extensibility: 3, testability: 3, separation_of_concerns: 3, complexity: 3 } },
        { id: "b", name: "B", summary: "", type: "modular", ir, evaluation: { maintainability: 5, simplicity: 5, extensibility: 4, testability: 4, separation_of_concerns: 5, complexity: 2 } },
      ],
    } as unknown as StructurePlan;
    expect(recommendedAlternative(plan)?.id).toBe("b");
    expect(otherAlternatives(plan).map((a) => a.id)).toEqual(["a"]);
  });
});
