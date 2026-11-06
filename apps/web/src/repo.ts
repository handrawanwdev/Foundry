import type { Project, RepoEdge, RepoNode, RepoPlan, RepoUnit, TechStatus } from "./types";

export const SOURCE_AREA: Record<string, string> = {
  requirements: "Requirement & fitur", modules: "Project Modules", architecture: "Arsitektur", technology: "Teknologi",
  data: "Data", integrations: "Integrasi", security: "Keamanan", operations: "Operasional & deployment", structure: "Keputusan struktur",
};

// Plan sections (stale-area labels; the workspace is a single page).
export const SECTION: Record<string, string> = {
  overview: "Ringkasan", tree: "pohon folder", modules: "pemetaan modul", dependency: "dependency", rules: "aturan",
  evaluation: "evaluasi", decisions: "keputusan", history: "riwayat",
};

export const CRITERIA: Record<string, string> = {
  cohesion: "High cohesion", coupling: "Low coupling", ownership: "Clear ownership", maintainability: "Maintainability",
  discoverability: "Discoverability", testability: "Testability", extensibility: "Extensibility", simplicity: "Simplicity",
  complexity: "Complexity (rendah lebih baik)",
};

export const FINDING: Record<string, string> = {
  "unnecessary-directory": "Direktori tidak perlu", "unnecessary-abstraction": "Abstraksi tidak perlu",
  "responsibility-overlap": "Tanggung jawab tumpang tindih", "circular-dependency": "Dependensi melingkar",
  "excessive-coupling": "Coupling berlebihan", "poor-naming": "Penamaan buruk", duplication: "Duplikasi",
  "over-engineering": "Over-engineering", other: "Lainnya",
};

export const CHECK_AREA: Record<string, string> = {
  architecture_identity: "Identitas arsitektur", project_modules: "Project Modules", folder_responsibility: "Responsibility folder",
  technology: "Keputusan teknologi", dependency_rules: "Aturan dependency", architecture_rules: "Aturan arsitektur",
  structure_decision: "Keputusan struktur",
};

export const OWNER_LABEL: Record<string, string> = { shared: "shared (bersama)", platform: "platform (teknis)" };

export interface TreeNode { node: RepoNode; depth: number; children: TreeNode[] }

// The flat node list (sorted by path, parents first) as a tree.
export function buildTree(nodes: RepoNode[]): TreeNode[] {
  const roots: TreeNode[] = [];
  const at = new Map<string, TreeNode>();
  for (const node of nodes) {
    const t: TreeNode = { node, depth: 0, children: [] };
    const parent = at.get(parentOf(node.path));
    if (parent) {
      t.depth = parent.depth + 1;
      parent.children.push(t);
    } else roots.push(t);
    at.set(node.path, t);
  }
  return roots;
}

export function parentOf(path: string): string {
  const p = path.replace(/\/$/, "");
  const i = p.lastIndexOf("/");
  return i < 0 ? "" : p.slice(0, i + 1);
}

export const baseName = (path: string) => path.replace(/\/$/, "").split("/").pop() ?? path;

// The rows to draw: folders collapse unless expanded; a search shows every
// match with the folders above it.
export function visibleRows(roots: TreeNode[], expanded: Set<string>, query: string): TreeNode[] {
  const q = query.trim().toLowerCase();
  const matches = (n: RepoNode) => `${n.path} ${n.owner} ${n.responsibility} ${n.part ?? ""}`.toLowerCase().includes(q);
  const out: TreeNode[] = [];
  const keep = (t: TreeNode): boolean => !q || matches(t.node) || t.children.some(keep);
  const dirFirst = (list: TreeNode[]) => [...list].sort((a, b) => Number(a.node.type === "file") - Number(b.node.type === "file"));
  const walk = (list: TreeNode[]) => {
    for (const t of dirFirst(list)) {
      if (!keep(t)) continue;
      out.push(t);
      if (q || expanded.has(t.node.path)) walk(t.children);
    }
  };
  walk(roots);
  return out;
}

// Every ancestor folder of path, outermost first.
export function ancestors(path: string): string[] {
  const out: string[] = [];
  for (let p = parentOf(path); p; p = parentOf(p)) out.unshift(p);
  return out;
}

// Modules that place code in or under path, and what a node depends on / is used by.
export function usedBy(plan: RepoPlan, path: string): string[] {
  return plan.nodes.filter((n) => n.depends_on?.some((d) => d === path || d + "/" === path || (path.endsWith("/") && d.startsWith(path)))).map((n) => n.path);
}

export function modulesAt(plan: RepoPlan, path: string): string[] {
  return plan.modules.filter((m) => m.locations?.some((l) => l.path === path)).map((m) => m.module);
}

export function subtreeSize(plan: RepoPlan, path: string): number {
  return plan.nodes.filter((n) => n.path === path || (path.endsWith("/") && n.path.startsWith(path))).length;
}

// Units on a circle, for the dependency diagram.
export function circleLayout(units: RepoUnit[], size: number): Record<string, { x: number; y: number }> {
  const out: Record<string, { x: number; y: number }> = {};
  const c = size / 2;
  const r = size / 2 - 56;
  units.forEach((u, i) => {
    const a = (2 * Math.PI * i) / Math.max(units.length, 1) - Math.PI / 2;
    out[u.name] = { x: Math.round(c + r * Math.cos(a)), y: Math.round(c + r * Math.sin(a)) };
  });
  return out;
}

// Units worth drawing: those with code or a dependency.
export function drawnUnits(units: RepoUnit[], edges: RepoEdge[]): RepoUnit[] {
  const linked = new Set(edges.flatMap((e) => [e.from, e.to]));
  return units.filter((u) => u.nodes > 0 || linked.has(u.name));
}

export const EDGE_LABEL: Record<RepoEdge["kind"], string> = {
  derived: "dipakai (dari depends_on)", allowed: "diizinkan", forbidden: "dilarang", communication: "komunikasi",
};

export const STATUS_LABEL: Record<TechStatus, string> = {
  recommended: "Rancangan AI", review: "Menunggu keputusan", impact_checked: "Diubah", approved: "Disetujui", changed: "Berubah setelah disetujui",
};

// Why the blueprint cannot be finalized because of the repository plan, or
// null. A project without a plan is not blocked.
export function repoGate(p: Project): string | null {
  const plan = p.structure?.repo;
  if (!plan || p.structure?.mode === "existing") return null;
  if (plan.status !== "approved") return "Setujui struktur repository dulu";
  if (plan.stale) return "Struktur repository kedaluwarsa; revisi bagian yang terdampak";
  if ((plan.analysis?.issues ?? []).some((i) => i.level === "error")) return "Selesaikan masalah struktur repository dulu";
  return null;
}

export function issueCounts(plan: RepoPlan): { errors: number; warnings: number } {
  const issues = plan.analysis?.issues ?? [];
  return { errors: issues.filter((i) => i.level === "error").length, warnings: issues.filter((i) => i.level !== "error").length };
}

// Findings of the architect and of Foundry's own measurement, worst first.
export function allFindings(plan: RepoPlan) {
  const rank = { error: 0, warning: 1, info: 2 } as const;
  return [...(plan.evaluation?.findings ?? []), ...(plan.analysis?.findings ?? [])].sort((a, b) => rank[a.severity] - rank[b.severity]);
}

// The checks the AI reported and the ones Foundry verified, in one list.
export function allChecks(plan: RepoPlan) {
  const seen = new Set<string>();
  return (plan.analysis?.checks ?? plan.consistency ?? []).filter((c) => {
    const key = `${c.area}|${c.source}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

export const nodeKind = (n: RepoNode) => (n.type === "dir" ? "folder" : "file");
