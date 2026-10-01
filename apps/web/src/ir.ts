import type { Device } from "./design";
import type { DesignSystem, DeviceLayout, LayoutChild, LayoutNode, Page, PageLayout, ThemeContext } from "./types";

// Design IR on the client: the same vocabulary, validation and per-device
// resolution as internal/core/designir.go. The renderer draws only what
// resolveLayout returns; it never picks a layout itself.

export const PRIMITIVES = ["stack", "row", "grid", "split", "sidebar", "center", "full", "overlay"];
export const FRAMES = ["app", "center", "full"];
export const NAVS = ["side", "rail", "top", "bottom", "none"];
export const WIDTHS = ["narrow", "medium", "wide", "full"];
export const DEVICE_KEYS = ["desktop", "tablet", "mobile"];
export const DENSITIES = ["compact", "comfortable", "spacious"];
export const SPACE = ["space.none", "space.xs", "space.sm", "space.md", "space.lg", "space.xl"];
export const SURFACES = ["surface.primary", "surface.secondary"];
export const ELEVATIONS = ["elevation.none", "elevation.raised", "elevation.overlay"];
export const EMPHASES = ["action.primary", "action.secondary", "action.ghost", "action.danger"];
export const TONES = ["text.primary", "text.secondary", "status.success", "status.warning", "status.error", "status.info"];
export const ALIGNS = ["start", "center", "end", "between", "stretch"];
export const RATIOS = ["1:1", "1:2", "2:1", "1:3", "3:1"];
export const SIDES = ["start", "end"];
export const PLACEMENTS = ["drawer", "modal", "sheet"];
export const SWAPS: Record<string, string> = { table: "list", list: "table" };

export const childKey = (c: LayoutChild): string => (typeof c === "string" ? c : c.id ?? "");

export interface ResolvedLayout {
  frame: string; nav: string; width: string; brand: boolean; back?: string; root: LayoutNode; swap: Record<string, string>;
}

// The layout as drawn on one device: overrides applied, hidden parts dropped.
export function resolveLayout(l: PageLayout, device: Device): ResolvedLayout {
  const d: DeviceLayout = l.devices?.[device] ?? {};
  return {
    frame: d.frame || l.frame, nav: d.nav || l.nav || "none", width: d.width || l.width || "full", brand: !!l.brand, back: l.back,
    root: resolveNode(l.root, d), swap: d.swap ?? {},
  };
}

function resolveNode(n: LayoutNode, d: DeviceLayout): LayoutNode {
  const ch = (n.id && d.nodes?.[n.id]) || {};
  let kids = n.children ?? [];
  if (ch.order?.length) {
    const order = ch.order;
    kids = [...order.map((k) => kids.find((c) => childKey(c) === k)).filter((c): c is LayoutChild => c !== undefined),
      ...kids.filter((c) => !order.includes(childKey(c)))];
  }
  const out: LayoutNode = { ...n, type: ch.type || n.type, children: [] };
  if (ch.columns) out.columns = ch.columns;
  for (const k of kids) {
    if (d.hide?.includes(childKey(k))) continue;
    out.children!.push(typeof k === "string" ? k : resolveNode(k, d));
  }
  return out;
}

function walk(n: LayoutChild, fn: (n: LayoutChild, parent?: LayoutNode) => void, parent?: LayoutNode) {
  fn(n, parent);
  if (typeof n !== "string") for (const k of n.children ?? []) walk(k, fn, n);
}

// Component ids in drawing order.
export function leaves(n: LayoutNode): string[] {
  const out: string[] = [];
  walk(n, (k) => { if (typeof k === "string") out.push(k); });
  return out;
}

// What is wrong with a page's IR; an invalid page is not rendered.
export function validateIR(page: Page, pageIds: Set<string>): string[] {
  const out: string[] = [];
  const oneOf = (what: string, v: string | undefined, set: string[]) => {
    if (v && !set.includes(v)) out.push(`${what} "${v}" tidak dikenal (pakai: ${set.join(", ")})`);
  };
  oneOf("density", page.density, DENSITIES);
  const comps = new Map<string, string>();
  let primary = 0;
  (page.components ?? []).forEach((c, i) => {
    if (!c.id) out.push(`komponen [${i}] ${c.type} tanpa id`);
    else if (comps.has(c.id)) out.push(`id komponen "${c.id}" dipakai lebih dari sekali`);
    else comps.set(c.id, c.type);
    oneOf(`komponen "${c.id}" emphasis`, c.emphasis, EMPHASES);
    oneOf(`komponen "${c.id}" tone`, c.tone, TONES);
    if (c.emphasis === "action.primary") primary++;
  });
  if (primary > 1) out.push(`${primary} komponen memakai action.primary; maksimal satu aksi utama per halaman`);

  const l = page.layout;
  if (!l) return [...out, "layout belum ditentukan"];
  if (!l.frame) out.push(`frame kosong (pakai: ${FRAMES.join(", ")})`);
  oneOf("frame", l.frame, FRAMES);
  oneOf("nav", l.nav, NAVS);
  oneOf("width", l.width, WIDTHS);
  if (l.back && (!pageIds.has(l.back) || l.back === page.id)) out.push(`back menuju halaman "${l.back}" yang tidak ada`);
  if (!l.root || typeof l.root === "string" || !l.root.type) return [...out, "root harus node layout, bukan komponen"];

  const nodes = new Map<string, LayoutNode>();
  const placed = new Map<string, number>();
  walk(l.root, (n, parent) => {
    if (typeof n === "string") {
      const type = comps.get(n);
      if (type === undefined) out.push(`layout menempatkan komponen "${n}" yang tidak ada`);
      else if (type === "nav") out.push(`komponen nav "${n}" tidak boleh di layout; posisinya diatur "nav"`);
      placed.set(n, (placed.get(n) ?? 0) + 1);
      return;
    }
    const name = n.id || n.type;
    if (!PRIMITIVES.includes(n.type)) out.push(`node "${name}": primitive "${n.type}" tidak didukung (pakai: ${PRIMITIVES.join(", ")})`);
    if (n.id) {
      if (nodes.has(n.id) || comps.has(n.id)) out.push(`id node "${n.id}" dipakai lebih dari sekali`);
      nodes.set(n.id, n);
    }
    if (!n.children?.length && parent) out.push(`node "${name}" tidak punya isi`);
    if (n.type === "grid" && !(n.columns && n.columns >= 1 && n.columns <= 4)) out.push(`node "${name}": grid butuh columns 1-4`);
    if (n.type !== "grid" && n.columns) out.push(`node "${name}": columns hanya untuk grid`);
    oneOf(`node "${name}" ratio`, n.ratio, RATIOS);
    oneOf(`node "${name}" side`, n.side, SIDES);
    oneOf(`node "${name}" placement`, n.placement, PLACEMENTS);
    oneOf(`node "${name}" gap`, n.gap, SPACE);
    oneOf(`node "${name}" align`, n.align, ALIGNS);
    oneOf(`node "${name}" surface`, n.surface, SURFACES);
    oneOf(`node "${name}" elevation`, n.elevation, ELEVATIONS);
  });
  let navs = 0;
  for (const c of page.components ?? []) {
    if (c.type === "nav") { navs++; continue; }
    const n = c.id ? placed.get(c.id) ?? 0 : -1;
    if (n === 0) out.push(`komponen "${c.id}" tidak ditempatkan di layout (orphan)`);
    if (n > 1) out.push(`komponen "${c.id}" ditempatkan ${n} kali di layout`);
  }
  const nested = (n: LayoutChild, inSurface: boolean) => {
    if (typeof n === "string") return;
    if (n.surface && inSurface) out.push(`node "${n.id ?? ""}" ber-surface di dalam node ber-surface (kartu di dalam kartu)`);
    for (const k of n.children ?? []) nested(k, inSurface || !!n.surface);
  };
  nested(l.root, false);

  for (const [dev, d] of Object.entries(l.devices ?? {})) {
    if (!DEVICE_KEYS.includes(dev)) { out.push(`perangkat "${dev}" tidak dikenal (pakai: ${DEVICE_KEYS.join(", ")})`); continue; }
    oneOf(dev + " frame", d.frame, FRAMES);
    oneOf(dev + " nav", d.nav, NAVS);
    oneOf(dev + " width", d.width, WIDTHS);
    for (const h of d.hide ?? []) if (!nodes.has(h) && !comps.has(h)) out.push(`${dev}: hide "${h}" tidak ada`);
    for (const [id, ch] of Object.entries(d.nodes ?? {})) {
      const n = nodes.get(id);
      if (!n) { out.push(`${dev}: node "${id}" tidak ada`); continue; }
      oneOf(`${dev} node "${id}" type`, ch.type, PRIMITIVES);
      if (ch.columns !== undefined && (ch.columns < 0 || ch.columns > 4)) out.push(`${dev} node "${id}": columns 1-4`);
      if (ch.order?.length) {
        const keys = (n.children ?? []).map(childKey);
        if ([...ch.order].sort().join("\n") !== [...keys].sort().join("\n") || keys.includes(""))
          out.push(`${dev} node "${id}": order harus berisi tepat anak node: ${keys.join(", ")}`);
      }
    }
    for (const [id, to] of Object.entries(d.swap ?? {})) {
      const type = comps.get(id);
      if (type === undefined) out.push(`${dev}: swap komponen "${id}" tidak ada`);
      else if (SWAPS[type] !== to) out.push(`${dev}: swap ${type} "${id}" ke "${to}" tidak didukung (hanya table ↔ list)`);
    }
  }
  const mobileRules = page.platform !== "desktop";
  for (const dev of DEVICE_KEYS as Device[]) {
    const r = resolveLayout(l, dev);
    if (r.nav !== "none" && navs === 0 && r.frame === "app") out.push(`${dev}: nav "${r.nav}" tanpa komponen nav`);
    walk(r.root, (n) => {
      if (typeof n === "string") return;
      const k = n.children ?? [];
      if ((n.type === "split" || n.type === "sidebar") && k.length !== 2) out.push(`${dev}: node "${n.id ?? ""}" (${n.type}) butuh tepat 2 anak, ada ${k.length}`);
      if (n.type === "grid" && !n.columns) out.push(`${dev}: node "${n.id ?? ""}" grid butuh columns`);
      if (dev === "mobile" && mobileRules) {
        if (n.type === "split" || n.type === "sidebar") out.push(`mobile: node "${n.id ?? ""}" masih ${n.type}; ubah lewat devices.mobile.nodes.${n.id ?? ""}.type`);
        else if (n.type === "grid" && (n.columns ?? 0) > 2) out.push(`mobile: node "${n.id ?? ""}" grid ${n.columns} kolom; maksimal 2 lewat devices.mobile.nodes.${n.id ?? ""}.columns`);
      }
    });
    if (dev === "mobile" && mobileRules && r.frame === "app" && (r.nav === "side" || r.nav === "rail"))
      out.push(`mobile: nav "${r.nav}" tidak muat; pakai devices.mobile.nav bottom, top atau none`);
  }
  return [...new Set(out)];
}

// ---------- Tokens ----------

const COLOR_VARS: Record<string, string> = {
  "text.primary": "--wf-text", "text.secondary": "--wf-muted", "surface.page": "--wf-bg", "surface.primary": "--wf-surface",
  "surface.secondary": "--wf-surface-2", "border.default": "--wf-line", "action.primary": "--wf-primary", "action.danger": "--wf-danger",
  "status.success": "--wf-ok", "status.warning": "--wf-warn", "status.error": "--wf-bad", "status.info": "--wf-info",
};
const LENGTH_VARS: Record<string, string> = {
  "space.xs": "--wf-space-xs", "space.sm": "--wf-space-sm", "space.md": "--wf-space-md", "space.lg": "--wf-space-lg", "space.xl": "--wf-space-xl",
  "radius.sm": "--wf-radius-sm", "radius.md": "--wf-radius-md", "radius.lg": "--wf-radius-lg",
};
const TYPE_VARS: Record<string, string> = {
  "type.display": "--wf-fs-display", "type.heading": "--wf-fs-heading", "type.body": "--wf-fs-body", "type.caption": "--wf-fs-caption",
};
const SHADOW_VARS: Record<string, string> = { "elevation.raised": "--wf-elev-raised", "elevation.overlay": "--wf-elev-overlay" };

const COLOR = /^(#[0-9a-f]{3,8}|(rgb|rgba|hsl|hsla)\([\d\s.,%/]+\))$/i;
const LENGTH = /^\d+(\.\d+)?(px|rem)$/;
const SHADOW = /^(none|[\d\s.,#()a-z%-]+)$/i;

// CSS variables for the semantic tokens the design system defines. Only
// well-formed values of known token names pass; everything else keeps the
// wireframe default, so a design system can never inject arbitrary CSS.
export function tokenVars(system?: DesignSystem): Record<string, string> {
  const out: Record<string, string> = {};
  if (!system) return out;
  for (const t of system.colors ?? []) if (COLOR_VARS[t.name] && COLOR.test(t.value.trim())) out[COLOR_VARS[t.name]] = t.value.trim();
  for (const t of [...(system.spacing ?? []), ...(system.radius ?? [])])
    if (LENGTH_VARS[t.name] && LENGTH.test(t.value.trim())) out[LENGTH_VARS[t.name]] = t.value.trim();
  for (const t of system.typography ?? []) {
    const size = TYPE_VARS[t.name] && /(\d+(\.\d+)?)px/.exec(t.value)?.[1];
    if (size) out[TYPE_VARS[t.name]] = size + "px";
  }
  for (const t of system.shadow ?? []) if (SHADOW_VARS[t.name] && SHADOW.test(t.value.trim()) && !/url|expression|;/i.test(t.value)) out[SHADOW_VARS[t.name]] = t.value.trim();
  return out;
}

// Component styles a theme may set; the renderer turns them into classes.
export const THEME_STYLES: Record<string, string[]> = {
  button: ["solid", "outline", "soft"], card: ["bordered", "flat", "elevated"], nav: ["dark", "light", "accent"],
  table: ["lined", "striped", "plain"], font: ["sans", "mono"],
};

// Classes of the theme on the screen: color mode and component style. Only
// known values pass; no theme (older designs) draws the defaults.
export function themeClass(theme?: ThemeContext): string {
  if (!theme) return "";
  const out = [theme.color_mode === "dark" ? "th-dark" : "th-light"];
  for (const [k, v] of Object.entries(theme.style ?? {})) if (THEME_STYLES[k]?.includes(v)) out.push(`th-${k}-${v}`);
  return out.join(" ");
}

const SPACE_VAR = (t?: string) => (t === "space.none" ? "0" : t ? `var(--wf-${t.replace(".", "-")})` : undefined);
const JUSTIFY: Record<string, string> = { start: "flex-start", center: "center", end: "flex-end", between: "space-between", stretch: "stretch" };

// Inline style of a layout node, computed only from its primitive and tokens.
export function nodeStyle(n: LayoutNode): Record<string, string | undefined> {
  const s: Record<string, string | undefined> = { gap: SPACE_VAR(n.gap) };
  if (n.type === "grid") s.gridTemplateColumns = `repeat(${n.columns ?? 1}, minmax(0, 1fr))`;
  if (n.type === "split") {
    const [a, b] = (n.ratio || "1:1").split(":");
    s.gridTemplateColumns = `minmax(0, ${a}fr) minmax(0, ${b}fr)`;
  }
  if (n.type === "sidebar") s.gridTemplateColumns = n.side === "end" ? "minmax(0, 1fr) var(--wf-side)" : "var(--wf-side) minmax(0, 1fr)";
  if (n.align) {
    if (n.type === "row") s.justifyContent = JUSTIFY[n.align];
    else s.alignItems = JUSTIFY[n.align];
  }
  return s;
}

// Where a component or node sits in the tree: its parent's id ("" = root)
// and position. Null when it is not in the tree or its parent has no id
// (a canvas edit names the parent by id).
export function locate(root: LayoutNode, key: string): { parent: string; index: number; count: number } | null {
  let out: { parent: string; index: number; count: number } | null = null;
  walk(root, (n, parent) => {
    if (out || !parent || childKey(n) !== key) return;
    if (parent !== root && !parent.id) return;
    const kids = parent.children ?? [];
    out = { parent: parent === root ? "" : parent.id!, index: kids.findIndex((k) => childKey(k) === key), count: kids.length };
  });
  return out;
}
