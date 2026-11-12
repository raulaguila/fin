import { describe, expect, it } from "vitest";
import { amountToCents, parseCSV, parseOFX, parseStatement, toISODate } from "./statement";

describe("amountToCents", () => {
  it.each([
    ["1.234,56", 123456],
    ["1234.56", 123456],
    ["-12,5", -1250],
    ["R$ 10,00", 1000],
    ["1,234.56", 123456],
    ["(5.00)", -500],
    ["abc", null],
  ])("%s -> %s", (raw, want) => expect(amountToCents(raw)).toBe(want));
});

describe("toISODate", () => {
  it.each([
    ["2026-11-10", "2026-11-10"],
    ["10/11/2026", "2026-11-10"],
    ["20261110120000[-3:BRT]", "2026-11-10"],
    ["11-10-2026", null],
  ])("%s -> %s", (raw, want) => expect(toISODate(raw)).toBe(want));
});

describe("parseOFX", () => {
  const sgml = `OFXHEADER:100
<OFX><BANKMSGSRSV1><STMTTRNRS><STMTRS><BANKTRANLIST>
<STMTTRN><TRNTYPE>DEBIT<DTPOSTED>20261105000000[-3:BRT]<TRNAMT>-55.90<FITID>abc1<MEMO>Netflix
<STMTTRN><TRNTYPE>CREDIT<DTPOSTED>20261106<TRNAMT>1000,00<FITID>abc2<NAME>Salario
</BANKTRANLIST><LEDGERBAL><BALAMT>944.10<DTASOF>20261110</LEDGERBAL></STMTRS></STMTTRNRS></BANKMSGSRSV1></OFX>`;
  it("reads lines, ids and the ledger balance", () => {
    const s = parseOFX(sgml);
    expect(s.lines).toEqual([
      { id: "abc1", date: "2026-11-05", description: "Netflix", amount: -5590 },
      { id: "abc2", date: "2026-11-06", description: "Salario", amount: 100000 },
    ]);
    expect(s.balance).toBe(94410);
  });
  it("is chosen by name or content", () => expect(parseStatement("x.txt", sgml).lines).toHaveLength(2));
});

describe("parseCSV", () => {
  it("flips the Nubank card file, where a charge is positive", () => {
    const s = parseCSV("date,title,amount\n2026-11-01,Uber *Trip,23.50\n2026-11-02,Pagamento recebido,-100\n");
    expect(s.lines).toEqual([
      { date: "2026-11-01", description: "Uber *Trip", amount: -2350 },
      { date: "2026-11-02", description: "Pagamento recebido", amount: 10000 },
    ]);
  });
  it("keeps the sign of the Nubank account file and its ids", () => {
    const s = parseCSV('Data,Valor,Identificador,Descrição\n01/11/2026,-10.00,id-1,"Pix enviado, Ana"\n');
    expect(s.lines).toEqual([{ id: "id-1", date: "2026-11-01", description: "Pix enviado, Ana", amount: -1000 }]);
  });
  it("reads semicolon files and skips bad lines", () => {
    const s = parseCSV("Data;Histórico;Valor\n01/11/2026;Mercado;-1.234,56\nlixo;x;y\n");
    expect(s.lines).toEqual([{ date: "2026-11-01", description: "Mercado", amount: -123456 }]);
  });
  it("refuses a file without the columns", () => expect(() => parseCSV("a,b\n1,2")).toThrow("csv_columns"));
});
