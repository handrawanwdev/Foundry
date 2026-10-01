import { useLocation } from "react-router-dom";
import { api } from "../api";
import { useProject } from "../store/project";

// Shows what Foundry is doing right now, for every step.
export function JobBanner() {
  const { project, run, pending } = useProject();
  const loc = useLocation();
  const job = project?.job;
  if (!job || job.status === "done") return null;
  // The design workspace shows its own progress.
  if (job.status === "running" && job.kind === "design" && loc.pathname === "/simple/rancang") return null;

  if (job.status === "failed") {
    return (
      <div className="banner error">
        <div>
          <strong>{job.message} gagal.</strong> <JobError text={job.error ?? ""} />
        </div>
        {/* An unclear change request is not retried: the user writes a clearer one. */}
        {job.kind !== "revise" && <button disabled={pending === "retry"} onClick={() => run("retry", api.retry)}>Coba lagi</button>}
      </div>
    );
  }
  if (job.status === "waiting_host") {
    return (
      <div className="banner waiting">
        <span className="spinner" /> <strong>{job.message}</strong>
        <div className="muted small">
          Di AI Host kamu, minta: <code>Lanjutkan pekerjaan Foundry (panggil next_work)</code>
        </div>
      </div>
    );
  }
  return (
    <div className="banner running">
      <span className="spinner" /> <strong>{job.message}…</strong>
      {job.kind === "build" && (
        <button className="ghost" disabled={pending === "stop"} onClick={() => run("stop", api.stopBuild)}>Hentikan</button>
      )}
    </div>
  );
}

// Long errors (a rejected patch lists every operation) show their first
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
