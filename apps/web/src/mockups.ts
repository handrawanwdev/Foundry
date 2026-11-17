import type { DesignGate, FlowWalk, LayoutSpec, MockupFrame, MockupStatus } from "./types";

// The mockup workspace arranges the designed pages as the layout spec does:
// by its navigation groups, or along one use case. Nothing here decides
// what a page is; the layout spec and the use cases do.

export interface PageGroup { name: string; pages: MockupStatus[] }

// mockupGroups groups the mockups by the navigation of the layout spec,
// each page once, then the pages no menu lists. A use case lays its pages
// out in the order it walks them.
export function mockupGroups(list: MockupStatus[], layout?: LayoutSpec | null, flow?: FlowWalk): PageGroup[] {
  const byID = new Map(list.map((m) => [m.page, m]));
  const seen = new Set<string>();
  const take = (ids: string[]) => ids.flatMap((id) => {
    const m = byID.get(id);
    if (!m || seen.has(id)) return [];
    seen.add(id);
    return [m];
  });
  if (flow) return [{ name: `${flow.id} · ${flow.name}`, pages: take(flow.pages ?? []) }];
  const groups = (layout?.navigation ?? [])
    .map((g) => ({ name: g.name, pages: take(g.items.map((it) => it.page)) }))
    .filter((g) => g.pages.length > 0);
  const rest = list.filter((m) => !seen.has(m.page));
  if (rest.length > 0) groups.push({ name: groups.length > 0 ? "Di luar menu" : "Halaman", pages: rest });
  return groups;
}

// pageFindings are the open quality-gate items that name the page.
export function pageFindings(gate: DesignGate | null | undefined, page: string): string[] {
  if (!gate) return [];
  const re = new RegExp(`halaman ${page.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}(?![\\w-])`, "i");
  return gate.checks
    .filter((c) => c.status === "fail" || c.status === "warn")
    .flatMap((c) => (c.items ?? []).filter((it) => re.test(it)));
}

// mobilePlatform says whether a page is drawn on the mobile viewport.
export function mobilePlatform(platform?: string): boolean {
  return platform === "android" || platform === "ios";
}

export interface FrameGroup { name: string; frames: MockupFrame[] }

// frameGroups keeps a page's frames in the order the renderer gives them,
// one group per action (Tambah, Ubah, Hapus, ...) and the states last.
export function frameGroups(frames: MockupFrame[] | undefined): FrameGroup[] {
  const out: FrameGroup[] = [];
  for (const f of frames ?? []) {
    const g = out.find((x) => x.name === f.group);
    if (g) g.frames.push(f);
    else out.push({ name: f.group, frames: [f] });
  }
  return out;
}

// withQuery appends the non-empty query parts to src.
export function withQuery(src: string, ...parts: (string | undefined | false)[]): string {
  const extra = parts.filter(Boolean).join("&");
  return extra ? `${src}${src.includes("?") ? "&" : "?"}${extra}` : src;
}
