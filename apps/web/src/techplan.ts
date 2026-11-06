import type { Project, TechCheck, TechCompat, TechFinding, TechItem, TechPlan } from "./types";

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

export const PLAN_CRITERIA: Record<string, string> = {
  requirement_coverage: "Requirement coverage", compatibility: "Compatibility", maintainability: "Maintainability",
  performance: "Performance", scalability: "Scalability", security: "Security", reliability: "Reliability",
  developer_experience: "Developer experience", operational_complexity: "Operational complexity (rendah lebih baik)",
  total_cost: "Total cost of ownership (rendah lebih baik)", sustainability: "Long-term sustainability",
};

export const PLAN_FINDING: Record<string, string> = {
  "redundant-dependency": "Dependensi redundan", "unnecessary-technology": "Teknologi tidak perlu",
  incompatibility: "Inkompatibilitas", "over-engineering": "Over-engineering", "operational-risk": "Risiko operasional",
  "single-point-of-failure": "Single point of failure", "vendor-lock-in": "Vendor lock-in", unverified: "Belum terverifikasi",
  "missing-decision": "Keputusan hilang", other: "Lainnya",
};

export const PLAN_CHECK: Record<string, string> = {
  requirements: "Requirements", project_modules: "Project Modules", architecture: "Arsitektur",
  folder_structure: "Struktur folder", data_architecture: "Arsitektur data", security: "Keamanan", deployment: "Deployment",
};

export const PLAN_DRIVER: Record<string, string> = {
  requirement: "kebutuhan", architecture: "arsitektur", compatibility: "kompatibilitas",
  engineering: "engineering", operational: "operasional", profile: "profil",
};

export const NONE = "Tidak perlu";

export type PlanTab = keyof typeof PLAN_SECTION;
export const PLAN_TABS: PlanTab[] = [
  "context", "overview", "stack", "data", "infra", "architecture", "standards",
  "compatibility", "alternatives", "reasoning", "evaluation", "history",
];

const TAB_GROUPS: Record<string, string[]> = {
  stack: ["stack"], data: ["data"], infra: ["app_infra", "operations"], architecture: ["architecture"], standards: ["standards"],
};

export function areaLabel(area: string): string {
  return area.replace(/_/g, " ");
}

export function isNone(it: TechItem): boolean {
  return !it.technology || it.technology === NONE;
}

export function itemsOf(plan: TechPlan, tab: PlanTab): TechItem[] {
  const groups = TAB_GROUPS[tab];
  if (!groups) return plan.items;
  return plan.items.filter((it) => groups.includes(it.group ?? ""));
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

export function allChecks(plan: TechPlan): TechCheck[] {
  const seen = new Set<string>();
  return [...(plan.analysis?.checks ?? []), ...(plan.consistency ?? [])].filter((c) => {
    const key = `${c.area}|${c.source ?? ""}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

export function allCompat(plan: TechPlan): TechCompat[] {
  return [...(plan.compatibility ?? []), ...(plan.analysis?.compat ?? [])];
}

export function planSummary(plan: TechPlan): { items: number; outside: number; unverified: number; score: number } {
  return {
    items: plan.analysis?.items ?? plan.items.length,
    outside: plan.analysis?.outside_catalog ?? 0,
    unverified: plan.analysis?.unverified ?? 0,
    score: plan.evaluation?.score ?? 0,
  };
}
