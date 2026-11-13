import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";

// The money path a regression would hurt most: an entry lands, shows in the
// list and the report, the data page sees the account, and export carries it.
test("a manual entry reaches the list, the report and the export", async ({ page }) => {
  const description = `e2e mercado ${Date.now()}`;

  await page.goto("/transactions");
  await page.getByRole("button", { name: "Novo lançamento" }).click();
  await page.getByLabel("Descrição").fill(description);
  await page.getByLabel("Valor").fill("12,34");
  await page.getByRole("button", { name: "Salvar" }).click();
  await expect(page.getByText("Lançamento salvo.")).toBeVisible();
  await expect(page.getByText(description).first()).toBeVisible();

  await page.goto("/report");
  await expect(page.getByRole("heading", { name: "Relatório anual" })).toBeVisible();

  await page.goto("/data");
  await expect(page.getByRole("heading", { name: "Contas" })).toBeVisible();
  await expect(page.getByText(/Carteira/).first()).toBeVisible();

  const download = page.waitForEvent("download");
  await page.getByRole("button", { name: "Exportar tudo" }).click();
  const archive = JSON.parse(await readFile((await (await download).path())!, "utf8"));
  expect(archive.version).toBe(1);
  const rows = (archive.accounts as { transactions: { description: string; amount: number }[] }[]).flatMap((a) => a.transactions);
  expect(rows).toContainEqual(expect.objectContaining({ description, amount: -1234 }));
});
