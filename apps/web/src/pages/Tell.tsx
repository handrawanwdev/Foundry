import { useState } from "react";
import { api } from "../api";
import { TemplatePicker } from "../components/TemplateGallery";
import { ThemeGallery } from "../components/ThemeGallery";
import { isWorking } from "../flow";
import { useProject } from "../store/project";
import type { Survey, SurveyKind } from "../types";
import { defaultSurvey, existingComponents, isExistingProject, SURVEY_KINDS, surveySummary, TELL_ANALYSIS, tellIntro, tellPlaceholder, tellTitle, tellWaiting } from "../tell";

const EXAMPLES = [
  "Aplikasi klinik: pasien membuat janji temu dari HP, admin mengatur jadwal dokter dan tagihan dari web.",
  "Laundry: pelanggan memesan jemput cucian, kurir melihat rute, pemilik melihat laporan harian.",
  "Keuangan pribadi: catat pemasukan dan pengeluaran, anggaran bulanan, grafik ringkasan.",
  "Sistem tiket perusahaan: karyawan membuat tiket, tim IT menangani sesuai prioritas dan SLA.",
];

export function TellPage() {
  const { project, run, pending } = useProject();
  const [story, setStory] = useState(project?.story ?? "");
  const [survey, setSurvey] = useState<Survey>(() => defaultSurvey(project?.survey));
  const [confirmReset, setConfirmReset] = useState(false);
  if (!project) return null;

  const existing = isExistingProject(project);
  const components = existingComponents(project);
  const discovering = project.stage === "DISCOVERY" && isWorking(project);
  const hasDownstream = (project.features?.length ?? 0) > 0;

  if (discovering) {
    return (
      <section className="card narrow">
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
    <section className="card narrow">
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
      <details className="theme-pick" open={!hasDownstream}>
        <summary className="muted small">Survey FOUNDRY: {surveySummary(survey)}</summary>
        <p className="muted small">Jawaban mengikat stack, latency, event, dan struktur. Kosong = default MVP.</p>
        <div className="survey-kinds">
          {SURVEY_KINDS.map((k) => {
            const on = (survey.kinds ?? []).includes(k.id);
            return (
              <button key={k.id} type="button" className={on ? "chip on" : "chip"} onClick={() => {
                const cur = survey.kinds ?? [];
                const next = on ? cur.filter((x) => x !== k.id) : [...cur, k.id];
                setSurvey({ ...survey, kinds: next as SurveyKind[] });
              }}>{k.label}</button>
            );
          })}
        </div>
        <div className="survey">
          <label>Pengguna
            <select value={survey.users} onChange={(e) => setSurvey({ ...survey, users: e.target.value as Survey["users"] })}>
              <option value="self">hanya saya</option>
              <option value="team">tim kecil</option>
              <option value="public">publik</option>
            </select>
          </label>
          <label>Tahap
            <select value={survey.stage} onChange={(e) => setSurvey({ ...survey, stage: e.target.value as Survey["stage"] })}>
              <option value="prototype">prototipe</option>
              <option value="mvp">MVP</option>
              <option value="production">produksi</option>
            </select>
          </label>
          <label>Latency p95
            <select value={survey.latency} onChange={(e) => setSurvey({ ...survey, latency: e.target.value as Survey["latency"] })}>
              <option value="500">&lt; 500 ms</option>
              <option value="200">&lt; 200 ms</option>
              <option value="100">&lt; 100 ms</option>
            </select>
          </label>
          <label>Webhook
            <select value={survey.webhook} onChange={(e) => setSurvey({ ...survey, webhook: e.target.value as Survey["webhook"] })}>
              <option value="none">tidak</option>
              <option value="in">terima</option>
              <option value="out">kirim</option>
              <option value="both">keduanya</option>
            </select>
          </label>
          <label>Service
            <select value={survey.services} onChange={(e) => setSurvey({ ...survey, services: e.target.value as Survey["services"] })}>
              <option value="one">satu aplikasi modular</option>
              <option value="many">beberapa service terpisah</option>
            </select>
          </label>
          <label>Stack (kosong = Foundry pilih)
            <input value={survey.stack ?? ""} onChange={(e) => setSurvey({ ...survey, stack: e.target.value })} placeholder="opsional" />
          </label>
        </div>
      </details>
      <details className="theme-pick" open={!hasDownstream && !existing}>
        <summary className="muted small">Theme visual (opsional): {project.theme || (existing ? "dari UI yang ada" : "otomatis")}</summary>
        <ThemeGallery project={project} compact />
      </details>
      {existing ? (
        <p className="muted small">Struktur mengikuti kode yang sudah ada; tidak memakai template folder baru.</p>
      ) : (
        <details className="theme-pick" open={!hasDownstream}>
          <summary className="muted small">Template awal (opsional): {project.design_template === "blank" ? "kanvas kosong" : project.design_template || "otomatis"}</summary>
          <p className="muted small">Hanya struktur awal halaman; fitur tetap dari ceritamu.</p>
          <TemplatePicker project={project} />
        </details>
      )}
      {confirmReset && (
        <p className="warn">Cerita baru akan membuat ulang rancangan dan task yang sudah ada. Tekan lagi untuk lanjut.</p>
      )}
      <button className="primary big" disabled={story.trim().length < 10 || pending === "story" || isWorking(project)} onClick={submit}>
        Mulai Merancang
      </button>
    </section>
  );
}
