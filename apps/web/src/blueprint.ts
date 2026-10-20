import type { AreaStatusKind, Artifact, ArtifactCategory, ArtifactStateKind, Command, DesignIssue, ExportManifest, Workflow, WorkflowReport } from "./types";

// The built-in artifacts of the UI design (core.ArtifactDesignRules / ArtifactLayoutSpec).
export const DESIGN_DOCS = { rules: "design-rules", layout: "layout-spec" } as const;

export const isDesignDoc = (id: string) => id === DESIGN_DOCS.rules || id === DESIGN_DOCS.layout;

// The design issues one design artifact shows: rules issues on the rules,
// the rest on the layout.
export function designIssuesOf(issues: DesignIssue[], doc: string): DesignIssue[] {
  const onRules = (i: DesignIssue) => i.where === "rules";
  return issues.filter((i) => (doc === DESIGN_DOCS.rules ? onRules(i) : !onRules(i)));
}

// The built-in artifacts of the blueprint scope, edited as structured data
// (core.ArtifactUseCases / ArtifactAcceptance / ArtifactDecisions).
export const SCOPE_DOCS = { useCases: "use-cases", acceptance: "acceptance", decisions: "decisions" } as const;

// The scope issues one scope artifact shows, by where they point
// (FLOW-001, REQ-001, DEC-001 or the artifact itself).
export function scopeIssuesOf(issues: DesignIssue[], doc: string): DesignIssue[] {
  const owner = (w = "") =>
    w.startsWith("FLOW-") || w === SCOPE_DOCS.useCases ? SCOPE_DOCS.useCases
      : w.startsWith("REQ-") || w === SCOPE_DOCS.acceptance ? SCOPE_DOCS.acceptance
        : w.startsWith("DEC-") ? SCOPE_DOCS.decisions : "";
  return issues.filter((i) => owner(i.where) === doc);
}

export const ARTIFACT_STATE_LABEL: Record<ArtifactStateKind, string> = {
  draft: "Draft", valid: "Valid", needs_review: "Perlu ditinjau", finalized: "Final",
};
// CSS state class (styles.css .state.st-*) of an artifact or area status.
export const ARTIFACT_STATE_CLASS: Record<ArtifactStateKind, string> = { draft: "st-issue", valid: "", needs_review: "st-review", finalized: "st-final" };

export const AREA_LABEL: Record<AreaStatusKind, string> = {
  covered: "Dicakup", invalid: "Belum valid", missing: "Belum ditulis", skipped: "Dilewati", not_applicable: "Tidak relevan",
};
export const AREA_CLASS: Record<AreaStatusKind, string> = { covered: "st-final", invalid: "st-issue", missing: "st-review", skipped: "", not_applicable: "" };

export const CATEGORY_LABEL: Record<ArtifactCategory, string> = {
  requirements: "Kebutuhan", domain: "Domain & bisnis", architecture: "Arsitektur", ui: "UI/UX",
  data: "Data & interface", platform: "Platform & operasional", quality: "Kualitas & keamanan", other: "Lainnya",
};
const ORDER = Object.keys(CATEGORY_LABEL) as ArtifactCategory[];

// Artifacts grouped by category, in blueprint order.
export function groupArtifacts(docs: Artifact[]): [ArtifactCategory, Artifact[]][] {
  return ORDER.map((c) => [c, docs.filter((d) => (ORDER.includes(d.category) ? d.category : "other") === c)] as [ArtifactCategory, Artifact[]])
    .filter(([, ds]) => ds.length > 0);
}

// Commands that reference an artifact.
export function usedBy(w: Workflow | null | undefined, artifact: string): string[] {
  return (w?.commands ?? []).filter((c) => c.artifacts?.some((r) => r.id === artifact)).map((c) => c.id);
}

export function dependents(w: Workflow | null | undefined, id: string): string[] {
  return (w?.commands ?? []).filter((c) => c.depends_on?.includes(id)).map((c) => c.id);
}

// 1-based wave of a command, 0 when the order is unknown (cycle).
export function waveOf(r: WorkflowReport | null, id: string): number {
  return (r?.waves.findIndex((w) => w.includes(id)) ?? -1) + 1;
}

export type CommandState = "issue" | "review" | "final" | "draft";

export function commandState(c: Command, r: WorkflowReport | null): CommandState {
  if (r?.issues.some((i) => i.command === c.id)) return "issue";
  if (r && c.id in r.review) return "review";
  return c.status === "finalized" ? "final" : "draft";
}

export const STATE_LABEL: Record<CommandState, string> = { issue: "Masalah", review: "Perlu ditinjau", final: "Final", draft: "Draft" };

// The command as a standalone instruction for a coding agent; matches the
// exported workflow/<id>.md.
export function commandText(c: Command, r: WorkflowReport | null): string {
  const out = [`# ${c.id} — ${c.name}`, ""];
  const meta = [`Status: ${c.status}`];
  const wave = waveOf(r, c.id);
  if (wave) meta.push(`Gelombang ${wave}`);
  meta.push(c.depends_on?.length ? `Bergantung pada: ${c.depends_on.join(", ")}` : "Tanpa dependency");
  out.push(meta.join(" · "), "");
  const section = (title: string, text?: string) => { if (text?.trim()) out.push(`## ${title}`, "", text.trim(), ""); };
  const list = (title: string, items?: string[], mark = "") => {
    if (items?.length) out.push(`## ${title}`, "", ...items.map((i) => `- ${mark}${i}`), "");
  };
  section("Tujuan", c.objective);
  section("Konteks", c.context);
  list("Acuan blueprint", c.artifacts?.map((a) => `blueprint/${a.id}.md`));
  if (c.covers?.length) out.push("## Mencakup requirement", "", `${c.covers.join(", ")} — kriteria penerimaannya di blueprint/acceptance.md.`, "");
  list("Input", c.inputs);
  section("Instruksi", c.instructions);
  list("Output yang diharapkan", c.expected_outputs);
  list("Acceptance criteria", c.acceptance_criteria, "[ ] ");
  list("Batasan", c.constraints);
  return out.join("\n");
}

// Ready-to-paste prompt for an external coding agent working from the
// export package at dir. wave is 1-based; 0 means the whole workflow, one
// wave at a time.
export function agentPrompt(m: ExportManifest, dir: string, wave = 0): string {
  const w = m.waves[wave - 1];
  const name = (id: string) => m.commands.find((c) => c.id === id)?.name ?? "";
  const out = [
    `Baca ${dir}/README.md dan ${dir}/manifest.json. Blueprint di ${dir}/blueprint/ adalah sumber kebenaran.`,
    "",
  ];
  if (w) {
    out.push(`Kerjakan Gelombang ${wave} saja:`, ...w.map((id) => `- ${dir}/workflow/${id}.md (${id} — ${name(id)})`), "");
    if (wave > 1) out.push(`Gelombang ${wave === 2 ? "1" : `1–${wave - 1}`} sudah selesai. Pastikan dependency tiap command memang sudah ada di repository sebelum mulai.`, "");
  } else {
    out.push(`Kerjakan command di ${dir}/workflow/ per gelombang (${m.waves.length} gelombang, ${m.commands.length} command), mulai dari Gelombang 1.`,
      "Jangan mulai command sebelum semua dependency-nya selesai.", "");
  }
  out.push(
    "Untuk tiap command:",
    "1. Baca file command dan artefak blueprint yang dirujuknya.",
    "2. Periksa repository dulu; pakai struktur dan konvensi yang ada, jangan mengarang path atau API.",
    "3. Implementasikan, lalu buktikan setiap acceptance criteria (test atau perintah beserta hasilnya).",
    "4. Laporkan: command ID, file yang diubah, bukti, dan criteria yang belum terpenuhi.",
    "",
    "Jika blueprint bertentangan dengan repository atau dengan command lain, berhenti dan tanya saya.",
    w ? "Setelah gelombang ini selesai, berhenti dan laporkan." : "Setelah satu gelombang selesai, berhenti dan tunggu konfirmasi saya sebelum lanjut.",
  );
  return out.join("\n");
}

export const lines = (s: string) => s.split("\n").map((l) => l.trim()).filter(Boolean);
