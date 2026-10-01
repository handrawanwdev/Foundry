import { describe, expect, it } from "vitest";
import { coverage, escalationDraft, suggestions } from "./support";
import type { Project, SupportMessage } from "./types";

const project = {
  features: [{ id: "FEAT-001", name: "Janji Temu" }],
  flows: [{ id: "FLOW-001", name: "Buat Janji", role: "Pasien", steps: ["Pilih dokter", "Konfirmasi"], page_ids: ["PAGE-001"] }],
  requirements: [], discovery: { roles: [{ name: "Pasien" }] },
  design: { pages: [{ id: "PAGE-001", role: "Pasien", access: "auth", data: { api: ["GET /doctors"] } }] },
} as unknown as Project;

const msg = (role: "user" | "assistant", extra: Partial<SupportMessage>): SupportMessage => ({ id: role, role, text: "", at: "", ...extra });

describe("support", () => {
  it("reports what the system defines, per area", () => {
    expect(coverage(project).filter((c) => c.count === 0).map((c) => c.area)).toEqual(["Business rules", "Module & arsitektur"]);
  });

  it("suggests questions from the system's own flows", () => {
    expect(suggestions(project)).toContain("Bagaimana alur buat janji?");
    expect(suggestions(project)).toContain("Module mana yang menangani janji temu?");
    expect(suggestions({} as Project)).toContain("Kenapa transaksi saya pending?");
  });

  it("drafts the escalation from the latest troubleshooting answer", () => {
    const ms = [
      msg("user", { text: "Kenapa janji saya pending?" }),
      msg("assistant", { answer: { kind: "troubleshoot", summary: "", sources: ["FLOW-001"],
        flow: { name: "Buat Janji", steps: [{ name: "Pilih dokter" }, { name: "Konfirmasi", module: "api/janji", current: true }] },
        trouble: { problem: "Janji pending", status: "Menunggu konfirmasi", causes: [{ text: "Admin belum konfirmasi" }, { text: "Jadwal bentrok", evidence: "api/janji.go:12", confirmed: true }], actions: ["Cek antrean admin"] },
        missing: ["batas waktu konfirmasi"] } }),
      msg("user", { text: "oke" }),
      msg("assistant", { answer: { kind: "explain", summary: "Sama-sama" } }),
    ];
    expect(escalationDraft(ms)).toEqual({
      issue: "Janji pending", module: "api/janji", flow: "Buat Janji — tertahan di “Konfirmasi”", status: "Menunggu konfirmasi",
      evidence: ["Pertanyaan user: Kenapa janji saya pending?", "Sumber: FLOW-001"],
      causes: ["Admin belum konfirmasi (kemungkinan)", "Jadwal bentrok (terkonfirmasi: api/janji.go:12)"],
      investigation: ["Cek antrean admin", "Lengkapi informasi: batas waktu konfirmasi"],
    });
  });
});
