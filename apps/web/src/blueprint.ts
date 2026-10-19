import type { Artifact, ArtifactCategory, Command, Workflow, WorkflowReport } from "./types";

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
  list("Input", c.inputs);
  section("Instruksi", c.instructions);
  list("Output yang diharapkan", c.expected_outputs);
  list("Acceptance criteria", c.acceptance_criteria, "[ ] ");
  list("Batasan", c.constraints);
  return out.join("\n");
}

export const lines = (s: string) => s.split("\n").map((l) => l.trim()).filter(Boolean);
