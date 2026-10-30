import { api } from "../api";
import { useProject } from "../store/project";
import { Elapsed } from "./Elapsed";

// Shows what Foundry is doing right now, for every step.
export function JobBanner() {
  const { project, run, pending } = useProject();
  const job = project?.job;
  if (!job || job.status === "done") return null;

  if (job.status === "failed") {
    return (
      <div className="banner error">
        <div>
          <strong>{job.message} gagal.</strong> <JobError text={job.error ?? ""} />
        </div>
        <div className="banner-actions">
          <button disabled={pending === "retry"} onClick={() => run("retry", api.retry)}>Coba lagi</button>
          <button className="ghost" title="Tutup" aria-label="Tutup" disabled={pending === "dismiss"} onClick={() => run("dismiss", api.dismissJob)}>✕</button>
        </div>
      </div>
    );
  }
  if (job.status === "waiting_host") {
    return (
      <div className="banner waiting">
        <span className="spinner" /> <strong>{job.message}</strong>
        <Elapsed started={job.started} />
        <div className="muted small banner-hint">
          Di AI Host kamu, minta: <code>Lanjutkan pekerjaan Foundry (panggil next_work)</code>
        </div>
      </div>
    );
  }
  return (
    <div className="banner running">
      <span className="spinner" /> <strong>{job.message}…</strong>
      <Elapsed started={job.started} />
    </div>
  );
}

// Long errors (a refused answer lists every problem) show their first
// lines; the rest folds away so the banner does not cover the page.
const SHOWN_LINES = 4;

function JobError({ text }: { text: string }) {
  const lines = text.trim().split("\n");
  if (lines.length <= SHOWN_LINES) return <span style={{ whiteSpace: "pre-line" }}>{text}</span>;
  return (
    <>
      <span style={{ whiteSpace: "pre-line" }}>{lines.slice(0, SHOWN_LINES).join("\n")}</span>
      <details className="banner-more">
        <summary>Detail ({lines.length - SHOWN_LINES} baris lagi)</summary>
        <div style={{ whiteSpace: "pre-line" }}>{lines.slice(SHOWN_LINES).join("\n")}</div>
      </details>
    </>
  );
}
