import { expect, test, type Page } from '@playwright/test';
import type http from 'node:http';
import { startLoopbackModel } from './continuity-setup';
import { WORKFLOW_MODEL_ADDR } from './workflow-setup';

// Workflow correctness acceptance (issue #81 / E3): the deterministic loopback
// model drives the real backend + Vite split pair on isolated ports. The
// assertions read committed host state, never a mocked callback.
const WORKFLOW_ID = 'pg-workflow';
const AUTHORED_INPUTS = {
  literal_text: { literal: 'PG-literal-value' },
  root_text: { source: 'input', pointer: '/topic' },
};

let modelServer: http.Server;
test.beforeAll(async () => {
  modelServer = await startLoopbackModel(WORKFLOW_MODEL_ADDR);
});
test.afterAll(() => {
  modelServer.close();
});
test.describe.configure({ mode: 'serial' });

type Json = Record<string, unknown>;

async function rpc(page: Page, method: string, params: Json): Promise<Json> {
  // `inofy.*` handlers require the same live session the bridge injects.
  const sessionId = await page.evaluate(() => localStorage.getItem('vivy.ui.activeSession'));
  if (!sessionId) throw new Error(`no active session for ${method}`);
  return page.evaluate(
    async ({ method, params }) => {
      const bootstrap = (await (await fetch('/rpc/bootstrap', { cache: 'no-store' })).json()) as {
        websocket_path: string;
        token: string;
      };
      const wsURL = new URL(bootstrap.websocket_path, location.origin);
      wsURL.searchParams.set('token', bootstrap.token);
      const socket = await new Promise<WebSocket>((resolve, reject) => {
        const candidate = new WebSocket(wsURL.toString());
        candidate.onopen = () => resolve(candidate);
        candidate.onerror = () => reject(new Error('websocket connect failed'));
      });
      try {
        return await new Promise<Json>((resolve, reject) => {
          socket.onmessage = (event) => {
            const reply = JSON.parse(String(event.data)) as {
              id?: string;
              result?: Json;
              error?: { code: number; message: string };
            };
            if (reply.error) reject(new Error(`rpc ${method}: ${reply.error.code} ${reply.error.message}`));
            else if (reply.id) resolve(reply.result ?? {});
          };
          socket.send(JSON.stringify({ jsonrpc: '2.0', id: 'e2e', method, params }));
          setTimeout(() => reject(new Error(`rpc ${method} timeout`)), 15_000);
        });
      } finally {
        socket.close();
      }
    },
    { method, params: { ...params, session_id: sessionId } },
  );
}

const inputsField = (page: Page) => page.locator('#wf-node-inputs');

// The first-run welcome dialog covers the whole shell; mark it completed before
// the app boots so the workflow page is directly operable.
test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('vivy.ui.welcome.completed', '1');
    localStorage.setItem('vivy.persona.onboarding.dismissed', '1');
  });
});

async function openSeededDraft(page: Page): Promise<void> {
  await page.goto('/workflows');
  await expect(page.getByRole('tab', { name: 'Editor' })).toBeVisible();
  await page.getByPlaceholder('workflow id').fill(WORKFLOW_ID);
  await page.getByRole('button', { name: 'Open', exact: true }).click();
  await expect(page.getByText(/No draft found/)).toBeVisible();
  await page.locator('.react-flow__node').first().click();
  await expect(inputsField(page)).toBeVisible();
}

test('authored node inputs survive save, reload, validate, and publish', async ({ page }) => {
  await openSeededDraft(page);

  await inputsField(page).fill(JSON.stringify(AUTHORED_INPUTS, null, 2));
  await inputsField(page).blur();
  await expect(page.getByText(/inputs\[|inputs must be/)).toHaveCount(0);
  await page.getByRole('button', { name: 'Save draft' }).click();
  await expect(page.getByText('Draft saved.')).toBeVisible();

  // Re-open from the backend: the draft value must come back from storage,
  // not from the panel's local state.
  await page.getByRole('button', { name: 'Open', exact: true }).click();
  await page.locator('.react-flow__node').first().click();
  await expect(inputsField(page)).toBeVisible();
  expect(JSON.parse((await inputsField(page).inputValue()) as string)).toEqual(AUTHORED_INPUTS);

  await page.getByRole('button', { name: 'Validate' }).click();
  await expect(page.getByText('Definition is valid.')).toBeVisible();
  await page.getByRole('button', { name: 'Publish' }).click();
  await expect(page.getByText(/Published r1/)).toBeVisible();

  const revision = (await rpc(page, 'inofy.getRevision', { workflow: WORKFLOW_ID, revision: 1 })) as {
    artifact: {
      definition: { graph: { nodes: Array<{ id: string; config?: Record<string, unknown>; inputs?: unknown }> } };
    };
  };
  const node = revision.artifact.definition.graph.nodes.find((candidate) => candidate.id === 'begin');
  if (!node) throw new Error('published artifact lost the begin node');
  expect(node.inputs).toEqual(AUTHORED_INPUTS);
  expect(node.config?.inputs).toBeUndefined();
  expect(node.config?.task).toBe(WORKFLOW_ID);
});
