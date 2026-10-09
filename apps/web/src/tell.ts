import type { Project } from "./types";

export const TELL_ANALYSIS = [
  "Bisnis", "Pengguna & peran", "Fitur & sub-fitur", "Alur kerja", "Data", "Integrasi", "Keamanan",
  "Platform & perangkat", "Modul domain", "Auth & izin", "Penyimpanan file", "Worker", "Performa", "Struktur project",
];

export function isExistingProject(p: Project | null | undefined): boolean {
  return p?.structure?.mode === "existing";
}

export function tellTitle(existing: boolean): string {
  return existing ? "Apa yang ingin ditambahkan?" : "Apa yang ingin kamu buat?";
}

export function tellIntro(existing: boolean): string {
  return existing
    ? "Ceritakan fitur atau perubahan. Foundry mengikuti struktur, teknologi, dan UI project ini — tidak diganti template."
    : "Ceritakan dengan bahasa biasa. Foundry menyusun blueprint arsitektur dulu — modul, data, auth, file, worker.";
}

export function tellWaiting(existing: boolean): string {
  return existing
    ? "Menganalisis ceritamu terhadap kode yang sudah ada. Tahap ini menyusun blueprint — belum menulis kode baru, dan tidak mengganti project ini."
    : "Menganalisis ceritamu untuk blueprint. Tahap ini belum menulis kode aplikasi.";
}

export function tellPlaceholder(existing: boolean): string {
  return existing ? "Saya ingin menambahkan fitur ..." : "Saya ingin membuat aplikasi klinik...";
}

export function existingComponents(p: Project | null | undefined): string {
  return (p?.structure?.components ?? [])
    .map((c) => (c.tech ? `${c.kind} (${c.tech})` : c.kind))
    .join(", ");
}
