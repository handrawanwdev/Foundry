import { renderToStaticMarkup } from "react-dom/server";
import { Wireframe } from "./components/Wireframe";
import { defaultDevice, type Device } from "./design";
import type { DesignSystem, Page } from "./types";

// Text outline of a page exactly as the wireframe draws it, for the designer
// AI: it is read from the rendered markup, so it cannot drift from what the
// user sees. "[n] type #id" is components[n] in the design JSON; "(layout)" is
// a node of page.layout; "(otomatis)" is drawn by the frame, not by a component.

const CONTAINERS: [string, string][] = [
  ["wf-sidenav", "Sidebar kiri"], ["wf-bottom", "Tab bar bawah"], ["wf-header", "Top bar"], ["wf-topnav", "Menu atas"],
  ["wf-centerpanel", "Panel di tengah layar"], ["wf-form-foot", "Tombol bawaan form"], ["wf-backdrop", "Lapisan overlay"],
  ["wf-invalid", "DESIGN_IR_INVALID"],
];

const text = (el: Element) => (el.textContent ?? "").replace(/\s+/g, " ").trim();
const has = (el: Element, c: string) => el.classList.contains(c);

// One line for a leaf element, or null to walk its children.
function leaf(el: Element): string | null {
  const tag = el.tagName.toLowerCase();
  if (has(el, "wf-bar")) {
    const url = el.querySelector(".wf-url");
    return url ? `Bilah alamat "${text(url)}"` : "Status bar ponsel";
  }
  if (has(el, "wf-brand")) return `Brand aplikasi "${text(el).slice(1)}"`;
  if (has(el, "wf-avatar")) return "Avatar pengguna";
  if (has(el, "wf-crumb")) return `Nama halaman "${text(el)}"`;
  if (has(el, "wf-back")) return `Link kembali "${text(el)}"`;
  if (has(el, "wf-nav-item")) return `Menu "${text(el)}"${has(el, "current") ? " (aktif)" : ""}`;
  if (tag === "button") return `Tombol "${text(el)}"${has(el, "primary") ? " (utama)" : ""}`;
  if (tag === "h2") return `Judul "${text(el)}"`;
  if (tag === "h3") return `Bagian "${text(el)}"`;
  if (tag === "legend") return `Grup isian "${text(el)}"`;
  if (tag === "p") return `Teks "${text(el)}"`;
  if (has(el, "wf-label")) return `Label "${text(el)}"`;
  if (has(el, "wf-search")) return `Isian cari "${el.querySelector("input")?.getAttribute("placeholder") ?? ""}"`;
  if (has(el, "wf-field")) {
    const input = el.querySelector("input, select, textarea");
    const type = input?.tagName.toLowerCase() === "input" ? input.getAttribute("type") ?? "text" : input?.tagName.toLowerCase();
    const name = text(el.querySelector("span")!).replace(/\s*\*$/, "");
    return `Isian "${name}" (${type}${el.querySelector("em") ? ", wajib" : ""})`;
  }
  if (has(el, "wf-steps")) return "Langkah: " + [...el.querySelectorAll("li")].map(text).join(" > ");
  if (has(el, "wf-stat")) return `Angka "${[...el.children].map(text).join(": ")}"`;
  if (has(el, "wf-card")) return `Kartu "${[...el.children].map(text).join(" · ")}"`;
  if (has(el, "wf-tr") && has(el, "head")) return "Kolom: " + [...el.children].map(text).join(" | ");
  if (has(el, "wf-tr")) return "Baris: " + [...el.children].map(text).join(" | ");
  if (has(el, "wf-row")) return "Baris: " + [...el.querySelectorAll(".wf-row-main, .wf-badge")].map(text).join(" | ");
  if (has(el, "wf-sent")) return null;
  if (tag === "li" && el.closest(".wf-invalid")) return `Galat "${text(el)}"`;
  return null;
}

function walk(el: Element, depth: number, inComponent: boolean, out: string[]) {
  const id = el.getAttribute("data-id");
  const c = el.getAttribute("data-c") !== null ? `${el.getAttribute("data-c")}] ${el.getAttribute("data-type")}${id ? " #" + id : ""}` : null;
  const pad = "  ".repeat(depth);
  const node = el.getAttribute("data-node-type");
  if (node !== null) {
    const name = el.getAttribute("data-node");
    out.push(`${pad}(layout) ${node}${name ? ` "${name}"` : ""}:`);
    for (const k of el.children) walk(k, depth + 1, inComponent, out);
    return;
  }
  if (c !== null) {
    const line = leaf(el.firstElementChild ?? el);
    const kids = el.firstElementChild;
    if (line) out.push(`${pad}[${c}: ${line}`);
    else {
      out.push(`${pad}[${c}:`);
      if (kids) walk(kids, depth + 1, true, out);
    }
    return;
  }
  const line = leaf(el);
  if (line) {
    out.push(pad + (inComponent ? "" : "(otomatis) ") + line);
    return;
  }
  const box = CONTAINERS.find(([cls]) => has(el, cls));
  if (box) out.push(pad + (inComponent ? "" : "(otomatis) ") + box[1] + ":");
  // Long tables and lists: first rows are enough to see the layout.
  const rows = [...el.children].filter((k) => (has(k, "wf-tr") && !has(k, "head")) || has(k, "wf-row"));
  for (const k of el.children) {
    if (rows.indexOf(k) >= 3) continue;
    walk(k, box ? depth + 1 : depth, inComponent, out);
  }
  if (rows.length > 3) out.push(`${"  ".repeat(box ? depth + 1 : depth)}… ${rows.length - 3} baris lagi`);
}

export function screenOutline(page: Page, pages: Page[], app: string, device: Device = defaultDevice(page), system?: DesignSystem): string {
  const html = renderToStaticMarkup(<Wireframe page={page} pages={pages} device={device} mode="select" onNavigate={() => {}} app={app} system={system} />);
  const root = new DOMParser().parseFromString(html, "text/html").body;
  const out: string[] = [];
  for (const k of root.children) walk(k, 0, false, out);
  return `Tampilan ${device}:\n` + out.join("\n");
}

// Outline of every page, keyed by page id, sent with a change request.
// devices: the device the user is looking at a page on; others use the
// page platform's default.
export function screens(pages: Page[], app: string, devices: Record<string, Device> = {}): Record<string, string> {
  return Object.fromEntries(pages.map((p) => [p.id, screenOutline(p, pages, app, devices[p.id])]));
}
