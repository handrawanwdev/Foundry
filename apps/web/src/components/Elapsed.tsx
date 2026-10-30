import { useEffect, useState } from "react";
import { formatElapsed } from "../elapsed";

// Live clock next to a loading line ("Memahami kebutuhan… 0:12").
// With a job timestamp, count from that instant. Without one, count from mount
// so boot and fetch loaders still tick.
export function Elapsed({ started }: { started?: string }) {
  const [mountedAt] = useState(() => Date.now());
  const parsed = started ? Date.parse(started) : NaN;
  const start = Number.isFinite(parsed) ? parsed : mountedAt;
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    setNow(Date.now());
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [start]);

  const text = formatElapsed(Math.max(0, now - start));
  return (
    <span className="elapsed" title={`Sudah berjalan ${text}`}>{text}</span>
  );
}
