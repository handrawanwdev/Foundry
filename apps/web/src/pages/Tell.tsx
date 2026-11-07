import { useState } from "react";
import { api } from "../api";
import { Elapsed } from "../components/Elapsed";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import type { Survey, WorkIntent } from "../types";
import {
  archOptions, contextSummary, defaultIntent, defaultSurvey, EXISTING_INTENTS, existingComponents, isExistingProject, kindsFromCode, SURVEY_KINDS, SURVEY_PRIORITIES, SURVEY_STAGES, SURVEY_USERS,
  surveySummary, TELL_ANALYSIS, tellIntro, tellPlaceholder, tellTitle, tellWaiting, toggle,
} from "../tell";

const EXAMPLES = [
  "Aplikasi klinik: pasien membuat janji temu dari HP, admin mengatur jadwal dokter dan tagihan dari web.",
  "Laundry: pelanggan memesan jemput cucian, kurir melihat rute, pemilik melihat laporan harian.",
  "Keuangan pribadi: catat pemasukan dan pengeluaran, anggaran bulanan, grafik ringkasan.",
  "Sistem tiket perusahaan: karyawan membuat tiket, tim IT menangani sesuai prioritas dan SLA.",
];

export function TellPage() {
  const { project, run, pending } = useProject();
  const [story, setStory] = useState(project?.story ?? "");
  const [survey, setSurvey] = useState<Survey>(() => defaultSurvey(project?.survey, project));
  const [intent, setIntent] = useState<WorkIntent>(() => defaultIntent(project));
  const [corrections, setCorrections] = useState(project?.structure?.context?.corrections ?? "");
  const [confirmReset, setConfirmReset] = useState(false);
  if (!project) return null;

  const existing = isExistingProject(project);
  const components = existingComponents(project);
  const ctx = project.structure?.context;
  const ctxLines = contextSummary(ctx);
  const fromCode = project.survey?.kinds?.length ? [] : kindsFromCode(project);
  const set = (patch: Partial<Survey>) => setSurvey({ ...survey, ...patch });
  const discovering = project.stage === "DISCOVERY" && isWorking(project);
  const mapping = project.job?.kind === "feature_map";
  const hasDownstream = (project.features?.length ?? 0) > 0 || (project.feature_map?.modules?.length ?? 0) > 0;

  if (discovering) {
    return (
      <section className="tell">
        <h1>
          <span className="spinner" /> {mapping ? "Menyusun daftar fitur" : "Foundry memahami kebutuhan"}{" "}
          <Elapsed started={project.job?.started} />
        </h1>
        <p className="muted">{mapping
          ? "Foundry lagi bikin daftar fitur dari ceritamu. Bisa kamu ubah dulu sebelum lanjut ke blueprint."
          : tellWaiting(existing)}</p>
        {components && <p className="muted small">Kode yang dibaca: {components}</p>}
        {!mapping && <ul className="checklist">{TELL_ANALYSIS.map((a) => <li key={a}>{a}</li>)}</ul>}
        <blockquote>{project.story}</blockquote>
      </section>
    );
  }

  const submit = async () => {
    if (hasDownstream && !confirmReset) {
      setConfirmReset(true);
      return;
    }
    setConfirmReset(false);
    await run("story", () => api.story(story, { ...survey, what: story }, existing ? intent : "new_project", corrections));
  };

  return (
    <section className="tell">
      <h1>{tellTitle(existing, existing ? intent : "new_project")}</h1>
      <p className="muted">{tellIntro(existing)}</p>
      {components && <p className="muted small">Kode yang dibaca: {components}</p>}
      {existing && (
        <div className="survey-field">
          <span>Jenis pekerjaan</span>
          <div className="survey-kinds">
            {EXISTING_INTENTS.map((k) => (
              <button key={k.id} type="button" aria-pressed={intent === k.id}
                className={intent === k.id ? "chip on" : "chip"}
                title={k.hint}
                onClick={() => setIntent(k.id)}>{k.label}</button>
            ))}
          </div>
          <span className="small">{EXISTING_INTENTS.find((k) => k.id === intent)?.hint}</span>
        </div>
      )}
      {existing && ctx && (
        <details className="theme-pick context-panel">
          <summary className="muted small">
            Hasil baca repository{ctxLines.length ? `: ${ctxLines.join(" · ")}` : ""}
          </summary>
          {ctx.stale && (
            <p className="warn">Konteks kedaluwarsa{ctx.stale_areas?.length ? ` (${ctx.stale_areas.join(", ")})` : ""}. Repository berubah setelah pemindaian. Temuan di bawah bukan fakta terkini sampai dipindai ulang.</p>
          )}
          <p className="muted small">Bukti dari kode dan dokumen. Path bukan tanggung jawab kecuali evidence mengatakannya. Koreksi di bawah mengikat rancangan.</p>
          <ContextList title="Layout" items={ctx.layout?.map((f) => f.text)} />
          <ContextList title="Bahasa" items={ctx.languages?.map((f) => f.text)} />
          <ContextList title="Framework" items={ctx.frameworks?.map((f) => f.text)} />
          <ContextList title="Modul" items={ctx.modules?.map((m) => m.depends_on?.length ? `${m.path} → ${m.depends_on.join(", ")}` : m.path)} />
          <ContextList title="Data" items={ctx.data?.map((e) => e.fields?.length ? `${e.name} (${e.fields.join(", ")})` : e.name)} />
          <ContextList title="API" items={ctx.apis?.map((a) => a.detail || a.path)} />
          <ContextList title="Auth" items={ctx.auth?.map((f) => f.path || f.text)} />
          <ContextList title="Tes" items={ctx.tests?.map((f) => f.text)} />
          <ContextList title="Dependensi" items={ctx.deps?.map((d) => `${d.from} → ${d.to} (${d.kind})`)} />
          <ContextList title="Perubahan repo" items={ctx.changes?.map((ch) => `${ch.action} ${ch.path || ch.area}`)} />
          <ContextList title="Fitur terdokumentasi" items={ctx.features?.map((f) => f.text)} />
          <label className="survey-wide">Koreksi (opsional)
            <textarea rows={2} value={corrections} onChange={(e) => setCorrections(e.target.value)}
              placeholder="Mis. modul billing sudah handle invoice; jangan buat ulang. Auth memakai session, bukan JWT." />
          </label>
        </details>
      )}
      <textarea
        rows={7}
        value={story}
        onChange={(e) => setStory(e.target.value)}
        placeholder={tellPlaceholder(existing, existing ? intent : undefined)}
      />
      {!existing && (
        <div className="examples">
          <span className="muted small">Contoh:</span>
          {EXAMPLES.map((e) => (
            <button key={e} className="chip" onClick={() => setStory(e)}>{e.split(":")[0]}</button>
          ))}
        </div>
      )}
      <div className="tell-options">
        <details className="theme-pick" open={!hasDownstream}>
          <summary className="muted small">Survey FOUNDRY: {surveySummary(survey)}</summary>
          <p className="muted small">Semua boleh dibiarkan. Yang kosong ditentukan Foundry dari cerita dan kebutuhan, dengan alasan — bukan dikarang jadi batasan.</p>
          <div className="survey">
            <div className="survey-field survey-wide">
              <span>Platform {fromCode.length > 0 ? "(terbaca dari kode; ubah bila perlu)" : "(boleh lebih dari satu; kosong = dari cerita)"}</span>
              <div className="survey-kinds">
                {SURVEY_KINDS.map((k) => (
                  <button key={k.id} type="button" aria-pressed={survey.kinds?.includes(k.id) ?? false}
                    className={survey.kinds?.includes(k.id) ? "chip on" : "chip"}
                    onClick={() => set({ kinds: toggle(survey.kinds, k.id) })}>{k.label}</button>
                ))}
              </div>
            </div>
            <label>Pengguna
              <select value={survey.users ?? ""} onChange={(e) => set({ users: e.target.value as Survey["users"] })}>
                <option value="">Foundry baca dari cerita</option>
                {SURVEY_USERS.map((o) => <option key={o.id} value={o.id}>{o.label}</option>)}
              </select>
            </label>
            <label>Target delivery
              <select value={survey.stage || "mvp"} onChange={(e) => set({ stage: e.target.value as Survey["stage"] })}>
                {SURVEY_STAGES.map((o) => <option key={o.id} value={o.id}>{o.label}</option>)}
              </select>
              <span className="small">{SURVEY_STAGES.find((o) => o.id === (survey.stage || "mvp"))?.hint}</span>
            </label>
            <label className="survey-wide">Preferensi arsitektur
              <select value={survey.architecture ?? ""} onChange={(e) => set({ architecture: e.target.value as Survey["architecture"] })}>
                <option value="">Foundry memilih berdasarkan kebutuhan</option>
                {archOptions(existing, survey.architecture).map((o) => <option key={o.id} value={o.id}>{o.label}</option>)}
              </select>
            </label>
            <div className="survey-field survey-wide">
              <span>Prioritas kualitas (opsional) — kualitas dasar tetap dipertimbangkan walau tidak dipilih</span>
              <div className="survey-kinds">
                {SURVEY_PRIORITIES.map((o) => (
                  <button key={o.id} type="button" aria-pressed={survey.priorities?.includes(o.id) ?? false}
                    className={survey.priorities?.includes(o.id) ? "chip on" : "chip"}
                    onClick={() => set({ priorities: toggle(survey.priorities, o.id) })}>{o.label}</button>
                ))}
              </div>
            </div>
            <label className="survey-wide">Batasan wajib (opsional)
              <textarea rows={3} maxLength={4000} value={survey.constraints ?? ""} onChange={(e) => set({ constraints: e.target.value })}
                placeholder="Mis. wajib PostgreSQL; integrasi dengan sistem kasir yang ada; harus jalan di server sendiri; tidak boleh pakai layanan berbayar." />
            </label>
            <label className="survey-wide">Referensi desain / template (opsional)
              <input type="text" value={survey.design_reference ?? ""} onChange={(e) => set({ design_reference: e.target.value })}
                placeholder="Mis. Duolingo; YouTube; Material 3; template minimalis — Foundry menurunkan aturan desain, bukan menyalin merek/aset." />
            </label>
          </div>
        </details>
      </div>
      {confirmReset && (
        <p className="warn">Cerita baru akan membuat ulang rancangan yang sudah ada. Tekan lagi untuk lanjut.</p>
      )}
      <button className="primary big" disabled={story.trim().length < 10 || pending === "story" || isWorking(project)} onClick={submit}>
        Mulai Merancang
      </button>
    </section>
  );
}

function ContextList({ title, items }: { title: string; items?: string[] }) {
  if (!items?.length) return null;
  return (
    <p className="context-line">
      <strong>{title}:</strong> {items.slice(0, 12).join(", ")}{items.length > 12 ? ` (+${items.length - 12})` : ""}
    </p>
  );
}
