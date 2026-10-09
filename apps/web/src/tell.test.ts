import { describe, expect, it } from "vitest";
import { existingComponents, isExistingProject, tellIntro, tellPlaceholder, tellTitle, tellWaiting } from "./tell";
import type { Project } from "./types";

const project = (mode?: "existing" | "new") =>
  ({ structure: mode ? { mode, components: mode === "existing" ? [{ kind: "mobile", path: ".", tech: "React Native" }] : null } : undefined }) as unknown as Project;

describe("tell copy", () => {
  it("treats missing structure as a new project", () => {
    expect(isExistingProject(null)).toBe(false);
    expect(isExistingProject(project())).toBe(false);
    expect(isExistingProject(project("new"))).toBe(false);
    expect(isExistingProject(project("existing"))).toBe(true);
  });

  it("does not claim nothing is built while discovering an existing project", () => {
    expect(tellWaiting(true)).not.toMatch(/Belum ada yang dibangun/);
    expect(tellWaiting(true)).toMatch(/kode yang sudah ada/);
    expect(tellWaiting(true)).toMatch(/tidak mengganti project ini/);
    expect(tellWaiting(false)).not.toMatch(/Belum ada yang dibangun/);
    expect(tellWaiting(false)).toMatch(/belum menulis kode aplikasi/);
  });

  it("asks to add to an existing project instead of creating one", () => {
    expect(tellTitle(true)).toBe("Apa yang ingin ditambahkan?");
    expect(tellIntro(true)).toMatch(/tidak diganti template/);
    expect(tellPlaceholder(true)).toMatch(/menambahkan/);
    expect(tellTitle(false)).toBe("Apa yang ingin kamu buat?");
    expect(tellIntro(false)).not.toMatch(/sebelum ada kode/);
  });

  it("names detected components", () => {
    expect(existingComponents(project("existing"))).toBe("mobile (React Native)");
    expect(existingComponents(project("new"))).toBe("");
  });
});
