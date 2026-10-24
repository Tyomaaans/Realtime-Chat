import { expect, test } from "@playwright/test";

test("three browser sessions complete a private conversation lifecycle", async ({ browser }) => {
  const first = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const second = await browser.newContext({ viewport: { width: 320, height: 800 } });
  const third = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const pageErrors: string[] = [];
  for (const context of [first, second, third]) {
    context.on("page", (page) => page.on("pageerror", (error) => pageErrors.push(error.message)));
  }
  try {
    const a = await first.newPage();
    const b = await second.newPage();
    let c = await third.newPage();
    await Promise.all([a.goto("/"), b.goto("/"), c.goto("/")]);
    await expect(a.getByRole("button", { name: "Start chatting" })).toBeEnabled();
    await c.getByRole("button", { name: "Start chatting" }).click();
    await c.getByRole("button", { name: "Cancel search" }).click();
    await expect(c.getByRole("button", { name: "Find someone", exact: true })).toBeVisible();
    await b.screenshot({ path: "../docs/screenshots/mobile.png", fullPage: true });
    expect(await b.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await a.getByRole("button", { name: "Start chatting" }).click();
    await expect(a.getByRole("button", { name: "Cancel search" })).toBeVisible();
    await b.getByRole("button", { name: "Start chatting" }).click();
    await expect(a.getByRole("textbox")).toBeEnabled();
    await a.getByRole("textbox").fill("Hello from this tab.");
    await a.getByRole("textbox").press("Enter");
    await expect(b.getByText("Hello from this tab.", { exact: true })).toBeVisible();
    await b.screenshot({ path: "../docs/screenshots/mobile-chat.png", fullPage: true });
    const markup = '<img src=x onerror="window.xssExecuted=true">';
    await b.getByRole("textbox").fill(markup);
    await b.getByRole("textbox").press("Enter");
    await expect(a.getByText(markup, { exact: true })).toBeVisible();
    expect(await a.evaluate(() => "xssExecuted" in window)).toBe(false);
    await a.getByRole("button", { name: "Next person" }).click();
    await expect(a.getByRole("dialog")).toBeVisible();
    await expect(a.getByRole("button", { name: "Keep chatting" })).toBeFocused();
    await a.keyboard.press("Escape");
    await expect(a.getByRole("button", { name: "Next person" })).toBeFocused();
    await a.getByRole("button", { name: "Next person" }).click();
    await a.getByRole("button", { name: "Find someone new", exact: true }).click();
    await expect(b.getByText("Hello from this tab.", { exact: true })).toHaveCount(0);
    await expect(b.getByRole("button", { name: "Find someone", exact: true })).toBeVisible();
    await c.getByRole("button", { name: "Find someone", exact: true }).click();
    await expect(a.getByRole("textbox")).toBeEnabled();
    await a.getByRole("textbox").fill("Hi! What made you smile today?");
    await a.getByRole("textbox").press("Enter");
    await expect(c.getByText("Hi! What made you smile today?", { exact: true })).toBeVisible();
    await c.getByRole("textbox").fill("A good conversation is a nice start.");
    await c.getByRole("textbox").press("Enter");
    await expect(a.getByText("A good conversation is a nice start.", { exact: true })).toBeVisible();
    await a.screenshot({ path: "../docs/screenshots/desktop.png", fullPage: true });
    await c.close();
    await expect(a.getByText("The other person left. Your messages have been cleared.")).toBeVisible();
    await expect(a.getByText("Hi! What made you smile today?", { exact: true })).toHaveCount(0);
    c = await third.newPage();
    await c.goto("/");
    await c.getByRole("button", { name: "Start chatting" }).click();
    await a.getByRole("button", { name: "Find someone", exact: true }).click();
    await expect(c.getByRole("textbox")).toBeEnabled();
    await c.getByRole("button", { name: "Stop", exact: true }).click();
    await c.getByRole("button", { name: "End conversation", exact: true }).click();
    await expect(c.getByRole("button", { name: "Find someone", exact: true })).toBeVisible();
    await expect(a.getByRole("textbox")).toBeDisabled();
    expect(await a.evaluate(() => localStorage.length + sessionStorage.length)).toBe(0);
    expect(pageErrors).toEqual([]);
  } finally {
    await Promise.all([first.close(), second.close(), third.close()]);
  }
});
