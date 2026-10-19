import type { IconName } from "../flow";

const PATHS: Record<IconName, string> = {
  copy: "M8 8h11v11H8z M5 16V5h11",
  chevron: "M6 9l6 6 6-6",
  terminal: "M5 8l5 4-5 4 M12 16h7",
  host: "M12 3a9 9 0 1 0 0 18a9 9 0 0 0 0-18z M3 12h18 M12 3c2.5 3 2.5 15 0 18 M12 3c-2.5 3-2.5 15 0 18",
};

export function Icon({ name, size = 22 }: { name: IconName; size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d={PATHS[name]} />
    </svg>
  );
}
