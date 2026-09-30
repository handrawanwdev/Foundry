import { describe, expect, it } from "vitest";
import { cells, changeRequest, changeScope, designChecks, featuresWithoutPage, formGroups, groupPages, incoming, navTarget, pageDensity, pageKind, pageOrder, primaryButton, requirementsOf, statParts } from "./design";
import type { Flow, Page, Project } from "./types";

const page = (id: string, name: string, role: string, extra: Partial<Page> = {}): Page => ({
  id, name, role, platform: "web", feature_id: "FEAT-001", sub_feature_ids: null, purpose: "", components: [], ...extra,
});
const flow = (id: string, page_ids: string[]): Flow => ({ id, name: id, feature_id: "FEAT-001", role: "", steps: [], page_ids });

describe("design", () => {
  const pages = [
    page("P1", "Login", "Pasien"),
    page("P2", "Janji Temu", "Pasien", { components: [{ type: "button", label: "Buat", target: "P3" }] }),
    page("P3", "Konfirmasi", "Pasien"),
    page("P4", "Dashboard", "Admin", { components: [{ type: "nav", label: "Menu", items: ["Janji Temu", "Tidak ada"] }] }),
  ];

  it("groups pages by role, then by flow, each page once", () => {
    const groups = groupPages(pages, [flow("A", ["P2", "P3"]), flow("B", ["P3", "P9"])]);
    expect(groups.map((g) => g.role)).toEqual(["Pasien", "Admin"]);
    expect(groups[0].loose.map((p) => p.id)).toEqual(["P1"]);
    expect(groups[0].flows.map((f) => f.pages.map((p) => p.id))).toEqual([["P2", "P3"]]);
    expect(pageOrder(groups).map((p) => p.id)).toEqual(["P1", "P2", "P3", "P4"]);
  });

  it("draws at most one primary button: the primary action", () => {
    const comps = [{ type: "button", label: "Filter", target: "P3" }, { type: "button", label: "Buat janji ", target: "P3" }] as Page["components"];
    expect(primaryButton(page("X", "X", "", { components: comps, primary_action: { label: "buat janji" } }))).toBe(1);
    expect(primaryButton(page("X", "X", "", { components: comps, primary_action: { reason: "baca saja" } }))).toBe(-1);
    expect(primaryButton(page("X", "X", "", { components: comps }))).toBe(0);
  });

  it("links nav items to pages by name and finds incoming links", () => {
    expect(navTarget("janji temu", pages[3], pages)?.id).toBe("P2");
    expect(navTarget("Tidak ada", pages[3], pages)).toBeUndefined();
    expect(incoming(pages[1], pages).map((p) => p.id)).toEqual(["P4"]);
    expect(incoming(pages[2], pages).map((p) => p.id)).toEqual(["P2"]);
  });

  it("finds UI features without pages and requirements behind pages", () => {
    const project = {
      design: { pages, status: "draft", feedback: null },
      features: [
        { id: "FEAT-001", name: "Janji", description: "", requirement_ids: ["REQ-001"], platforms: ["web"], sub_features: [] },
        { id: "FEAT-002", name: "Laporan", description: "", requirement_ids: ["REQ-002"], platforms: ["web", "backend"], sub_features: [] },
        { id: "FEAT-003", name: "API", description: "", requirement_ids: [], platforms: ["backend"], sub_features: [] },
      ],
    } as unknown as Project;
    expect(featuresWithoutPage(project).map((f) => f.id)).toEqual(["FEAT-002"]);
    expect(requirementsOf(["P1", "P2"], project)).toEqual(["REQ-001"]);
  });

  it("infers page kind and density for designs without kind", () => {
    expect(pageKind(page("A", "Masuk", "Pasien"))).toBe("auth");
    expect(pageKind(page("B", "Jadwal", "Admin Klinik", { components: [{ type: "table", label: "J" }] }))).toBe("admin");
    expect(pageKind(page("C", "Janji", "Pasien", { components: [{ type: "form", label: "F" }] }))).toBe("form");
    expect(pageKind(page("D", "Masuk", "x", { kind: "status" }))).toBe("status");
    expect(pageDensity(page("A", "Masuk", "Pasien"))).toBe("low");
    expect(pageDensity(page("B", "Jadwal", "Admin", { components: [{ type: "table", label: "J" }] }))).toBe("high");
  });

  it("parses table rows, stats and form groups", () => {
    expect(cells("Budi | Dr. Andi |Aktif")).toEqual(["Budi", "Dr. Andi", "Aktif"]);
    expect(statParts("Janji hari ini: 12")).toEqual(["Janji hari ini", "12"]);
    expect(statParts("Pasien")).toEqual(["Pasien", undefined]);
    expect(formGroups(["Nama", "# Kontak", "Telepon"])).toEqual([{ fields: ["Nama"] }, { title: "Kontak", fields: ["Telepon"] }]);
  });

  it("flags one-template pages", () => {
    const stats = Array.from({ length: 5 }, (_, i) => ({ type: "stat", label: `S${i}` }));
    const same = [{ type: "header", label: "x" }, { type: "table", label: "t", columns: ["a"] }, { type: "button", label: "b" }];
    const project = {
      design: {
        status: "draft", feedback: null, pages: [
          page("P1", "Dashboard", "Admin", { kind: "dashboard", components: [{ type: "header", label: "Hai" }, ...stats] }),
          page("P2", "Pasien", "Admin", { kind: "list", states: { empty: "Belum ada" }, components: same }),
          page("P3", "Dokter", "Admin", { kind: "list", components: same }),
          page("P4", "Form", "Admin", { kind: "form", components: [{ type: "form", label: "F", items: ["a", "b", "c", "d", "e", "f", "g"] }, { type: "nav", label: "n", items: ["Pasien", "Hilang"] }] }),
        ],
      },
    } as unknown as Project;
    const got = designChecks(project).map((f) => `${f.page?.id} ${f.text}`);
    expect(got).toEqual([
      "P1 terlalu banyak kartu/angka (5)",
      "P1 dashboard hanya berisi angka/kartu, belum menampilkan pekerjaan user",
      "P3 belum ada tampilan saat data kosong",
      "P3 susunan sama persis dengan halaman “Pasien”",
      "P4 form “F” punya 7 isian tanpa pengelompokan",
      "P4 form “F” belum punya spesifikasi isian (tipe, wajib, validasi)",
    ]);
  });

  it("builds one change request for several pages", () => {
    expect(changeRequest("  Warna lebih kontras ", [])).toBe("Warna lebih kontras");
    expect(changeRequest("", [{ page: pages[1], text: "tambah filter" }])).toBe('Di halaman "Janji Temu" (P2): tambah filter');
    expect(changeRequest("rapikan nav", [{ page: pages[0], text: "tambah lupa sandi" }, { page: pages[1], text: " " }, { page: pages[3], text: "hapus stat" }])).toBe(
      'Ubah beberapa halaman sekaligus:\n- Di halaman "Login" (P1): tambah lupa sandi\n- Di halaman "Dashboard" (P4): hapus stat\n- Umum: rapikan nav');
    expect(changeRequest("", [{ page: pages[0], text: "" }])).toBe("");
    expect(changeRequest("", [{ page: pages[1], text: "hapus", component: 0 }])).toBe('Di halaman "Janji Temu" (P2), komponen [0] button "Buat": hapus');
  });

  it("locks the change scope", () => {
    expect(changeScope("", [{ page: pages[1], text: " hapus ", component: 0 }, { page: pages[0], text: "" }], { P2: "mobile" })).toEqual(
      { pages: [{ id: "P2", request: "hapus", components: [0], device: "mobile" }] });
    expect(changeScope("rapikan", [{ page: pages[0], text: "ganti jenis", fields: true }])).toEqual(
      { open: true, general: "rapikan", pages: [{ id: "P1", request: "ganti jenis", fields: true }] });
  });
});
