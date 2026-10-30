import { describe, expect, it } from "vitest";
import {
  addFeature, addModule, featureCount, featureMapOf, fromFeatures, locate, pathOf, removeId, slugId, toSave, updateNode, usedIds,
} from "./features";
import type { FeatureMap, Project } from "./types";

const map: FeatureMap = {
  summary: "Klinik",
  modules: [{
    id: "janji-temu",
    name: "Janji Temu",
    features: [{
      id: "buat-janji",
      name: "Buat Janji",
      description: "Pasien membuat janji",
      purpose: "Jadwal periksa",
      benefit: "Tanpa antrian",
      outcome: "Janji tersimpan",
      children: [{ id: "cari-dokter", name: "Cari dokter" }],
    }],
  }],
};

describe("featureMapOf", () => {
  it("prefers the saved feature map even when empty", () => {
    const p = { feature_map: { modules: [] }, features: [{ id: "F1", name: "Lama" }] } as unknown as Project;
    expect(featureMapOf(p).modules).toEqual([]);
  });

  it("derives a one-module map from older blueprint features", () => {
    const p = {
      features: [{
        id: "F1", name: "Janji Temu", description: "Pasien membuat janji",
        sub_features: [{ id: "x", name: "Cari dokter" }],
      }],
    } as unknown as Project;
    const m = featureMapOf(p);
    expect(m.modules[0].name).toBe("Fitur");
    expect(m.modules[0].features[0]).toMatchObject({ id: "F1", name: "Janji Temu" });
    expect(m.modules[0].features[0].children?.[0].name).toBe("Cari dokter");
  });

  it("returns an empty map when nothing is stored", () => {
    expect(featureMapOf(null).modules).toEqual([]);
    expect(fromFeatures([]).modules[0].features).toEqual([]);
  });
});

describe("tree edits", () => {
  it("counts every feature node", () => {
    expect(featureCount(map)).toBe(2);
    expect(featureCount({ modules: [] })).toBe(0);
  });

  it("adds a module and a nested feature with unique ids", () => {
    let next = addModule(map, "Tagihan");
    expect(next.modules.map((x) => x.id)).toContain("tagihan");
    next = addFeature(next, "tagihan", null, "Invoice");
    expect(locate(next, "invoice").node?.name).toBe("Invoice");
    next = addFeature(next, "janji-temu", "buat-janji", "Pilih jam");
    expect(locate(next, "buat-janji").node?.children?.some((c) => c.name === "Pilih jam")).toBe(true);
  });

  it("updates and removes without dropping siblings", () => {
    const edited = updateNode(map, "buat-janji", { purpose: "Jadwal pasti" });
    expect(locate(edited, "buat-janji").node?.purpose).toBe("Jadwal pasti");
    expect(featureCount(removeId(edited, "cari-dokter"))).toBe(1);
    expect(removeId(edited, "janji-temu").modules).toEqual([]);
  });

  it("builds the ancestor path for a nested feature", () => {
    const nested = pathOf(map, "cari-dokter");
    expect(nested.module?.id).toBe("janji-temu");
    expect(nested.nodes.map((n) => n.id)).toEqual(["buat-janji", "cari-dokter"]);
    expect(pathOf(map, "janji-temu").nodes).toEqual([]);
  });

  it("strips the in-flight request before save", () => {
    expect(toSave({ ...map, request: { action: "gaps" } })).toEqual({ summary: "Klinik", modules: map.modules });
  });

  it("slugs ids and skips ones already used", () => {
    const used = usedIds(map);
    expect(slugId("Buat Janji", "feat", used)).toBe("buat-janji-2");
    expect(slugId("!!!", "feat", used)).toBe("feat");
  });
});
