import { describe, expect, it } from "vitest";
import { categoryColor, categoryIcon, merchantName } from "./merchant";

describe("merchantName", () => {
  it("cleans what banks put around the merchant", () => {
    expect(merchantName("UBER *TRIP")).toBe("Uber");
    expect(merchantName("IFOOD *PEDIDO")).toBe("iFood");
    expect(merchantName("NETFLIX.COM")).toBe("Netflix");
    expect(merchantName("PIX ENVIADO JOAO SILVA")).toBe("Joao Silva");
    expect(merchantName("SUPERMERCADO BOM PRECO")).toBe("Supermercado Bom Preco");
    expect(merchantName("LOJA TV 3/10")).toBe("Loja Tv");
    expect(merchantName("Pagamento de Pix QR Code Paymentwall, Inc.")).toBe("Paymentwall");
    expect(merchantName("ALUGUEL DE NOVEMBRO")).toBe("Aluguel de Novembro");
  });

  it("keeps the description when nothing is left", () => {
    expect(merchantName("*")).toBe("*");
  });
});

describe("category look", () => {
  it("gives a known category its icon and a stable color", () => {
    expect(categoryIcon("Mercado")).not.toBe(categoryIcon("Desconhecida"));
    expect(categoryColor("Mercado")).toBe(categoryColor("mercado"));
  });
});
