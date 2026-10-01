import type { Escalation, Project, SupportAnswer, SupportMessage } from "./types";

// What the assistant can know about the system, per knowledge area. An area
// without data is shown as not available instead of being guessed.
export function coverage(p: Project): { area: string; count: number }[] {
  const pages = p.design?.pages ?? [];
  const flows = p.flows ?? [];
  return [
    { area: "Fitur", count: p.features?.length ?? 0 },
    { area: "Business flow", count: flows.length },
    { area: "User flow (halaman)", count: flows.filter((f) => (f.page_ids ?? []).length > 0).length },
    { area: "Role", count: p.discovery?.roles?.length ?? 0 },
    { area: "Permission (akses halaman)", count: pages.filter((pg) => pg.access && pg.role).length },
    { area: "Business rules", count: p.requirements?.length ?? 0 },
    { area: "Module & arsitektur", count: p.structure?.components?.length ?? 0 },
    { area: "API", count: pages.filter((pg) => (pg.data?.api ?? []).length > 0).length },
  ];
}

const GENERIC = [
  "Bagaimana alur utama sistem ini?",
  "Kenapa transaksi saya pending?",
  "Siapa yang bisa menyetujui sebuah aksi?",
  "Module mana yang menangani proses ini?",
];

// Starter questions built from the system's own flows and features.
export function suggestions(p: Project): string[] {
  const flows = p.flows ?? [];
  const feature = p.features?.[0];
  if (flows.length === 0) return GENERIC;
  const out = flows.slice(0, 2).map((f) => `Bagaimana alur ${f.name.toLowerCase()}?`);
  const f = flows[0];
  out.push(`Kenapa proses ${f.name.toLowerCase()} saya tertahan (pending)?`);
  out.push(`Siapa yang bisa menjalankan atau menyetujui ${f.name.toLowerCase()}?`);
  if (f.steps.length > 1) out.push(`Setelah “${f.steps[0]}”, apa yang terjadi?`);
  if (feature) out.push(`Module mana yang menangani ${feature.name.toLowerCase()}?`);
  return out;
}

export function lastAnswer(ms: SupportMessage[]): { question: string; answer?: SupportAnswer } {
  let answer: SupportAnswer | undefined;
  for (let i = ms.length - 1; i >= 0; i--) {
    const a = ms[i].answer;
    if (ms[i].role === "assistant" && a && (!answer || a.trouble)) {
      answer = a;
      const q = ms[i - 1]?.role === "user" ? ms[i - 1].text : "";
      if (a.trouble) return { question: q, answer };
    }
  }
  const q = [...ms].reverse().find((m) => m.role === "user")?.text ?? "";
  return { question: q, answer };
}

export type EscalationDraft = Omit<Escalation, "id" | "at">;

// The escalation summary starts from the latest troubleshooting answer (or
// the latest answer); the user edits it before sending.
export function escalationDraft(ms: SupportMessage[]): EscalationDraft {
  const { question, answer: a } = lastAnswer(ms);
  const t = a?.trouble;
  const step = a?.flow?.steps.find((s) => s.current);
  const flow = a?.flow ? `${a.flow.name}${step ? ` — tertahan di “${step.name}”` : ""}` : "";
  return {
    issue: t?.problem || question,
    module: t?.module || step?.module || "",
    flow: flow || t?.step || "",
    status: t?.status || "",
    evidence: [...(question ? [`Pertanyaan user: ${question}`] : []), ...(a?.sources ?? []).map((s) => `Sumber: ${s}`)],
    causes: (t?.causes ?? []).map((c) => c.confirmed ? `${c.text} (terkonfirmasi: ${c.evidence})` : `${c.text} (kemungkinan)`),
    investigation: [...(t?.actions ?? []), ...(a?.missing ?? []).map((m) => `Lengkapi informasi: ${m}`)],
  };
}

export function escalationText(e: EscalationDraft): string {
  const list = (xs?: string[]) => (xs ?? []).length ? (xs ?? []).map((x) => `- ${x}`).join("\n") : "- (belum ada)";
  return [
    `Issue: ${e.issue}`,
    `Affected module: ${e.module || "belum diketahui"}`,
    `Current flow: ${e.flow || "belum diketahui"}`,
    `Current status: ${e.status || "belum diketahui"}`,
    `Evidence:\n${list(e.evidence)}`,
    `Possible cause:\n${list(e.causes)}`,
    `Recommended investigation:\n${list(e.investigation)}`,
  ].join("\n");
}
