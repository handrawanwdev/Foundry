import { afterEach, describe, expect, it, vi } from "vitest";
import { useProject } from "./project";

const ok = (data: unknown) => new Response(JSON.stringify({ success: true, data }));

afterEach(() => vi.unstubAllGlobals());

describe("load", () => {
  it("shares a follow-up load between calls that arrive during a load", async () => {
    let version = 0;
    const calls: string[] = [];
    vi.stubGlobal("fetch", async (url: string) => {
      calls.push(url);
      if (url.endsWith("/projects")) return ok({ current: "/p", recent: [] });
      return ok({ dir: "/p", version: ++version });
    });
    const { load } = useProject.getState();
    await Promise.all([load(), load(), load()]);
    expect(calls.filter((c) => c.endsWith("/project"))).toHaveLength(2);
    expect(useProject.getState().project?.version).toBe(2);
  });

  it("keeps loading after a failed load", async () => {
    vi.stubGlobal("fetch", async () => {
      throw new Error("offline");
    });
    await useProject.getState().load();
    expect(useProject.getState().error).toBe("offline");
    vi.stubGlobal("fetch", async () => ok({ current: "", recent: [] }));
    await useProject.getState().load();
    expect(useProject.getState().project).toBeNull();
  });
});
