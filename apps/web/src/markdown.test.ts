import { describe, expect, it } from "vitest";
import { parse } from "./components/Markdown";

describe("markdown", () => {
  it("parses blueprint documents into blocks", () => {
    const md = [
      "## Modul", "", "Teks **tebal** dan `kode`.", "lanjutan", "",
      "- satu", "- [ ] cek", "", "1. a", "2. b", "",
      "| A | B |", "|---|---|", "| 1 | x \\| y |", "",
      "```json", "{\"a\": 1}", "```", "> catatan",
    ].join("\n");
    expect(parse(md).map((b) => b.kind)).toEqual(["h", "p", "ul", "ol", "table", "code", "quote"]);
    const table = parse(md)[4];
    expect(table.kind === "table" && table.rows[0]).toEqual(["1", "x | y"]);
    const p = parse(md)[1];
    expect(p.kind === "p" && p.text).toBe("Teks **tebal** dan `kode`. lanjutan");
  });

  it("always advances on lines that look like a table row", () => {
    expect(parse("| bukan tabel |\nteks").map((b) => b.kind)).toEqual(["p"]);
  });
});
