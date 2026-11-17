import { useState } from "react";
import { api } from "../api";
import { useProject } from "../store/project";
import type { Job } from "../types";
import { ConfirmDialog } from "./ConfirmDialog";

// Stops the running AI job: the agent process is killed on the server, what
// already landed stays, and the job can be continued with "Lanjutkan".
// runOpen: a blueprint run still shows work; with no job behind it the stop
// closes the run.
export function StopJobButton({ job, runOpen = false, label = "Hentikan" }: { job?: Job; runOpen?: boolean; label?: string }) {
  const { run, pending } = useProject();
  const [confirm, setConfirm] = useState(false);
  const active = !!job && (job.status === "running" || job.status === "waiting_host");
  if (!active && !runOpen) return null;
  const busy = pending === "stopJob";
  return (
    <>
      <button className="danger stop-job" disabled={busy} onClick={() => setConfirm(true)}>
        <span aria-hidden>■</span> {busy ? "Menghentikan…" : label}
      </button>
      {confirm && (
        <ConfirmDialog
          title={active ? `Hentikan "${job.message}"?` : "Tutup perancangan Blueprint?"}
          body={active
            ? "Proses AI dimatikan sekarang. Bagian yang sudah selesai tetap tersimpan; pekerjaan yang sedang ditulis dibuang. Kamu bisa melanjutkannya nanti."
            : "Tidak ada proses AI yang sedang berjalan. Tampilan progres ditutup; bagian yang belum selesai bisa dilanjutkan dengan Susun rancangan."}
          confirmLabel="Ya, hentikan"
          danger
          busy={busy}
          busyLabel="Menghentikan…"
          onCancel={() => setConfirm(false)}
          onConfirm={() => void run("stopJob", api.stopJob).finally(() => setConfirm(false))}
        />
      )}
    </>
  );
}
