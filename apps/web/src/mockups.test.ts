import { describe, expect, it } from "vitest";
import { frameGroups, mockupGroups, pageFindings, withQuery } from "./mockups";
import type { DesignGate, FlowWalk, LayoutSpec, MockupStatus } from "./types";

const m = (page: string): MockupStatus => ({ page, name: page, platform: "web", pattern: "table", template: true, hash: "", status: "pending" });

describe("mockup workspace", () => {
  const list = [m("dash"), m("users"), m("user-form"), m("login")];

  it("groups pages by the navigation, then the rest", () => {
    const layout = { shells: null, pages: null, navigation: [
      { name: "Utama", items: [{ label: "Dashboard", page: "dash" }, { label: "Pengguna", page: "users" }] },
      { name: "Admin", items: [{ label: "Pengguna", page: "users" }, { label: "Hilang", page: "nope" }] },
    ] } as LayoutSpec;
    const g = mockupGroups(list, layout);
    expect(g.map((x) => [x.name, x.pages.map((p) => p.page)])).toEqual([
      ["Utama", ["dash", "users"]],
      ["Di luar menu", ["user-form", "login"]],
    ]);
    expect(mockupGroups(list, null).map((x) => x.name)).toEqual(["Halaman"]);
  });

  it("lays a use case out in its order", () => {
    const flow = { id: "UC-1", name: "Kelola pengguna", pages: ["users", "user-form", "users"], steps: [] } as FlowWalk;
    expect(mockupGroups(list, null, flow)).toEqual([{ name: "UC-1 · Kelola pengguna", pages: [list[1], list[2]] }]);
  });

  it("finds the gate items that name a page", () => {
    const gate = { checks: [
      { id: "a", name: "A", status: "warn", detail: "", items: ["halaman users belum punya aksi utama", "halaman users-admin (x) belum punya aksi utama"] },
      { id: "b", name: "B", status: "pass", detail: "", items: ["halaman users lolos"] },
    ] } as DesignGate;
    expect(pageFindings(gate, "users")).toEqual(["halaman users belum punya aksi utama"]);
    expect(pageFindings(null, "users")).toEqual([]);
  });

  it("groups a page's frames by action, in order", () => {
    const f = (group: string, name: string) => ({ id: `${group}-${name}`, group, name, kind: "form" as const });
    expect(frameGroups([f("Halaman", "Utama"), f("Tambah", "Form"), f("Tambah", "Gagal"), f("Hapus", "Konfirmasi")])
      .map((g) => [g.name, g.frames.map((x) => x.name)])).toEqual([["Halaman", ["Utama"]], ["Tambah", ["Form", "Gagal"]], ["Hapus", ["Konfirmasi"]]]);
    expect(frameGroups(undefined)).toEqual([]);
  });

  it("adds a frame's query to the mockup URL", () => {
    expect(withQuery("/m/a", "", "view=x", false, "v=2")).toBe("/m/a?view=x&v=2");
    expect(withQuery("/m/a?role=r", "v=2")).toBe("/m/a?role=r&v=2");
    expect(withQuery("/m/a")).toBe("/m/a");
  });
});
