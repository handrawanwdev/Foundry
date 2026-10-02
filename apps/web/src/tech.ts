import type { Project, TechArea, TechChange, TechDecision, TechStack, TechStatus } from "./types";

export const TECH_AREA: Record<TechArea, string> = {
  frontend: "Frontend", mobile: "Mobile", desktop: "Desktop", backend: "Backend", database: "Database", cache: "Cache",
  messaging: "Messaging", workers: "Worker", storage: "File storage", search: "Search", architecture: "Arsitektur",
  repository: "Repository", deployment: "Deployment", observability: "Observability",
};

export const TECH_STATUS: Record<TechStatus, string> = {
  recommended: "Rekomendasi", review: "Menunggu keputusan", impact_checked: "Diubah, dampak sudah dicek",
  approved: "Disetujui", changed: "Berubah setelah disetujui",
};

export const NONE = "Tidak perlu";

export const techChoice = (d: TechDecision) => d.override?.value ?? d.recommended;
export const techLang = (d: TechDecision) => (d.override ? d.override.language : d.language);

// Status of the decision; stacks saved before the lifecycle derive it.
export function techState(s: TechStack): TechStatus {
  if (s.status) return s.status;
  if (s.approved) return "approved";
  return s.decisions.some((d) => d.override) ? "impact_checked" : "recommended";
}

export const techVersion = (s: TechStack) => s.history?.at(-1)?.version ?? 0;

// Areas whose change is reviewed against the design (core.AffectsDesign).
const DESIGN_AREAS = new Set<TechArea>(["frontend", "mobile", "desktop", "backend", "database"]);

// Technology changes made after the draft design was last checked that
// affect it (core.Project.TechReview).
export function techReview(p: Project): TechChange[] {
  const d = p.design, s = p.stack;
  if (!d || !d.pages.length || d.status === "approved" || !s) return [];
  const since = d.tech_version ?? 0;
  return (s.history ?? []).filter((v) => v.version > since && v.design)
    .flatMap((v) => (v.changes ?? []).filter((c) => DESIGN_AREAS.has(c.area)));
}

// Why the design cannot be approved yet because of the technology, or null.
export function techGate(p: Project): string | null {
  const s = p.stack;
  if (!s) return "Keputusan teknologi belum ada";
  if (s.mode === "existing") return null;
  if (techState(s) !== "approved") return "Setujui keputusan teknologi dulu";
  if (techReview(p).length) return "Teknologi berubah setelah rancangan dibuat; periksa dampaknya dulu";
  return null;
}

export const changeText = (c: TechChange) => `${TECH_AREA[c.area] ?? c.area}: ${c.from || "—"} → ${c.to || "—"}`;
