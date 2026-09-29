import { PLATFORM_LABEL } from "./flow";
import type { Density, Flow, Page, PageComponent, PageKind, Project } from "./types";

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
  section: "Bagian", steps: "Langkah",
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

// ---------- Page kinds ----------

export const KIND_LABEL: Record<PageKind, string> = {
  auth: "Masuk / autentikasi", dashboard: "Ringkasan kerja", list: "Daftar data", detail: "Detail",
  form: "Form", wizard: "Langkah bertahap", search: "Pencarian", settings: "Pengaturan", payment: "Pembayaran",
  profile: "Profil", analytics: "Analitik", admin: "Operasional (padat)", status: "Hasil / status",
};

export const DENSITY_LABEL: Record<Density, string> = { low: "rendah", medium: "sedang", high: "tinggi" };

const NAME_KINDS: [RegExp, PageKind][] = [
  [/login|masuk|sign ?in|sign ?up|registrasi|daftar akun|lupa (kata sandi|password)/, "auth"],
  [/pengaturan|setting|konfigurasi/, "settings"],
  [/profil|profile|akun saya/, "profile"],
  [/bayar|pembayaran|payment|checkout/, "payment"],
  [/cari|pencarian|search/, "search"],
  [/laporan|analitik|analytics|statistik/, "analytics"],
  [/berhasil|sukses|gagal|error/, "status"],
  [/dashboard|beranda|home/, "dashboard"],
];

// Kind set by the designer, otherwise guessed from the name and components
// (older designs have no kind).
export function pageKind(p: Page): PageKind {
  if (p.kind) return p.kind;
  const name = p.name.toLowerCase();
  for (const [re, k] of NAME_KINDS) if (re.test(name)) return k;
  const comps = p.components ?? [];
  const has = (t: string) => comps.some((c) => c.type === t);
  if (has("steps")) return "wizard";
  if (has("table")) return /admin/i.test(p.role) ? "admin" : "list";
  if (comps.filter((c) => c.type === "stat").length >= 2) return "dashboard";
  if (has("form") || comps.filter((c) => c.type === "input").length >= 2) return "form";
  if (has("list")) return "list";
  return "detail";
}

export function pageDensity(p: Page): Density {
  if (p.density) return p.density;
  const k = pageKind(p);
  if (k === "auth" || k === "payment" || k === "status") return "low";
  if (k === "admin") return "high";
  return "medium";
}

// "Budi | Dr. Andi | Aktif" → cells.
export function cells(row: string): string[] {
  return row.split("|").map((c) => c.trim());
}

// "Janji hari ini: 12" → ["Janji hari ini", "12"].
export function statParts(label: string): [string, string | undefined] {
  const i = label.lastIndexOf(":");
  return i > 0 ? [label.slice(0, i).trim(), label.slice(i + 1).trim() || undefined] : [label, undefined];
}

// Form items; "# Kontak" starts a group.
export function formGroups(items: string[]): { title?: string; fields: string[] }[] {
  const out: { title?: string; fields: string[] }[] = [];
  for (const it of items) {
    if (it.startsWith("#")) out.push({ title: it.replace(/^#+\s*/, ""), fields: [] });
    else {
      if (!out.length) out.push({ fields: [] });
      out[out.length - 1].fields.push(it);
    }
  }
  return out;
}

// ---------- Quality check (anti one-template) ----------

export interface Finding { page?: Page; text: string }

const signature = (p: Page) => (p.components ?? []).filter((c) => c.type !== "nav").map((c) => c.type).join(",");

// Checks that can be measured on a wireframe. Visual rules (gradients,
// illustrations) do not apply: the design has no visuals yet.
export function designChecks(project: Project): Finding[] {
  const pages = project.design?.pages ?? [];
  const out: Finding[] = [];
  const seen = new Map<string, Page>();
  for (const p of pages) {
    const comps = p.components ?? [];
    const kind = pageKind(p);
    const count = (...t: string[]) => comps.filter((c) => t.includes(c.type)).length;
    const add = (text: string) => out.push({ page: p, text });

    if (!p.kind) add("jenis halaman belum ditentukan");
    if (count("stat", "card") > 4) add(`terlalu banyak kartu/angka (${count("stat", "card")})`);
    const content = comps.filter((c) => c.type !== "nav" && c.type !== "header");
    if (kind === "dashboard" && content.length > 0 && content.every((c) => c.type === "stat" || c.type === "card"))
      add("dashboard hanya berisi angka/kartu, belum menampilkan pekerjaan user");
    for (const c of comps) {
      const fields = (c.items ?? []).filter((it) => !it.startsWith("#"));
      if (c.type === "form" && fields.length > 6 && fields.length === (c.items ?? []).length)
        add(`form “${c.label}” punya ${fields.length} isian tanpa pengelompokan`);
      if (c.type === "table" && !c.columns?.length) add(`tabel “${c.label}” belum punya kolom`);
      if (c.type === "nav") {
        const dead = (c.items ?? []).filter((it) => !navTarget(it, p, pages));
        if (dead.length) add(`menu tidak menuju halaman: ${dead.join(", ")}`);
        if ((p.platform === "android" || p.platform === "ios") && (c.items?.length ?? 0) > 5)
          add(`navigasi mobile punya ${c.items!.length} item (maks. 5)`);
      }
    }
    // Detail pages list attributes of one record; only data views can be empty.
    if (["list", "admin", "dashboard", "search"].includes(kind) && count("list", "table") > 0 && !p.states?.empty)
      add("belum ada tampilan saat data kosong");
    const sig = signature(p);
    const twin = sig.split(",").length >= 3 && seen.get(p.platform + "|" + sig);
    if (twin) add(`susunan sama persis dengan halaman “${twin.name}”`);
    else seen.set(p.platform + "|" + sig, p);
  }
  return out;
}

// Change request that asks the designer to fix the findings.
export function fixRequest(findings: Finding[]): string {
  return "Perbaiki kualitas rancangan: tiap halaman pakai layout sesuai jenisnya, isi \"kind\", \"density\" dan \"states\".\n" +
    findings.map((f) => `- ${f.page ? `Halaman "${f.page.name}": ` : ""}${f.text}`).join("\n");
}
