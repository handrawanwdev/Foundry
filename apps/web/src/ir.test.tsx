import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Wireframe } from "./components/Wireframe";
import { leaves, locate, nodeStyle, resolveLayout, themeClass, tokenVars, validateIR } from "./ir";
import type { DesignSystem, LayoutChild, LayoutNode, Page, ThemeContext } from "./types";

// Same page as internal/core/designir_test.go irPage, so both sides agree.
const irPage = (): Page => ({
  id: "PAGE-001", name: "Tiket", platform: "web", role: "Agen", feature_id: "FEAT-001", sub_feature_ids: null, purpose: "",
  density: "compact",
  components: [
    { id: "menu", type: "nav", label: "", items: ["Tiket"] },
    { id: "title", type: "header", label: "Tiket" },
    { id: "create", type: "button", label: "Buat tiket", emphasis: "action.primary" },
    { id: "filters", type: "card", label: "Filter" },
    { id: "queue", type: "table", label: "Antrian", columns: ["ID", "SLA"], items: ["T-101 | 2 jam"] },
  ],
  layout: {
    frame: "app", nav: "side", width: "full",
    root: { id: "main", type: "stack", gap: "space.md", children: [
      { id: "toolbar", type: "row", align: "between", children: ["title", "create"] },
      { id: "body", type: "sidebar", children: ["filters", "queue"] },
    ] },
    devices: { mobile: { nav: "bottom", hide: ["filters"], nodes: { body: { type: "stack" }, main: { order: ["body", "toolbar"] } }, swap: { queue: "list" } } },
  },
});
const ids = new Set(["PAGE-001"]);

const keys = (n: LayoutNode): string => (n.children ?? []).map((k: LayoutChild) =>
  typeof k === "string" ? k : `${k.id}(${k.type}:${keys(k)})`).join(",");

describe("design IR", () => {
  it("accepts a valid page", () => {
    expect(validateIR(irPage(), ids)).toEqual([]);
  });

  it("rejects invalid IR with actionable errors", () => {
    const cases: [string, (p: Page) => void, string][] = [
      ["no layout", (p) => { delete p.layout; }, "layout belum ditentukan"],
      ["unknown component", (p) => { (p.layout!.root.children![0] as LayoutNode).children = ["ghost", "create"]; }, `komponen "ghost" yang tidak ada`],
      ["orphan", (p) => { (p.layout!.root.children![0] as LayoutNode).children = ["title"]; }, `"create" tidak ditempatkan`],
      ["duplicate id", (p) => { p.components![3].id = "queue"; }, `"queue" dipakai lebih dari sekali`],
      ["unsupported primitive", (p) => { p.layout!.root.type = "flexbox"; }, `primitive "flexbox" tidak didukung`],
      ["arbitrary css", (p) => { p.layout!.root.gap = "37px"; }, `gap "37px" tidak dikenal`],
      ["raw color", (p) => { p.components![2].emphasis = "#123456"; }, `emphasis "#123456" tidak dikenal`],
      ["two primaries", (p) => { p.components![3].emphasis = "action.primary"; }, "maksimal satu aksi utama"],
      ["mobile sidebar", (p) => { delete p.layout!.devices!.mobile.nodes!.body; }, `mobile: node "body" masih sidebar`],
      ["bad swap", (p) => { p.layout!.devices!.mobile.swap = { title: "list" }; }, "tidak didukung (hanya table ↔ list)"],
      ["bad order", (p) => { p.layout!.devices!.mobile.nodes!.main = { order: ["body"] }; }, "order harus berisi tepat anak node"],
    ];
    for (const [name, edit, want] of cases) {
      const p = irPage();
      edit(p);
      expect(validateIR(p, ids).join("\n"), name).toContain(want);
    }
  });

  it("resolves the composition per device", () => {
    const l = irPage().layout!;
    const desk = resolveLayout(l, "desktop");
    expect(keys(desk.root)).toBe("toolbar(row:title,create),body(sidebar:filters,queue)");
    expect(desk.nav).toBe("side");
    const tab = resolveLayout(l, "tablet");
    expect(keys(tab.root)).toBe(keys(desk.root));
    const mob = resolveLayout(l, "mobile");
    expect(keys(mob.root)).toBe("body(stack:queue),toolbar(row:title,create)");
    expect(mob.nav).toBe("bottom");
    expect(mob.swap).toEqual({ queue: "list" });
    expect(leaves(mob.root)).toEqual(["queue", "title", "create"]);
    expect(keys(l.root)).toBe("toolbar(row:title,create),body(sidebar:filters,queue)");
  });

  it("styles primitives only from their tokens", () => {
    expect(nodeStyle({ type: "grid", columns: 3, gap: "space.sm" })).toMatchObject({ gridTemplateColumns: "repeat(3, minmax(0, 1fr))", gap: "var(--wf-space-sm)" });
    expect(nodeStyle({ type: "split", ratio: "1:2" }).gridTemplateColumns).toBe("minmax(0, 1fr) minmax(0, 2fr)");
    expect(nodeStyle({ type: "sidebar", side: "end" }).gridTemplateColumns).toBe("minmax(0, 1fr) var(--wf-side)");
    expect(nodeStyle({ type: "row", align: "between" }).justifyContent).toBe("space-between");
    expect(nodeStyle({ type: "stack", gap: "space.none" }).gap).toBe("0");
  });

  it("resolves semantic tokens from the design system and drops unsafe values", () => {
    const vars = tokenVars({
      colors: [{ name: "action.primary", value: "#0f766e" }, { name: "text.primary", value: "red; background: url(x)" }, { name: "brand", value: "#fff" }],
      typography: [{ name: "type.heading", value: "Inter 22px/28px 600" }],
      spacing: [{ name: "space.md", value: "12px" }, { name: "space.lg", value: "calc(1px)" }],
      radius: [{ name: "radius.md", value: "4px" }],
    });
    expect(vars).toEqual({ "--wf-primary": "#0f766e", "--wf-fs-heading": "22px", "--wf-space-md": "12px", "--wf-radius-md": "4px" });
  });
});

describe("renderer", () => {
  const render = (page: Page, device: "desktop" | "mobile") =>
    renderToStaticMarkup(<Wireframe page={page} pages={[page]} device={device} mode="run" onNavigate={() => {}} app="Helpdesk" />);

  it("same IR gives the same output", () => {
    expect(render(irPage(), "desktop")).toBe(render(irPage(), "desktop"));
    expect(render(irPage(), "mobile")).toBe(render(irPage(), "mobile"));
  });

  it("draws the tree and the device changes, nothing else", () => {
    const desk = render(irPage(), "desktop");
    expect(desk).toContain('data-node-type="sidebar"');
    expect(desk).toContain('class="wf-sidenav side"');
    expect(desk).toContain("wf-btn primary");
    expect(desk).toContain('data-type="table"');
    const mob = render(irPage(), "mobile");
    expect(mob).not.toContain('data-node-type="sidebar"');
    expect(mob).not.toContain('data-id="filters"');
    expect(mob).toContain("wf-bottom");
    expect(mob).toContain("wf-list");
    expect(mob.indexOf('data-id="queue"')).toBeLessThan(mob.indexOf('data-id="title"'));
  });

  it("does not render invalid IR", () => {
    const p = irPage();
    p.layout!.root.gap = "37px";
    const html = render(p, "desktop");
    expect(html).toContain("DESIGN_IR_INVALID");
    expect(html).not.toContain('data-type="table"');
  });

  it("same IR with another theme: other tokens and style, same structure", () => {
    const terminal: ThemeContext = { id: "trading-terminal", name: "Trading Terminal", source: "auto", color_mode: "dark",
      style: { button: "solid", table: "striped", font: "mono", nav: "dark" } };
    const finance: ThemeContext = { id: "finance-minimal", name: "Finance Minimal", source: "user", color_mode: "light",
      style: { button: "outline", table: "plain", nav: "light", card: "flat" } };
    const sys = (primary: string): DesignSystem => ({ colors: [{ name: "action.primary", value: primary }], typography: [], spacing: [] });
    const draw = (theme?: ThemeContext, system?: DesignSystem) =>
      renderToStaticMarkup(<Wireframe page={irPage()} pages={[irPage()]} device="desktop" mode="run" onNavigate={() => {}} app="Helpdesk" theme={theme} system={system} />);
    const a = draw(terminal, sys("#f0b90b"));
    const b = draw(finance, sys("#0f766e"));
    expect(a).not.toBe(b);
    expect(a).toContain("th-dark th-button-solid th-table-striped th-font-mono th-nav-dark");
    expect(a).toContain("--wf-primary:#f0b90b");
    expect(b).toContain("th-light th-button-outline th-table-plain th-nav-light th-card-flat");
    // The structure is the IR's, whatever the theme.
    const structure = (html: string) => html.match(/data-(node-type|id)="[^"]*"/g);
    expect(structure(a)).toEqual(structure(b));
    expect(structure(a)).toEqual(structure(draw()));
  });

  it("old designs without a theme draw the defaults", () => {
    expect(themeClass(undefined)).toBe("");
    expect(themeClass({ id: "x", name: "X", source: "auto", color_mode: "light", style: { button: "neon", evil: "x" } })).toBe("th-light");
    expect(render(irPage(), "desktop")).toContain('class="wf-screen desktop d-compact"');
  });
});

describe("locate", () => {
  const root: LayoutNode = { id: "main", type: "stack", children: ["a", { id: "bar", type: "row", children: ["b", "c"] }, { type: "row", children: ["d"] }] };
  it("names the parent by id, root as empty", () => {
    expect(locate(root, "a")).toEqual({ parent: "", index: 0, count: 3 });
    expect(locate(root, "c")).toEqual({ parent: "bar", index: 1, count: 2 });
    expect(locate(root, "bar")).toEqual({ parent: "", index: 1, count: 3 });
  });
  it("is null outside the tree or under an unnamed node", () => {
    expect(locate(root, "x")).toBeNull();
    expect(locate(root, "d")).toBeNull();
  });
});
