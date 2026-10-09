import { Icon } from "./Icon";

export function AgentsField({
  value, max, fromEnv, disabled, onChange,
}: {
  value: number; max: number; fromEnv?: boolean; disabled?: boolean;
  onChange: (n: number) => void;
}) {
  const locked = disabled || fromEnv;
  const clamp = (n: number) => Math.min(max, Math.max(1, Math.round(n)));
  return (
    <label className="small agents-field">
      Agent paralel
      <span className="stepper">
        <button type="button" aria-label="Kurangi" disabled={locked || value <= 1} onClick={() => onChange(clamp(value - 1))}>
          <Icon name="minus" size={14} />
        </button>
        <input type="number" min={1} max={max} value={value} disabled={locked}
          onChange={(e) => {
            const n = Number(e.target.value);
            if (Number.isFinite(n)) onChange(clamp(n));
          }} />
        <button type="button" aria-label="Tambah" disabled={locked || value >= max} onClick={() => onChange(clamp(value + 1))}>
          <Icon name="plus" size={14} />
        </button>
      </span>
      <span className="muted">{fromEnv ? "FOUNDRY_AGENTS" : `1–${max}`}</span>
    </label>
  );
}
