import type { ReactNode } from "react";

// Minimal Markdown for blueprint documents: headings, paragraphs, lists,
// checklists, tables, quotes and code blocks; inline code, bold and links.
// Renders React elements only (no raw HTML), so AI text cannot inject markup.
export function Markdown({ text, className = "" }: { text: string; className?: string }) {
  return <div className={`md ${className}`}>{blocks(text)}</div>;
}

type Block =
  | { kind: "h"; level: number; text: string }
  | { kind: "p"; text: string }
  | { kind: "ul" | "ol"; items: string[] }
  | { kind: "quote"; text: string }
  | { kind: "code"; lang: string; text: string }
  | { kind: "table"; head: string[]; rows: string[][] };

export function parse(md: string): Block[] {
  const lines = md.replace(/\r\n/g, "\n").split("\n");
  const out: Block[] = [];
  let i = 0;
  const isList = (l: string) => /^\s*([-*]|\d+[.)])\s+/.test(l);
  const isTableRow = (l: string) => /^\s*\|.*\|\s*$/.test(l);
  while (i < lines.length) {
    const line = lines[i];
    if (!line.trim()) { i++; continue; }
    const fence = line.match(/^\s*```(\S*)/);
    if (fence) {
      const body: string[] = [];
      for (i++; i < lines.length && !/^\s*```/.test(lines[i]); i++) body.push(lines[i]);
      i++;
      out.push({ kind: "code", lang: fence[1], text: body.join("\n") });
      continue;
    }
    const h = line.match(/^(#{1,6})\s+(.*)$/);
    if (h) { out.push({ kind: "h", level: h[1].length, text: h[2].trim() }); i++; continue; }
    if (isTableRow(line) && i + 1 < lines.length && /^\s*\|?\s*:?-{2,}/.test(lines[i + 1])) {
      const head = cells(line);
      const rows: string[][] = [];
      for (i += 2; i < lines.length && isTableRow(lines[i]); i++) rows.push(cells(lines[i]));
      out.push({ kind: "table", head, rows });
      continue;
    }
    if (isList(line)) {
      const ordered = /^\s*\d/.test(line);
      const items: string[] = [];
      for (; i < lines.length && (isList(lines[i]) || (/^\s{2,}\S/.test(lines[i]) && items.length)); i++) {
        if (isList(lines[i])) items.push(lines[i].replace(/^\s*([-*]|\d+[.)])\s+/, ""));
        else items[items.length - 1] += " " + lines[i].trim();
      }
      out.push({ kind: ordered ? "ol" : "ul", items });
      continue;
    }
    if (/^\s*>/.test(line)) {
      const body: string[] = [];
      for (; i < lines.length && /^\s*>/.test(lines[i]); i++) body.push(lines[i].replace(/^\s*>\s?/, ""));
      out.push({ kind: "quote", text: body.join(" ") });
      continue;
    }
    // A paragraph takes at least this line, so parsing always advances.
    const body: string[] = [line.trim()];
    for (i++; i < lines.length && lines[i].trim() && !/^(#{1,6}\s|\s*```|\s*>)/.test(lines[i]) && !isList(lines[i]) && !isTableRow(lines[i]); i++) {
      body.push(lines[i].trim());
    }
    out.push({ kind: "p", text: body.join(" ") });
  }
  return out;
}

function cells(row: string): string[] {
  return row.trim().replace(/^\||\|$/g, "").split(/(?<!\\)\|/).map((c) => c.trim().replace(/\\\|/g, "|"));
}

function blocks(md: string): ReactNode[] {
  return parse(md).map((b, k) => {
    switch (b.kind) {
      case "h": {
        const Tag = (["h2", "h3", "h4", "h5", "h5", "h5"] as const)[b.level - 1];
        return <Tag key={k}>{inline(b.text)}</Tag>;
      }
      case "p": return <p key={k}>{inline(b.text)}</p>;
      case "quote": return <blockquote key={k}>{inline(b.text)}</blockquote>;
      case "code": return <pre key={k} className="md-code" data-lang={b.lang || undefined}><code>{b.text}</code></pre>;
      case "table":
        return (
          <div key={k} className="md-table">
            <table>
              <thead><tr>{b.head.map((c, j) => <th key={j}>{inline(c)}</th>)}</tr></thead>
              <tbody>{b.rows.map((r, j) => <tr key={j}>{r.map((c, n) => <td key={n}>{inline(c)}</td>)}</tr>)}</tbody>
            </table>
          </div>
        );
      default: {
        const Tag = b.kind;
        return (
          <Tag key={k}>
            {b.items.map((it, j) => {
              const check = it.match(/^\[( |x)\]\s+(.*)$/i);
              return check
                ? <li key={j} className="md-check"><span aria-hidden>{check[1].trim() ? "☑" : "☐"}</span> {inline(check[2])}</li>
                : <li key={j}>{inline(it)}</li>;
            })}
          </Tag>
        );
      }
    }
  });
}

// Inline: `code`, **bold**, [text](url). Only http(s) links become anchors.
function inline(text: string): ReactNode[] {
  const out: ReactNode[] = [];
  const re = /`([^`]+)`|\*\*([^*]+)\*\*|\[([^\]]+)\]\(([^)\s]+)\)/g;
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text))) {
    if (m.index > last) out.push(text.slice(last, m.index));
    if (m[1] !== undefined) out.push(<code key={m.index}>{m[1]}</code>);
    else if (m[2] !== undefined) out.push(<strong key={m.index}>{m[2]}</strong>);
    else if (/^https?:\/\//.test(m[4])) out.push(<a key={m.index} href={m[4]} target="_blank" rel="noreferrer noopener">{m[3]}</a>);
    else out.push(<span key={m.index} className="md-ref">{m[3]}</span>);
    last = re.lastIndex;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}
