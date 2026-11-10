import { expect, test } from "@playwright/test";
import { choosePassword, invite, login, strongPassword, uniqueEmail , guestPage } from "./helpers";

test("an invited person activates the account and signs in", async ({ page, browser }) => {
  const email = uniqueEmail("invite");
  const link = await invite(page, "Pessoa Convidada", email);

  const guest = await guestPage(browser);
  await guest.goto(link);
  await expect(guest).not.toHaveURL(/#token=/); // the token leaves the address bar
  await choosePassword(guest, strongPassword);
  await guest.getByRole("button", { name: "Ativar conta" }).click();
  await expect(guest.getByText("Conta ativada")).toBeVisible();

  await login(guest, email, strongPassword);
  await expect(guest.getByRole("heading", { level: 1 })).toContainText("Olá, Pessoa");
  await guest.close();
});

test("an activation link works only once", async ({ page, browser }) => {
  const link = await invite(page, "Pessoa Repetida", uniqueEmail("once"));

  for (const attempt of [1, 2]) {
    const guest = await guestPage(browser);
    await guest.goto(link);
    await choosePassword(guest, strongPassword);
    await guest.getByRole("button", { name: "Ativar conta" }).click();
    if (attempt === 1) await expect(guest.getByText("Conta ativada")).toBeVisible();
    else await expect(guest.getByRole("alert")).toBeVisible();
    await guest.close();
  }
});
