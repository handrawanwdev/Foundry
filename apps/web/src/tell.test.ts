import { describe, expect, it } from "vitest";
import { archOptions, defaultSurvey, existingComponents, isExistingProject, surveySummary, tellIntro, tellPlaceholder, tellTitle, tellWaiting, toggle } from "./tell";
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

  it("leaves unanswered survey questions to Foundry", () => {
    const s = defaultSurvey();
    expect(s).toEqual({ kinds: [], users: "", stage: "", architecture: "", priorities: [], constraints: "" });
    expect(surveySummary(s)).toBe("platform otomatis · MVP · arsitektur otomatis");
    expect(archOptions(false).map((o) => o.id)).toEqual(["modular", "services"]);
    expect(archOptions(false, "existing").map((o) => o.id)).toContain("existing");
  });

  it("does not ask an existing project again what its code shows", () => {
    const p = { structure: { mode: "existing", components: [{ kind: "web" }, { kind: "backend" }, { kind: "database" }, { kind: "web" }] } } as unknown as Project;
    const s = defaultSurvey(null, p);
    expect(s.kinds).toEqual(["web", "api"]);
    expect(s.architecture).toBe("existing");
    expect(archOptions(true).map((o) => o.id)).toContain("existing");
    // Saved answers win over what the code shows.
    expect(defaultSurvey({ kinds: ["mobile"], architecture: "modular", stage: "production" }, p)).toMatchObject({ kinds: ["mobile"], architecture: "modular", stage: "production" });
    expect(surveySummary({ kinds: ["web", "embedded"], stage: "production", architecture: "services" })).toBe("Web, Embedded · Production-ready · Beberapa service terpisah");
  });

  it("toggles multi-select answers", () => {
    expect(toggle(undefined, "web")).toEqual(["web"]);
    expect(toggle(["web", "api"], "web")).toEqual(["api"]);
  });
});
