import type { Project, Survey, SurveyKind } from "./types";

export const TELL_ANALYSIS = [
  "Bisnis", "Pengguna & peran", "Fitur & sub-fitur", "Alur kerja", "Data", "Integrasi", "Keamanan",
  "Platform & perangkat", "Modul domain", "Auth & izin", "Penyimpanan file", "Worker", "Performa", "Struktur project",
  "Survey FOUNDRY", "Event & Redis", "Anggaran latency",
];

export const SURVEY_KINDS: { id: SurveyKind; label: string }[] = [
  { id: "web", label: "Web" },
  { id: "mobile", label: "Mobile" },
  { id: "desktop", label: "Desktop" },
  { id: "cli", label: "CLI" },
  { id: "api", label: "API" },
];

export const defaultSurvey = (s?: Survey | null): Survey => ({
  kinds: s?.kinds ?? [],
  users: s?.users ?? "self",
  stage: s?.stage ?? "mvp",
  latency: s?.latency ?? "500",
  webhook: s?.webhook ?? "none",
  services: s?.services ?? "one",
  stack: s?.stack ?? "",
});

export function surveySummary(s: Survey): string {
  const kinds = (s.kinds ?? []).length ? s.kinds!.join(", ") : "otomatis";
  return `${kinds} · ${s.stage} · p95 <${s.latency}ms · webhook ${s.webhook}`;
}

export function isExistingProject(p: Project | null | undefined): boolean {
  return p?.structure?.mode === "existing";
}

export function tellTitle(existing: boolean): string {
  return existing ? "Apa yang ingin ditambahkan?" : "Apa yang ingin kamu buat?";
}

export function tellIntro(existing: boolean): string {
  return existing
    ? "Ceritakan fitur atau perubahan. Foundry mengikuti struktur, teknologi, dan UI project ini — tidak diganti template."
    : "Ceritakan dengan bahasa biasa: aplikasi, layanan, pipeline data, firmware, atau perubahan teknis. Foundry menyusun blueprint dan command workflow untuk coding agent eksternal.";
}

export function tellWaiting(existing: boolean): string {
  return existing
    ? "Menganalisis ceritamu terhadap kode yang sudah ada. Tahap ini menyusun blueprint — belum menulis kode baru, dan tidak mengganti project ini."
    : "Menganalisis ceritamu untuk blueprint. Tahap ini belum menulis kode aplikasi.";
}

export function tellPlaceholder(existing: boolean): string {
  return existing ? "Saya ingin menambahkan fitur ..." : "Contoh: aplikasi klinik untuk janji temu pasien, atau firmware sensor suhu yang mengirim data lewat MQTT…";
}

export function existingComponents(p: Project | null | undefined): string {
  return (p?.structure?.components ?? [])
    .map((c) => (c.tech ? `${c.kind} (${c.tech})` : c.kind))
    .join(", ");
}
