import type { Feature, FeatureMap, FeatureModule, FeatureNode, Project } from "./types";

// Display map: the saved tree if present, else a one-module view of older
// blueprint features so DESIGN projects opened before Feature Discovery still
// show a list.
export function featureMapOf(p: Project | null | undefined): FeatureMap {
  if (p?.feature_map) return { summary: p.feature_map.summary, modules: p.feature_map.modules ?? [] };
  if (p?.features?.length) return fromFeatures(p.features);
  return { modules: [] };
}

export function fromFeatures(feats: Feature[]): FeatureMap {
  return {
    modules: [{
      id: "fitur",
      name: "Fitur",
      features: feats.map((f) => ({
        id: f.id,
        name: f.name,
        description: f.description,
        children: (f.sub_features ?? []).map((s) => ({ id: s.id, name: s.name })),
      })),
    }],
  };
}

export function featureCount(m: FeatureMap | null | undefined): number {
  let n = 0;
  for (const mod of m?.modules ?? []) n += countNodes(mod.features ?? []);
  return n;
}

function countNodes(nodes: FeatureNode[]): number {
  let n = 0;
  for (const node of nodes) n += 1 + countNodes(node.children ?? []);
  return n;
}

export function cloneMap(m: FeatureMap): FeatureMap {
  return JSON.parse(JSON.stringify(m ?? { modules: [] })) as FeatureMap;
}

export function toSave(m: FeatureMap): FeatureMap {
  return { summary: m.summary ?? "", modules: m.modules ?? [] };
}

export function usedIds(m: FeatureMap): Set<string> {
  const used = new Set<string>();
  for (const mod of m.modules ?? []) {
    if (mod.id) used.add(mod.id);
    walk(mod.features ?? [], (n) => { if (n.id) used.add(n.id); });
  }
  return used;
}

function walk(nodes: FeatureNode[], fn: (n: FeatureNode) => void) {
  for (const n of nodes) {
    fn(n);
    if (n.children?.length) walk(n.children, fn);
  }
}

export function slugId(s: string, prefix: string, used: Set<string>): string {
  let base = s.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 32);
  if (!base) base = prefix;
  let out = base;
  for (let n = 2; used.has(out); n++) out = `${base}-${n}`;
  used.add(out);
  return out;
}

export function addModule(m: FeatureMap, name = "Modul baru"): FeatureMap {
  const next = cloneMap(m);
  next.modules = next.modules ?? [];
  const id = slugId(name, "mod", usedIds(next));
  next.modules.push({ id, name, features: [] });
  return next;
}

export function addFeature(m: FeatureMap, moduleId: string, parentId: string | null, name = "Fitur baru"): FeatureMap {
  const next = cloneMap(m);
  const mod = next.modules.find((x) => x.id === moduleId);
  if (!mod) return next;
  const node: FeatureNode = { id: slugId(name, "feat", usedIds(next)), name, children: [] };
  if (!parentId) {
    mod.features = [...(mod.features ?? []), node];
    return next;
  }
  const parent = findNode(mod.features ?? [], parentId);
  if (parent) parent.children = [...(parent.children ?? []), node];
  return next;
}

export function findNode(nodes: FeatureNode[], id: string): FeatureNode | undefined {
  for (const n of nodes) {
    if (n.id === id) return n;
    const c = findNode(n.children ?? [], id);
    if (c) return c;
  }
}

export function findModule(m: FeatureMap, id: string): FeatureModule | undefined {
  return (m.modules ?? []).find((x) => x.id === id);
}

export function locate(m: FeatureMap, id: string): { module?: FeatureModule; node?: FeatureNode } {
  const module = findModule(m, id);
  if (module) return { module };
  for (const mod of m.modules ?? []) {
    const node = findNode(mod.features ?? [], id);
    if (node) return { module: mod, node };
  }
  return {};
}

/** Module plus ancestor chain ending at `id`. Empty `nodes` when `id` is a module. */
export function pathOf(m: FeatureMap, id: string): { module?: FeatureModule; nodes: FeatureNode[] } {
  const module = findModule(m, id);
  if (module) return { module, nodes: [] };
  for (const mod of m.modules ?? []) {
    const nodes = nodePath(mod.features ?? [], id);
    if (nodes) return { module: mod, nodes };
  }
  return { nodes: [] };
}

function nodePath(nodes: FeatureNode[], id: string): FeatureNode[] | undefined {
  for (const n of nodes) {
    if (n.id === id) return [n];
    const rest = nodePath(n.children ?? [], id);
    if (rest) return [n, ...rest];
  }
}

export function updateModule(m: FeatureMap, id: string, name: string): FeatureMap {
  const next = cloneMap(m);
  const mod = findModule(next, id);
  if (mod) mod.name = name;
  return next;
}

export function updateNode(m: FeatureMap, id: string, patch: Partial<Pick<FeatureNode, "name" | "description" | "purpose" | "benefit" | "outcome">>): FeatureMap {
  const next = cloneMap(m);
  const { node } = locate(next, id);
  if (node) Object.assign(node, patch);
  return next;
}

export function removeId(m: FeatureMap, id: string): FeatureMap {
  const next = cloneMap(m);
  next.modules = (next.modules ?? []).filter((mod) => {
    if (mod.id === id) return false;
    mod.features = removeNode(mod.features ?? [], id);
    return true;
  });
  return next;
}

function removeNode(nodes: FeatureNode[], id: string): FeatureNode[] {
  return nodes.filter((n) => n.id !== id).map((n) => ({ ...n, children: removeNode(n.children ?? [], id) }));
}
