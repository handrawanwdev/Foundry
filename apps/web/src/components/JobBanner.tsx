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
          <strong>{job.message} gagal.</strong> <span style={{ whiteSpace: "pre-line" }}>{job.error}</span>
        </div>
        <button disabled={pending === "retry"} onClick={() => run("retry", api.retry)}>Coba lagi</button>
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
