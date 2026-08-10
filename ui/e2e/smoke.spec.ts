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
  const sessions = await rpc<{ sessions: { id: string }[] }>(page, "session/list");
  expect(sessions.sessions).toHaveLength(1);
  const sessionID = sessions.sessions[0].id;
  const msgs = await rpc<{ messages: { role: string; content: string }[] }>(page, "session/messages", { session_id: sessionID });
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
