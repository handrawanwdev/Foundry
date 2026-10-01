export function AgentsField({
  value, max, fromEnv, disabled, onChange,
}: {
  value: number; max: number; fromEnv?: boolean; disabled?: boolean;
  onChange: (n: number) => void;
}) {
  return (
    <label className="small">Agent paralel
      <input type="number" min={1} max={max} value={value} disabled={disabled || fromEnv}
        onChange={(e) => {
          const n = Number(e.target.value);
          if (Number.isFinite(n)) onChange(Math.min(max, Math.max(1, Math.round(n))));
        }} />
      <span className="muted">{fromEnv ? "FOUNDRY_AGENTS" : `1–${max}`}</span>
    </label>
  );
}
