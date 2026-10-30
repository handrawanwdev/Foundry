import type { Project, Survey, SurveyArch, SurveyKind, SurveyPriority, SurveyStage, SurveyUsers } from "./types";

// The seven planning domains Foundry analyzes the story across, shown while
// it understands the request. They mirror the discovery prompt's analysisDomains
// (internal/modules/work/domains.go): a reasoning framework, not seven
// artifacts or files. Keep the two in sync.
export const TELL_ANALYSIS = [
  "Product & Business",
  "Domain & Data",
  "Security & Access",
  "Experience & Platform",
  "Architecture & Infrastructure",
  "Quality & Delivery",
  "Planning Validation",
];

type Option<T extends string> = { id: T; label: string; hint?: string };

export const SURVEY_KINDS: Option<SurveyKind>[] = [
  { id: "web", label: "Web" },
  { id: "mobile", label: "Mobile" },
  { id: "desktop", label: "Desktop" },
  { id: "cli", label: "CLI" },
  { id: "api", label: "API" },
  { id: "embedded", label: "Embedded" },
  { id: "other", label: "Lainnya" },
];

export const SURVEY_USERS: Option<SurveyUsers>[] = [
  { id: "public", label: "Publik, tanpa akun" },
  { id: "registered", label: "Pengguna terdaftar" },
  { id: "internal", label: "Internal perusahaan" },
  { id: "mixed", label: "Campuran pengguna dan admin" },
  { id: "system", label: "Sistem atau layanan lain" },
];

export const SURVEY_STAGES: Option<SurveyStage>[] = [
  { id: "prototype", label: "Prototype", hint: "Validasi ide dan alur." },
  { id: "mvp", label: "MVP", hint: "Fitur inti siap digunakan." },
  { id: "production", label: "Production-ready", hint: "Keamanan, testing, deployment, observability, dan operasional." },
];

export const SURVEY_ARCHS: Option<SurveyArch>[] = [
  { id: "modular", label: "Satu aplikasi modular" },
  { id: "services", label: "Beberapa service terpisah" },
  { id: "existing", label: "Mengikuti arsitektur project yang sudah ada" },
];

export const SURVEY_PRIORITIES: Option<SurveyPriority>[] = [
  { id: "security", label: "Keamanan dan kontrol akses" },
  { id: "performance", label: "Performa dan latency" },
  { id: "reliability", label: "Reliabilitas dan konsistensi data" },
  { id: "scalability", label: "Skalabilitas" },
  { id: "testing", label: "Testing dan verifikasi" },
  { id: "operations", label: "Deployment dan operasional" },
];

const CODE_KINDS: Record<string, SurveyKind> = { web: "web", mobile: "mobile", desktop: "desktop", cli: "cli", backend: "api" };

// kindsFromCode is the platforms an existing project's code already has.
export function kindsFromCode(p: Project | null | undefined): SurveyKind[] {
  const out: SurveyKind[] = [];
  for (const c of p?.structure?.components ?? []) {
    const k = CODE_KINDS[c.kind];
    if (k && !out.includes(k)) out.push(k);
  }
  return out;
}

// defaultSurvey is the saved answers; for an existing project what its
// code already shows (platforms, architecture) is not asked again.
export const defaultSurvey = (s?: Survey | null, p?: Project | null): Survey => {
  const existing = isExistingProject(p);
  return {
    kinds: s?.kinds?.length ? s.kinds : existing ? kindsFromCode(p) : [],
    users: s?.users ?? "",
    stage: s?.stage ?? "", // shown as MVP; empty keeps it Foundry's default
    architecture: s?.architecture ?? (existing ? "existing" : ""),
    priorities: s?.priorities ?? [],
    constraints: s?.constraints ?? "",
    design_reference: s?.design_reference ?? "",
  };
};

const label = <T extends string>(opts: Option<T>[], id?: T | "") => opts.find((o) => o.id === id)?.label;

export function surveySummary(s: Survey): string {
  const kinds = s.kinds?.length ? s.kinds.map((k) => label(SURVEY_KINDS, k)).join(", ") : "platform otomatis";
  const stage = label(SURVEY_STAGES, s.stage) ?? "MVP";
  const arch = label(SURVEY_ARCHS, s.architecture) ?? "arsitektur otomatis";
  return `${kinds} · ${stage} · ${arch}`;
}

// archOptions hides "follow the existing project" when there is no code to
// follow, unless it was chosen before.
export function archOptions(existing: boolean, current?: string): Option<SurveyArch>[] {
  return SURVEY_ARCHS.filter((o) => o.id !== "existing" || existing || current === "existing");
}

export function toggle<T>(list: T[] | undefined, v: T): T[] {
  const cur = list ?? [];
  return cur.includes(v) ? cur.filter((x) => x !== v) : [...cur, v];
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
    : "Ceritakan dengan bahasa biasa: aplikasi, layanan, pipeline data, firmware, atau perubahan teknis. Foundry menyusun daftar fitur, lalu blueprint dan command workflow untuk coding agent eksternal.";
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
