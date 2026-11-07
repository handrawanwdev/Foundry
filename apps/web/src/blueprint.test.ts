import { describe, expect, it } from "vitest";
import { agentPrompt, BLUEPRINT_STEP_LABEL, blueprintGenerateNeeded, changeSummary, commandText, coverageRequest, DESIGN_DOCS, designIssuesOf, isDesignDoc, overviewTiles, SCOPE_DOCS, scopeIssuesOf } from "./blueprint";
import type { BlueprintView, ExportManifest, Project } from "./types";

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
    expect(p).toContain("report_progress");
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

describe("unified planning labels", () => {
  it("names the unified job, not three separate design stages", () => {
    expect(BLUEPRINT_STEP_LABEL.plan).toBe("Blueprint");
  });
});

describe("blueprintGenerateNeeded", () => {
  const base = { stage: "DESIGN", platform: { targets: [{ kind: "web" }] }, stack: {}, structure: { mode: "new" } } as Project;
  it("asks for a run when tech, structure or repo is missing", () => {
    expect(blueprintGenerateNeeded(base, null)).toBe(true);
    expect(blueprintGenerateNeeded({ ...base, ui: { rules: {} }, stack: { plan: {} }, structure: { mode: "new", plan: {}, repo: {} } } as Project, null)).toBe(false);
  });
  it("asks for a run when a stopped run left no structure after the technology", () => {
    expect(blueprintGenerateNeeded({ ...base, ui: { rules: {} }, stack: { plan: {} }, structure: undefined } as Project, null)).toBe(true);
  });
  it("asks for a run when an area is missing", () => {
    const view = { areas: [{ id: "data", status: "missing" }], artifacts: [], has_ui: false } as unknown as BlueprintView;
    expect(blueprintGenerateNeeded({ ...base, stack: { plan: {} }, structure: { mode: "existing" } } as Project, view)).toBe(true);
  });
});

describe("scopeIssuesOf", () => {
  const issues = [
    { level: "error" as const, where: "FLOW-001", message: "tujuan belum diisi" },
    { level: "error" as const, where: "REQ-002", message: "belum punya acceptance criteria" },
    { level: "error" as const, where: "acceptance", message: "Definition of Done belum diisi" },
    { level: "error" as const, where: "DEC-001", message: "isi keputusan kosong" },
    { level: "error" as const, where: "page:x", message: "bukan scope" },
  ];
  it("shows each issue on the artifact it belongs to", () => {
    expect(scopeIssuesOf(issues, SCOPE_DOCS.useCases).map((i) => i.where)).toEqual(["FLOW-001"]);
    expect(scopeIssuesOf(issues, SCOPE_DOCS.acceptance).map((i) => i.where)).toEqual(["REQ-002", "acceptance"]);
    expect(scopeIssuesOf(issues, SCOPE_DOCS.decisions).map((i) => i.where)).toEqual(["DEC-001"]);
  });
});

describe("execution dashboard", () => {
  const overview = { total: 6, completed: 1, in_progress: 1, pending: 2, blocked: 1, failed: 1, review: 0, revision: 0, skipped: 0, stale: 0, percent: 16 };

  it("always shows the six main numbers and adds the rest only when they hold something", () => {
    expect(overviewTiles(overview).map((t) => t.label)).toEqual(["Total", "Selesai", "Dikerjakan", "Belum mulai", "Terhambat", "Gagal"]);
    expect(overviewTiles({ ...overview, revision: 2 }).at(-1)).toMatchObject({ label: "Perlu revisi", value: 2 });
  });

  it("says what a revision did as a delta", () => {
    expect(changeSummary({ added: ["CMD-004"], modified: [{}, {}], reused: 3 })).toBe("1 ditambah · 2 diubah · 3 dipakai ulang");
  });

  it("asks only for what is not covered", () => {
    const text = coverageRequest(["REQ-002"], ["FEAT-003"]);
    expect(text).toContain("fitur FEAT-003");
    expect(text).toContain("requirement MUST REQ-002");
    expect(coverageRequest([], ["FEAT-003"])).not.toContain("requirement");
  });

  it("keeps the copied instruction in line with the exported one", () => {
    const text = commandText({ id: "CMD-001", name: "n", objective: "o", instructions: "i", status: "draft", priority: "high", complexity: "low", revision: 2, files: ["api/"] }, null);
    expect(text).toContain("Prioritas: high · Kompleksitas: low · Revisi 2");
    expect(text).toContain("## File dan modul terkait\n\n- api/");
  });
});
