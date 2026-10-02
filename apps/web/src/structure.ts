import type { Project, Structure, StructureIR, TechStatus } from "./types";

export const STRUCTURE_TYPE: Record<string, string> = {
  simple: "Simple", modular: "Modular", monorepo: "Monorepo", microservices: "Microservices",
};

// Status of the structure decision; structures saved before the lifecycle
// derive it (core.Structure.State).
export function structureState(s: Structure): TechStatus {
  return s.status ?? (s.approved ? "approved" : "recommended");
}

export const structureVersion = (s: Structure) => s.history?.at(-1)?.version ?? 0;

// Every folder of the IR in scaffold order (structure.Layout).
export function layoutOf(ir: StructureIR): string[] {
  const out: string[] = [];
  for (const p of ir.apps ?? []) out.push(p.path);
  for (const p of ir.services ?? []) {
    out.push(p.path);
    for (const m of p.modules ?? []) out.push(`${p.path}${m}/`);
  }
  for (const p of [...(ir.workers ?? []), ...(ir.packages ?? [])]) out.push(p.path);
  out.push(ir.docs || "docs/");
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

// Why the design cannot be approved yet because of the structure, or null.
export function structureGate(p: Project): string | null {
  const s = p.structure;
  if (!s || s.mode === "existing" || !s.ir) return null;
  if (s.proposal) return "Terapkan atau batalkan perubahan struktur dulu";
  if (structureState(s) !== "approved") return "Setujui struktur project dulu";
  if ((s.issues ?? []).some((i) => i.level === "error")) return "Selesaikan masalah struktur dulu";
  return null;
}
