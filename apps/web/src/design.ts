import { PLATFORM_LABEL } from "./flow";
import type { Flow, Page, PageComponent, Project } from "./types";

// Pure helpers for the design workspace.

export type Device = "desktop" | "tablet" | "mobile";

export const DEVICES: { key: Device; label: string }[] = [
  { key: "desktop", label: "Desktop" },
  { key: "tablet", label: "Tablet" },
  { key: "mobile", label: "Mobile" },
];

export function defaultDevice(page?: Page): Device {
  return page && (page.platform === "android" || page.platform === "ios") ? "mobile" : "desktop";
}

export const COMPONENT_LABEL: Record<string, string> = {
  header: "Judul", nav: "Navigasi", text: "Teks", list: "Daftar", table: "Tabel",
  form: "Form", button: "Tombol", card: "Kartu", stat: "Angka", input: "Isian",
};

export function componentName(c: PageComponent): string {
  return `${COMPONENT_LABEL[c.type] ?? c.type} “${c.label}”`;
}

export interface PageGroup {
  role: string;
  loose: Page[]; // pages outside any flow, listed first
  flows: { flow: Flow; pages: Page[] }[];
}

export function roleOf(p: Page): string {
  return p.role?.trim() || PLATFORM_LABEL[p.platform] || p.platform;
}

// Groups pages by role, then by user flow. Each page appears once: under the
// first flow that walks through it, otherwise loose in its role.
export function groupPages(pages: Page[], flows: Flow[]): PageGroup[] {
  const byId = new Map(pages.map((p) => [p.id, p]));
  const groups = new Map<string, PageGroup>();
  const group = (role: string) => {
    let g = groups.get(role);
    if (!g) groups.set(role, (g = { role, loose: [], flows: [] }));
    return g;
  };
  for (const p of pages) group(roleOf(p));
  const placed = new Set<string>();
  for (const flow of flows) {
    const ps = (flow.page_ids ?? []).map((id) => byId.get(id)).filter((p): p is Page => !!p && !placed.has(p.id));
    if (ps.length === 0) continue;
    ps.forEach((p) => placed.add(p.id));
    group(roleOf(ps[0])).flows.push({ flow, pages: ps });
  }
  for (const p of pages) if (!placed.has(p.id)) group(roleOf(p)).loose.push(p);
  return [...groups.values()];
}

// Sidebar order, used by previous/next.
export function pageOrder(groups: PageGroup[]): Page[] {
  return groups.flatMap((g) => [...g.loose, ...g.flows.flatMap((f) => f.pages)]);
}

export function flowPages(flow: Flow, pages: Page[]): Page[] {
  return (flow.page_ids ?? []).map((id) => pages.find((p) => p.id === id)).filter((p): p is Page => !!p);
}

// A nav item opens the page of the same platform with the same name.
export function navTarget(item: string, from: Page, pages: Page[]): Page | undefined {
  const name = item.trim().toLowerCase();
  return pages.find((p) => p.platform === from.platform && p.name.trim().toLowerCase() === name);
}

// Pages that link to `to` through a target or a nav item.
export function incoming(to: Page, pages: Page[]): Page[] {
  return pages.filter((p) => p.id !== to.id && (p.components ?? []).some((c) =>
    c.target === to.id || (c.type === "nav" && (c.items ?? []).some((it) => navTarget(it, p, pages)?.id === to.id))));
}

// UI features (not backend-only) that have no page yet.
export function featuresWithoutPage(project: Project) {
  const pages = project.design?.pages ?? [];
  return (project.features ?? []).filter((f) =>
    f.platforms.some((pl) => pl !== "backend") && !pages.some((p) => p.feature_id === f.id));
}

// Requirement ids behind the given pages (page → feature → requirements).
export function requirementsOf(pageIds: string[], project: Project): string[] {
  const out = new Set<string>();
  for (const id of pageIds) {
    const page = project.design?.pages.find((p) => p.id === id);
    const f = project.features?.find((x) => x.id === page?.feature_id);
    f?.requirement_ids.forEach((r) => out.add(r));
  }
  return [...out];
}
