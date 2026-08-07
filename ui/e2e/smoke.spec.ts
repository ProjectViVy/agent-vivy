// D4(b): UI smoke against the real Go process. The server under test is
// ui/../vivy.exe with a generated mock-provider config, so the whole
// stack — HTTP API, SSE, journal replay, embedded UI — runs for real;
// only the provider is deterministic.

import { expect, test } from "@playwright/test";

test("mock conversation streams to completion and survives a reload", async ({ page, request }) => {
  // 1. Load: the embedded UI shell renders.
  await page.goto("/");
  await expect(page.locator("#app")).toBeVisible();
  await expect(page.locator("#composer-input")).toBeVisible();
  await expect(page.locator("#session-list li")).toHaveCount(0);

  // 2. New session appears and gets selected.
  await page.click("#new-session");
  await expect(page.locator("#session-list li")).toHaveCount(1);

  // 3. Send a message through the composer.
  await page.fill("#composer-input", "hello vivy");
  await page.click("#composer-send");

  // 4. The deterministic mock reply streams to a completed run.
  await expect(page.locator("#run-status")).toHaveText("run completed", { timeout: 15_000 });
  await expect(page.locator("#chat .bubble.user")).toHaveText("hello vivy");
  await expect(page.locator("#chat .bubble.assistant")).toHaveText("mock reply to: hello vivy");

  // 5. Persistence: the API holds the full exchange (>= 2 messages).
  const sessions = (await (await request.get("/api/sessions")).json()) as {
    sessions: { id: string }[];
  };
  expect(sessions.sessions).toHaveLength(1);
  const sessionID = sessions.sessions[0].id;
  const msgs = (await (await request.get(`/api/sessions/${sessionID}/messages`)).json()) as {
    messages: { role: string; content: string }[];
  };
  expect(msgs.messages.length).toBeGreaterThanOrEqual(2);
  expect(msgs.messages[0]).toMatchObject({ role: "user", content: "hello vivy" });
  expect(msgs.messages[1]).toMatchObject({ role: "assistant", content: "mock reply to: hello vivy" });

  // 6. Reload: the history comes back complete from storage.
  await page.reload();
  await expect(page.locator("#session-list li")).toHaveCount(1);
  await page.locator("#session-list li span").first().click();
  await expect(page.locator("#chat .bubble")).toHaveCount(2);
  await expect(page.locator("#chat .bubble.user")).toHaveText("hello vivy");
  await expect(page.locator("#chat .bubble.assistant")).toHaveText("mock reply to: hello vivy");
});
