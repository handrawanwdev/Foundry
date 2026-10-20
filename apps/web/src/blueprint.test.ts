import { describe, expect, it } from "vitest";
import { agentPrompt, DESIGN_DOCS, designIssuesOf, isDesignDoc } from "./blueprint";
import type { ExportManifest } from "./types";

const manifest = {
  waves: [["CMD-001"], ["CMD-002", "CMD-003"]],
  commands: [
    { id: "CMD-001", name: "Skema data" },
    { id: "CMD-002", name: "API login" },
    { id: "CMD-003", name: "Halaman login" },
  ],
} as unknown as ExportManifest;

describe("agentPrompt", () => {
  it("covers the whole workflow wave by wave", () => {
    const p = agentPrompt(manifest, "docs/foundry");
    expect(p).toContain("docs/foundry/README.md");
    expect(p).toContain("(2 gelombang, 3 command)");
    expect(p).toContain("tunggu konfirmasi");
    expect(p).not.toContain("CMD-002");
  });

  it("lists only the commands of one wave", () => {
    const p = agentPrompt(manifest, "docs/foundry", 2);
    expect(p).toContain("Kerjakan Gelombang 2 saja:");
    expect(p).toContain("- docs/foundry/workflow/CMD-003.md (CMD-003 — Halaman login)");
    expect(p).toContain("Gelombang 1 sudah selesai");
    expect(p).not.toContain("CMD-001.md");
  });

  it("falls back to the whole workflow for an unknown wave", () => {
    expect(agentPrompt(manifest, "docs/foundry", 9)).toBe(agentPrompt(manifest, "docs/foundry"));
  });
});

describe("design artifacts", () => {
  it("splits design issues between rules and layout", () => {
    const issues = [
      { level: "error" as const, where: "rules", message: "principles belum diisi" },
      { level: "error" as const, where: "page:antrean", message: "state wajib" },
      { level: "warning" as const, where: "layout", message: "fitur belum punya halaman" },
    ];
    expect(designIssuesOf(issues, DESIGN_DOCS.rules).map((i) => i.message)).toEqual(["principles belum diisi"]);
    expect(designIssuesOf(issues, DESIGN_DOCS.layout)).toHaveLength(2);
    expect(isDesignDoc("layout-spec") && isDesignDoc("design-rules") && !isDesignDoc("ui-design")).toBe(true);
  });
});
