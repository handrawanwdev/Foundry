import { useState } from "react";
import { api } from "../api";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import type { Survey } from "../types";
import {
  archOptions, defaultSurvey, existingComponents, isExistingProject, kindsFromCode, SURVEY_KINDS, SURVEY_PRIORITIES, SURVEY_STAGES, SURVEY_USERS,
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
  const [confirmReset, setConfirmReset] = useState(false);
  if (!project) return null;

  const existing = isExistingProject(project);
  const components = existingComponents(project);
  const fromCode = project.survey?.kinds?.length ? [] : kindsFromCode(project);
  const set = (patch: Partial<Survey>) => setSurvey({ ...survey, ...patch });
  const discovering = project.stage === "DISCOVERY" && isWorking(project);
  const hasDownstream = (project.features?.length ?? 0) > 0;

  if (discovering) {
    return (
      <section className="tell">
        <h1><span className="spinner" /> Foundry memahami kebutuhan</h1>
        <p className="muted">{tellWaiting(existing)}</p>
        {components && <p className="muted small">Kode yang dibaca: {components}</p>}
        <ul className="checklist">{TELL_ANALYSIS.map((a) => <li key={a}>{a}</li>)}</ul>
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
    await run("story", () => api.story(story, { ...survey, what: story }));
  };

  return (
    <section className="tell">
      <h1>{tellTitle(existing)}</h1>
      <p className="muted">{tellIntro(existing)}</p>
      {components && <p className="muted small">Kode yang dibaca: {components}</p>}
      <textarea
        rows={7}
        value={story}
        onChange={(e) => setStory(e.target.value)}
        placeholder={tellPlaceholder(existing)}
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
