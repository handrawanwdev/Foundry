import { describe, expect, it } from "vitest";
import { structureGate, tree } from "./structure";
import type { Project, Structure, StructureIR } from "./types";

const ir: StructureIR = {
  type: "modular", docs: "docs/",
  apps: [{ name: "web", path: "apps/web/" }],
  services: [{ name: "backend", path: "backend/", modules: ["booking", "payment"] }],
};
const project = (s: Partial<Structure>) =>
  ({ structure: { mode: "new", components: null, reason: "", commands: null, approved: false, scaffolded: false, ir, ...s } }) as unknown as Project;

describe("structure", () => {
  it("draws the IR as a folder tree", () => {
    expect(tree(ir)).toBe("apps/\n└── web/\nbackend/\n├── booking/\n└── payment/\ndocs/");
  });

  it("gates the design approval on an approved structure without errors", () => {
    expect(structureGate(project({}))).toMatch(/Setujui struktur/);
    expect(structureGate(project({ status: "approved", approved: true }))).toBeNull();
    expect(structureGate(project({ status: "approved", issues: [{ level: "error", message: "x" }] }))).toMatch(/masalah/);
    expect(structureGate(project({ mode: "existing", ir: undefined }))).toBeNull();
  });
});
