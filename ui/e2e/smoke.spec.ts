// D4(b): UI smoke against the real Go process. The server under test is
// ui/../vivy.exe with a generated mock-provider config, so the whole
// stack — JSON-RPC, journal replay, embedded UI — runs for real;
// only the provider is deterministic.

import { expect, test } from "@playwright/test";

test("mock conversation streams to completion and survives a reload", async ({ page }) => {
  // 1. Load: the embedded UI shell renders.
  await page.goto("/");
  await expect(page.locator("#app")).toBeVisible();
  await expect(page.locator("#composer-input")).toBeVisible();
  await expect(page.locator("#session-list .session-item")).toHaveCount(0);

  // 2. New session appears and gets selected.
  await page.click("#new-session");
  await expect(page.locator("#session-list .session-item")).toHaveCount(1);

  // The global Review Center is available before any review is created and
  // returns to the same conversation without losing composer state.
  await expect(page.locator("#studio-button")).toBeVisible();
  await page.click("#review-center-button");
  await expect(page.locator("#review-center")).toBeVisible();
  await expect(page.locator("#review-center")).toContainText(/No pending reviews|目前没有待处理审查项/);
  await page.getByRole("button", { name: /Back to chat|返回对话/ }).click();
  await expect(page.locator("#chat")).toBeVisible();

  // 3. Send a message through the composer.
  await page.fill("#composer-input", "hello vivy");
  await page.click("#composer-send");

  // 4. The deterministic mock reply streams to a completed run.
  await expect(page.locator("#run-status")).toHaveText(/Completed|已完成/, { timeout: 15_000 });
  await expect(page.locator("#chat .message-user .message-content")).toHaveText("hello vivy");
  await expect(page.locator("#chat .message-assistant .message-content")).toHaveText("mock reply to: hello vivy");
  await expect(page.locator("#chat .message-run-link")).toHaveCount(2);
  await expect(page.locator("#activity-heading")).toBeHidden();
  await expect(page.locator(".child-runs-card")).toBeVisible();
  await expect(page.locator(".child-runs-card .inline-error")).toHaveCount(0);
  await expect(page.locator(".child-runs-card")).toContainText(/no child|没有子任务/i);

  // 5. Persistence: the API holds the full exchange (>= 2 messages).
  const sessions = await rpc<{ sessions: { id: string }[] }>(page, "session/list");
  expect(sessions.sessions).toHaveLength(1);
  const sessionID = sessions.sessions[0].id;
  const msgs = await rpc<{ messages: { role: string; content: string }[] }>(page, "session/messages", { session_id: sessionID });
  expect(msgs.messages.length).toBeGreaterThanOrEqual(2);
  expect(msgs.messages[0]).toMatchObject({ role: "user", content: "hello vivy" });
  expect(msgs.messages[1]).toMatchObject({ role: "assistant", content: "mock reply to: hello vivy" });

  // 6. Reload: the history comes back complete from storage.
  await page.reload();
  await expect(page.locator("#session-list .session-item")).toHaveCount(1);
  await page.locator("#session-list .session-row-main").first().click();
  await expect(page.locator("#chat .message")).toHaveCount(2);
  await expect(page.locator("#chat .message-user .message-content")).toHaveText("hello vivy");
  await expect(page.locator("#chat .message-assistant .message-content")).toHaveText("mock reply to: hello vivy");

  // 7. The real Go process can suspend an Eino tool call and expose it in
  // the cross-session Review Center. The fixture provider is deterministic;
  // storage, runtime, JSON-RPC, journal replay, and UI are not mocked.
  await page.click("#new-session");
  await page.fill("#composer-input", "e2e approval: save a note");
  await page.click("#composer-send");
  await continueAfterPreflight(page);
  await expect(page.locator("#run-status")).toHaveText(/Running|运行中|Waiting|等待/, { timeout: 15_000 });
  await page.click("#review-center-button");
  await expect(page.locator("#review-center")).toBeVisible();
  await expect(page.locator(".review-list-item")).toHaveCount(1, { timeout: 15_000 });
  await expect(page.locator("#review-center-title")).toBeFocused();
  await expect(page.locator("#review-center .review-card-approval")).toContainText(/write_note|e2e approval note/);
  await expect(page.locator("#review-center .review-arguments")).toContainText("e2e approval note");
  await page.keyboard.press("Tab");
  await page.keyboard.press("Tab");
  await page.keyboard.press("Tab");
  await expect(page.locator(".review-list-item")).toBeFocused();
  await page.locator("#review-center").getByRole("button", { name: /Approve write_note|批准 write_note/ }).click();
  await expect(page.locator(".review-list-item")).toHaveCount(0, { timeout: 15_000 });
  await expect(page.locator("#run-status")).toHaveText(/Completed|已完成/, { timeout: 15_000 });

  // A question is a separate review kind and uses an answer-only action.
  await page.getByRole("button", { name: /Back to chat|返回对话/ }).click();
  await page.click("#new-session");
  await page.fill("#composer-input", "e2e question");
  await page.click("#composer-send");
  await continueAfterPreflight(page);
  await page.click("#review-center-button");
  await expect(page.locator(".review-list-item")).toHaveCount(1, { timeout: 15_000 });
  await expect(page.locator("#review-center .review-card-question")).toContainText(/Which color should I use|哪种颜色/);
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator(".review-center-grid")).toHaveCSS("grid-template-columns", /^(\d+(\.\d+)?px)$/);
  await page.locator("#review-center .review-answer").fill("blue");
  await page.locator("#review-center").getByRole("button", { name: /Answer Vivy needs your answer|回答 需要你的回答/ }).click();
  await expect(page.locator(".review-list-item")).toHaveCount(0, { timeout: 15_000 });
  await expect(page.locator("#run-status")).toHaveText(/Completed|已完成/, { timeout: 15_000 });

  // 8. Locale/theme preferences and the narrow-screen inspector remain real UI state.
  await page.getByRole("button", { name: /Back to chat|返回对话/ }).click();
  await page.setViewportSize({ width: 1280, height: 900 });
  const initialLocale = await page.locator("html").getAttribute("lang");
  await page.click("#locale-button");
  await expect(page.locator("html")).not.toHaveAttribute("lang", initialLocale ?? "");
  await page.click("#locale-button");
  await page.click("#theme-button");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("vivy.theme"))).toBe("light");
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator("#main")).toBeVisible();
  await expect(page.locator("#inspector")).toBeVisible();
  await page.click("#inspector-close");
  await expect(page.locator("#main")).toBeVisible();

  // 9. Low-frequency session management lives behind the shared menu/dialog.
  await page.setViewportSize({ width: 1280, height: 900 });
  const sessionCountBeforeManagement = await page.locator("#session-list .session-item").count();
  await page.click("#new-session");
  await expect(page.locator("#session-list .session-item")).toHaveCount(sessionCountBeforeManagement + 1);
  const newest = page.locator("#session-list .session-item").first();
  await newest.locator(".session-more").click();
  await page.locator(".floating-menu .menu-item").first().click();
  await page.fill("#dialog-root input", "Renamed session");
  await page.click("#dialog-root button[type=submit]");
  await expect(newest).toContainText("Renamed session");
  await newest.locator(".session-more").click();
  await page.locator(".floating-menu .menu-item.is-danger").click();
  await page.click("#dialog-root .button-danger");
  await expect(page.locator("#session-list .session-item")).toHaveCount(sessionCountBeforeManagement);
});

async function continueAfterPreflight(page: import("@playwright/test").Page): Promise<void> {
  const continueButton = page.getByRole("button", { name: /Continue|继续发送/ }).last();
  await expect(continueButton).toBeVisible({ timeout: 5_000 });
  await continueButton.click();
}

async function rpc<T>(page: import("@playwright/test").Page, method: string, params?: unknown): Promise<T> {
  return page.evaluate(async ({ method, params }) => {
    const bootstrap = (await (await fetch("/rpc/bootstrap", { cache: "no-store" })).json()) as {
      protocol_version: string;
      websocket_path: string;
      token: string;
    };
    const socket = await new Promise<WebSocket>((resolve, reject) => {
      const scheme = window.location.protocol === "https:" ? "wss:" : "ws:";
      const ws = new WebSocket(`${scheme}//${window.location.host}${bootstrap.websocket_path}?token=${encodeURIComponent(bootstrap.token)}`);
      ws.onopen = () => resolve(ws);
      ws.onerror = () => reject(new Error("RPC websocket failed"));
    });
    const call = (id: string, callMethod: string, callParams?: unknown) => {
      socket.send(JSON.stringify({ jsonrpc: "2.0", id, method: callMethod, params: callParams }));
    };
    call("init", "initialize", { protocol_version: bootstrap.protocol_version });
    await new Promise<void>((resolve, reject) => {
      socket.onmessage = (event) => {
        const message = JSON.parse(String(event.data)) as { id?: string; error?: { message: string } };
        if (message.id !== "init") return;
        if (message.error) reject(new Error(message.error.message));
        else resolve();
      };
    });
    call("request", method, params);
    const result = await new Promise<T>((resolve, reject) => {
      socket.onmessage = (event) => {
        const message = JSON.parse(String(event.data)) as { id?: string; result?: T; error?: { message: string } };
        if (message.id !== "request") return;
        if (message.error) reject(new Error(message.error.message));
        else resolve(message.result as T);
      };
    });
    socket.close();
    return result;
  }, { method, params });
}
