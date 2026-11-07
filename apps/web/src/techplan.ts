import type { Project, TechCompat, TechFinding, TechItem, TechPlan } from "./types";

export const PLAN_GROUP: Record<string, string> = {
  stack: "Application Stack", data: "Data Architecture", app_infra: "Application Infrastructure",
  architecture: "Architecture & Repository", operations: "Infrastructure & Operations", standards: "Engineering Standards",
};

export const PLAN_SECTION: Record<string, string> = {
  context: "Konteks", overview: "Ringkasan", stack: "Stack", data: "Data",
  infra: "Infra", architecture: "Arsitektur", standards: "Standar",
  compatibility: "Kompatibilitas", alternatives: "Alternatif", reasoning: "Alasan",
  evaluation: "Evaluasi", history: "Riwayat",
};

export const PLAN_SOURCE: Record<string, string> = {
  requirements: "Requirement & fitur", modules: "Project Modules", platform: "Platform", data: "Data",
  integrations: "Integrasi", security: "Keamanan", folder: "Struktur folder",
};

export const NONE = "Tidak perlu";

// Groups always visible; the rest fold so the page stays short.
const CORE_GROUPS = ["stack", "data", "architecture"];

export function areaLabel(area: string): string {
  return area.replace(/_/g, " ");
}

export function isNone(it: TechItem): boolean {
  return !it.technology || it.technology === NONE;
}

export function isCoreGroup(id: string): boolean {
  return CORE_GROUPS.includes(id);
}

export function groupItems(plan: TechPlan): { id: string; title: string; items: TechItem[] }[] {
  return Object.keys(PLAN_GROUP).map((id) => ({ id, title: PLAN_GROUP[id], items: plan.items.filter((it) => it.group === id) }))
    .filter((g) => g.items.length > 0);
}

export function issueCounts(plan: TechPlan): { errors: number; warnings: number } {
  const issues = plan.analysis?.issues ?? [];
  return { errors: issues.filter((i) => i.level === "error").length, warnings: issues.filter((i) => i.level !== "error").length };
}

export function planGate(p: Project): string | null {
  const plan = p.stack?.plan;
  if (!plan) return null;
  if (plan.stale) return "Sumber rencana teknologi berubah; revisi bagian yang terdampak dulu";
  if ((plan.analysis?.issues ?? []).some((i) => i.level === "error")) return "Selesaikan masalah rencana teknologi dulu";
  return null;
}

export function allFindings(plan: TechPlan): TechFinding[] {
  const rank = { error: 0, warning: 1, info: 2 } as const;
  return [...(plan.evaluation?.findings ?? []), ...(plan.analysis?.findings ?? [])].sort((a, b) => rank[a.severity] - rank[b.severity]);
}

export function allCompat(plan: TechPlan): TechCompat[] {
  return [...(plan.compatibility ?? []), ...(plan.analysis?.compat ?? [])];
}

export interface PlanNote { level: "error" | "warning"; area?: string; text: string }

// planNotes is what needs the reader's attention, errors first. Informational
// findings stay in the full document.
export function planNotes(plan: TechPlan): PlanNote[] {
  const notes: PlanNote[] = (plan.analysis?.issues ?? []).map((i) => ({ level: i.level, area: i.area, text: i.message }));
  for (const f of allFindings(plan)) {
    if (f.severity !== "info" && !notes.some((n) => n.text === f.message)) {
      notes.push({ level: f.severity, area: f.area, text: f.resolution ? `${f.message} Solusi: ${f.resolution}` : f.message });
    }
  }
  const diverged = plan.analysis?.diverged ?? [];
  if (diverged.length && !plan.evaluation?.outdated) {
    notes.push({ level: "warning", text: `Evaluasi belum mencerminkan pilihan user: ${diverged.map(areaLabel).join(", ")}.` });
  }
  return notes.sort((a, b) => Number(b.level === "error") - Number(a.level === "error"));
}
