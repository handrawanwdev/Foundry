import type { DecisionState, Project, TechArea, TechChange, TechDecision, TechSource, TechStack, TechStatus } from "./types";

export const TECH_AREA: Record<TechArea, string> = {
  frontend: "Frontend", ui: "UI library", mobile: "Mobile", desktop: "Desktop", cli: "CLI", backend: "Backend",
  api: "API protocol", database: "Database", cache: "Cache", messaging: "Messaging", workers: "Worker",
  storage: "File storage", search: "Search", architecture: "Arsitektur aplikasi", repository: "Struktur repository",
  deployment: "Deployment", observability: "Observability", testing: "Testing", quality: "Lint & format",
};

export const DECISION_STATE: Record<DecisionState, string> = {
  recommended: "Rekomendasi AI", existing: "Sudah dipakai", selected: "Dipilih user", approved: "Disetujui",
  needs_review: "Perlu ditinjau", conflict: "Konflik",
};

export const TECH_SOURCE: Record<TechSource, string> = {
  existing: "kode yang ada", preference: "preferensi stack kamu", need: "kebutuhan teknis", profile: "kebijakan profil",
  default: "default", dependency: "mengikuti keputusan lain",
};

export const TECH_STATUS: Record<TechStatus, string> = {
  recommended: "Rekomendasi", review: "Menunggu keputusan", impact_checked: "Diubah, dampak sudah dicek",
  approved: "Disetujui", changed: "Berubah setelah disetujui",
};

export const NONE = "Tidak perlu";

export const techChoice = (d: TechDecision) => d.override?.value ?? d.recommended;
export const techLang = (d: TechDecision) => (d.override ? d.override.language : d.language);
export const techRuntime = (d: TechDecision) => (d.override ? d.override.runtime : d.runtime);

// Read from the project's code (core: Source existing, or no source in an
// existing-mode stack saved before sources).
export const fromExisting = (s: TechStack, d: TechDecision) => d.source === "existing" || (!d.source && s.mode === "existing");

// Choices of the last approved version.
function approvedChoices(s: TechStack): Map<TechArea, string> {
  const out = new Map<TechArea, string>();
  const hist = s.history ?? [];
  if (techState(s) === "approved" && hist.length === 0) {
    s.decisions.forEach((d) => out.set(d.area, techChoice(d)));
    return out;
  }
  const v = [...hist].reverse().find((h) => h.status === "approved");
  v?.decisions.forEach((d) => out.set(d.area, techChoice(d)));
  return out;
}

// State of one decision (mirrors core.TechStack.DecisionState): a
// recommendation is never an approval.
export function decisionState(s: TechStack, d: TechDecision): DecisionState {
  if ((s.issues ?? []).some((i) => i.level === "error" && i.area === d.area)) return "conflict";
  if (d.review) return "needs_review";
  if (approvedChoices(s).get(d.area) === techChoice(d)) return "approved";
  if (d.override) return "selected";
  if (fromExisting(s, d)) return "existing";
  return "recommended";
}

// The user approves explicitly unless the stack of an existing project is
// only what its code already uses (stack.NeedsApproval).
export function needsApproval(s: TechStack): boolean {
  if (s.mode !== "existing" || s.proposal || (s.history ?? []).length > 0) return true;
  return s.decisions.some((d) => d.override || d.review || !fromExisting(s, d));
}

// Status of the decision; stacks saved before the lifecycle derive it.
export function techState(s: TechStack): TechStatus {
  if (s.status) return s.status;
  if (s.approved) return "approved";
  return s.decisions.some((d) => d.override) ? "impact_checked" : "recommended";
}

export const techVersion = (s: TechStack) => s.history?.at(-1)?.version ?? 0;

// Areas whose change is reviewed against the UI design (core.AffectsDesign).
const DESIGN_AREAS = new Set<TechArea>(["frontend", "ui", "mobile", "desktop", "backend", "database"]);

// Technology changes made after the UI design was last checked that
// affect it (core.Project.TechReview).
export function techReview(p: Project): TechChange[] {
  const d = p.ui, s = p.stack;
  if (!d || !(d.rules || d.layout?.pages?.length) || p.stage === "READY" || !s) return [];
  const since = d.tech_version ?? 0;
  return (s.history ?? []).filter((v) => v.version > since && v.design)
    .flatMap((v) => (v.changes ?? []).filter((c) => DESIGN_AREAS.has(c.area)));
}

// Why the project cannot be finalized yet because of the technology, or null.
export function techGate(p: Project): string | null {
  const s = p.stack;
  if (!s) return "Keputusan teknologi belum ada";
  if (!needsApproval(s)) return null;
  if (techState(s) !== "approved") return "Setujui keputusan teknologi dulu";
  if (techReview(p).length) return "Teknologi berubah setelah rancangan UI dibuat; periksa dampaknya dulu";
  return null;
}

export const changeText = (c: TechChange) => `${TECH_AREA[c.area] ?? c.area}: ${c.from || "—"} → ${c.to || "—"}`;
