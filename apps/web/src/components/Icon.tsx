import type { IconName } from "../flow";

const PATHS: Record<IconName, string> = {
  files: "M4 3h9l5 5v13H4z M13 3v5h5",
  chat: "M4 5h16v11H9l-5 4z",
  design: "M3 4h18v14H3z M3 8h18 M8 8v10",
  board: "M3 4h5v16H3z M10 4h5v10h-5z M17 4h4v7h-4z",
  check: "M4 12l5 5L20 6",
  plug: "M9 3v5 M15 3v5 M6 8h12v4a6 6 0 0 1-12 0z M12 18v3",
  help: "M12 21a9 9 0 1 0 0-18a9 9 0 0 0 0 18z M9.6 9.2a2.5 2.5 0 1 1 3.4 2.3c-.6.3-1 .8-1 1.5v.5 M12 16.5v.5",
  close: "M6 6l12 12 M18 6L6 18",
  copy: "M8 8h11v11H8z M5 16V5h11",
  plus: "M12 5v14 M5 12h14",
  minus: "M5 12h14",
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
