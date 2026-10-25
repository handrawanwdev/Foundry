import { describe, expect, it } from "vitest";
import { recentFolder } from "./Start";

describe("recentFolder", () => {
  it("returns the parent folder of a unix project path", () => {
    expect(recentFolder("/home/handrawan/Project/FoundryProject/rbac-app")).toBe("FoundryProject");
  });

  it("returns the parent of a path one level under Project", () => {
    expect(recentFolder("/home/handrawan/Project/gatra-lang")).toBe("Project");
  });

  it("handles windows separators and a root-level folder", () => {
    expect(recentFolder("C:\\Users\\me\\apps\\rbac-app")).toBe("apps");
    expect(recentFolder("rbac-app")).toBe("");
  });
});
