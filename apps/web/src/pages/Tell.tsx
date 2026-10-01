import { useState } from "react";
import { api } from "../api";
import { ThemeGallery } from "../components/ThemeGallery";
import { isWorking } from "../flow";
import { useProject } from "../store/project";

const EXAMPLES = [
  "Aplikasi klinik: pasien membuat janji temu dari HP, admin mengatur jadwal dokter dan tagihan dari web.",
  "Laundry: pelanggan memesan jemput cucian, kurir melihat rute, pemilik melihat laporan harian.",
  "Keuangan pribadi: catat pemasukan dan pengeluaran, anggaran bulanan, grafik ringkasan.",
  "Sistem tiket perusahaan: karyawan membuat tiket, tim IT menangani sesuai prioritas dan SLA.",
];

const ANALYSIS = ["Bisnis", "Pengguna & peran", "Fitur & sub-fitur", "Alur kerja", "Data", "Integrasi", "Keamanan", "Platform & perangkat", "Struktur project"];

export function TellPage() {
  const { project, run, pending } = useProject();
  const [story, setStory] = useState(project?.story ?? "");
  const [confirmReset, setConfirmReset] = useState(false);
  if (!project) return null;

  const discovering = project.stage === "DISCOVERY" && isWorking(project);
  const hasDownstream = (project.features?.length ?? 0) > 0;

  if (discovering) {
    return (
      <section className="card narrow">
        <h1><span className="spinner" /> Foundry memahami kebutuhan</h1>
        <p className="muted">Belum ada yang dibangun. Foundry menganalisis ceritamu dulu:</p>
        <ul className="checklist">{ANALYSIS.map((a) => <li key={a}>{a}</li>)}</ul>
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
    await run("story", () => api.story(story));
  };

  return (
    <section className="card narrow">
      <h1>Apa yang ingin kamu buat?</h1>
      <p className="muted">
        Ceritakan dengan bahasa biasa. Foundry akan membantu menentukan kebutuhan, fitur, platform, dan struktur aplikasinya.
      </p>
      <textarea
        rows={7}
        value={story}
        onChange={(e) => setStory(e.target.value)}
        placeholder="Saya ingin membuat aplikasi klinik..."
      />
      <div className="examples">
        <span className="muted small">Contoh:</span>
        {EXAMPLES.map((e) => (
          <button key={e} className="chip" onClick={() => setStory(e)}>{e.split(":")[0]}</button>
        ))}
      </div>
      <details className="theme-pick">
        <summary className="muted small">Theme visual: {project.theme || "otomatis"}</summary>
        <ThemeGallery project={project} compact />
      </details>
      {confirmReset && (
        <p className="warn">Cerita baru akan membuat ulang rancangan dan task yang sudah ada. Tekan lagi untuk lanjut.</p>
      )}
      <button className="primary big" disabled={story.trim().length < 10 || pending === "story" || isWorking(project)} onClick={submit}>
        Mulai Merancang
      </button>
    </section>
  );
}
