import type { Project, Structure, StructureAlternative, StructureIR, StructurePart, StructurePlan, TechStatus } from "./types";

export const SCORE_LABEL: Record<string, string> = {
  maintainability: "Maintainability", simplicity: "Simplicity", extensibility: "Extensibility",
  testability: "Testability", separation_of_concerns: "Separation", complexity: "Complexity",
};

// The alternative the user should look at first (core.StructurePlan).
export function recommendedAlternative(plan?: StructurePlan): StructureAlternative | undefined {
  if (!plan) return undefined;
  return plan.alternatives.find((a) => a.id === plan.recommendation) ?? plan.alternatives[0];
}

export function otherAlternatives(plan?: StructurePlan): StructureAlternative[] {
  const rec = recommendedAlternative(plan);
  return (plan?.alternatives ?? []).filter((a) => a.id !== rec?.id);
}

// Status of the structure decision; structures saved before the lifecycle
// derive it (core.Structure.State).
export function structureState(s: Structure): TechStatus {
  return s.status ?? (s.approved ? "approved" : "recommended");
}

export const structureVersion = (s: Structure) => s.history?.at(-1)?.version ?? 0;

// The folder of module m inside part p, as the design chose it
// (structure.ModulePath).
export const modulePath = (p: StructurePart, m: string) => p.module_paths?.[m] || `${p.path}${m}/`;

// Every folder of the IR in scaffold order (structure.Layout).
export function layoutOf(ir: StructureIR): string[] {
  const out: string[] = [];
  for (const p of ir.apps ?? []) out.push(p.path);
  for (const p of ir.services ?? []) {
    out.push(p.path);
    for (const m of p.modules ?? []) out.push(modulePath(p, m));
  }
  for (const p of [...(ir.workers ?? []), ...(ir.packages ?? [])]) out.push(p.path);
  if (ir.docs) out.push(ir.docs);
  return out;
}

interface Node { name: string; kids: Node[] }

// The layout drawn as a folder tree (structure.Tree).
export function tree(ir: StructureIR): string {
  const root: Node = { name: "", kids: [] };
  for (const l of layoutOf(ir)) {
    let at = root;
    for (const part of l.replace(/\/$/, "").split("/")) {
      let n = at.kids.find((k) => k.name === part);
      if (!n) at.kids.push((n = { name: part, kids: [] }));
      at = n;
    }
  }
  const lines: string[] = [];
  const walk = (n: Node, prefix: string) => n.kids.forEach((k, i) => {
    const last = i === n.kids.length - 1;
    lines.push(`${prefix}${last ? "└── " : "├── "}${k.name}/`);
    walk(k, prefix + (last ? "    " : "│   "));
  });
  for (const k of root.kids) {
    lines.push(`${k.name}/`);
    walk(k, "");
  }
  return lines.join("\n");
}

// Why the project cannot be finalized yet because of the structure, or null.
export function structureGate(p: Project): string | null {
  const s = p.structure;
  if (!s || s.mode === "existing") return null;
  if (!s.ir) return "Rancang struktur dengan AI dulu";
  if (s.proposal) return "Terapkan atau batalkan perubahan struktur dulu";
  if (structureState(s) !== "approved") return "Setujui struktur project dulu";
  if ((s.issues ?? []).some((i) => i.level === "error")) return "Selesaikan masalah struktur dulu";
  return null;
}
