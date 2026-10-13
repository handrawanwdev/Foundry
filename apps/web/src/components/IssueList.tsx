import type { Verification } from "../types";

const LABEL: Record<string, string> = {
  task_open: "task belum selesai",
  task_blocked: "task terblokir",
  test_failed: "test gagal",
  build_failed: "build gagal",
  missing_test: "test belum ada",
  missing_task: "fitur tanpa task",
  missing_page: "fitur tanpa halaman",
  missing_feature: "kebutuhan tanpa fitur",
  broken_flow: "alur belum terverifikasi",
  no_smoke: "app belum terbukti berjalan",
  smoke_failed: "app gagal berjalan",
  no_commands: "build/test belum bisa dijalankan",
};

// Summary counts first, detail on demand.
export function IssueList({ v }: { v: Verification }) {
  const issues = v.issues ?? [];
  const counts = new Map<string, number>();
  for (const i of issues) counts.set(i.kind, (counts.get(i.kind) ?? 0) + 1);
  return (
    <div className="issues">
      <ul>
        {[...counts.entries()].map(([k, n]) => <li key={k}>{n} {LABEL[k] ?? k}</li>)}
      </ul>
      <details className="small">
        <summary>Detail</summary>
        <ul>{issues.map((i, n) => <li key={n}>{i.message}</li>)}</ul>
      </details>
    </div>
  );
}
