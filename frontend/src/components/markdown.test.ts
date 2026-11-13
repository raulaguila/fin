import { describe, expect, it } from "vitest";
import { blocks } from "./markdown";

describe("blocks", () => {
  it("splits paragraphs, headings and lists", () => {
    const text = "## Resumo\nGastou **R$ 50**\nno mês.\n\n- Mercado\n* Uber\n\n1. Corte o iFood\n2) Revise a Netflix";
    expect(blocks(text)).toEqual([
      { kind: "h", text: "Resumo" },
      { kind: "p", text: "Gastou **R$ 50**\nno mês." },
      { kind: "ul", items: ["Mercado", "Uber"] },
      { kind: "ol", items: ["Corte o iFood", "Revise a Netflix"] },
    ]);
  });

  it("keeps a negative amount as text", () => {
    expect(blocks("-R$ 30,00 no Uber")).toEqual([{ kind: "p", text: "-R$ 30,00 no Uber" }]);
  });
});
