import { expect, test } from "@playwright/test";

test("studio promote applies at next launch only", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator("#app")).toBeVisible();
  await page.click("#studio-button");
  await expect(page.locator("#studio")).toBeVisible();
  await expect(page.locator("#studio-generation-id")).toHaveText("builtin");
  const binaryBefore = (await page.locator("#studio-binary-id").textContent()) ?? "";
  expect(binaryBefore.length).toBeGreaterThan(0);
  await expect(page.locator("#studio-tool-list")).toContainText("echo_info");
  const toolsBefore = (await page.locator("#studio-tool-list").textContent()) ?? "";

  const from = await rpc<{ id: string }>(page, "generations/create", {
    artifact_sha256: "from-sha",
    recipe: { loop: "eino", world: "sandbox" },
  });
  const to = await rpc<{ id: string }>(page, "generations/create", {
    parent_id: from.id,
    artifact_sha256: "to-sha",
    recipe: { loop: "eino", world: "sandbox", plugins: ["hello-fs"] },
  });
  await rpc(page, "evals/record", {
    candidate_id: to.id,
    baseline_id: from.id,
    suite: "airgap.probe",
    verdict: "mixed",
  });

  await page.getByRole("button", { name: /Refresh Studio|刷新工作室/ }).click();
  await page.locator(`.review-list-item[data-generation-id="${to.id}"]`).click();
  await expect(page.locator("#studio-selected-id")).toHaveText(to.id);
  await page.click("#studio-promote");
  await expect(page.locator("#studio-next-launch")).toContainText(/next launch|下次启动/);
  await expect(page.locator("#studio-generation-id")).toHaveText(to.id);
  await expect(page.locator("#studio-binary-id")).toHaveText(binaryBefore);
  await expect(page.locator("#studio-tool-list")).toHaveText(toolsBefore);
  await expect(page.locator("#studio-tool-list")).toContainText("echo_info");
  await expect(page.locator("#studio-tool-list")).not.toContainText("hello_stat");

  await page.getByRole("button", { name: /Back to chat|返回对话/ }).click();
  await expect(page.locator("#chat")).toBeVisible();
  await page.click("#new-session");
  await page.fill("#composer-input", "hello vivy");
  await page.click("#composer-send");
  await expect(page.locator("#run-status")).toHaveText(/Completed|已完成/, { timeout: 15_000 });

  const extra = await rpc<{ id: string }>(page, "generations/create", {
    parent_id: to.id,
    artifact_sha256: "extra-sha",
    recipe: { loop: "eino", world: "sandbox" },
  });
  await page.click("#studio-button");
  await page.getByRole("button", { name: /Refresh Studio|刷新工作室/ }).click();
  await page.locator(`.review-list-item[data-generation-id="${extra.id}"]`).click();
  await page.click("#studio-reject");
  await expect(page.locator("#studio-selected-phase")).toContainText("rejected");
  await expect(page.locator("#studio-generation-id")).toHaveText(to.id);
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
