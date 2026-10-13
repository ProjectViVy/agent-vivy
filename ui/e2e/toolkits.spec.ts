import { expect, test, type Page } from '@playwright/test';
import { createServer, type Server } from 'node:http';
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';

let fixtureRoot = '';
let releaseV1 = '';
let hookMarker = '';
let mcpServer: Server | undefined;
let mcpRequests = 0;
let mcpEndpoint = '';

function writeRelease(root: string, version: string, hookCommand: string): void {
  const files: Record<string, string> = {
    'skills/create-prd/SKILL.md': '---\ndescription: Create product requirements\n---\n# PRD workflow\nCreate a concise product requirements document.\n',
    'skills/implement-story/SKILL.md': '---\ndescription: Implement a reviewed story\n---\n# Implementation workflow\nImplement one reviewed story at a time.\n',
    'masks/pm.md': '# Product Manager\nClarify users, outcomes and acceptance before implementation.\n',
    'masks/dev.md': '# Developer\nPrefer small changes with focused evidence.\n',
    'rules/delivery.md': '# Delivery rule\nRecord verification evidence for each deliverable.\n',
    'resources/prd-template.md': '# Problem\n# Users\n# Success measures\n',
  };
  for (const [relative, content] of Object.entries(files)) {
    const target = path.join(root, relative);
    mkdirSync(path.dirname(target), { recursive: true });
    writeFileSync(target, content, 'utf8');
  }
  const manifest = {
    apiVersion: 'vivy.bundle/v1',
    id: 'example/bmad-experts',
    version,
    name: 'BMAD Experts',
    description: 'Product and engineering perspectives with independent Skills.',
    components: {
      skills: [
        { id: 'create-prd', path: 'skills/create-prd/SKILL.md' },
        { id: 'implement-story', path: 'skills/implement-story/SKILL.md' },
      ],
      masks: [
        { id: 'pm', name: 'Product Manager', path: 'masks/pm.md', recommendedSkills: ['create-prd'] },
        { id: 'dev', name: 'Developer', path: 'masks/dev.md', recommendedSkills: ['implement-story'] },
      ],
      rules: [{ id: 'delivery', path: 'rules/delivery.md', faces: ['code'] }],
      hooks: [{ id: 'before-tool', event: 'PreToolUse', command: hookCommand, matcher: '.*', cwd: 'bundle', timeoutMs: 1000 }],
      mcp: [{ id: 'docs', endpoint: mcpEndpoint, authEnv: 'DOCS_TOKEN', args: [], resourceBridge: false }],
    },
    resources: ['resources/prd-template.md'],
    licenseFiles: [],
  };
  writeFileSync(path.join(root, 'vivy-bundle.json'), `${JSON.stringify(manifest, null, 2)}\n`, 'utf8');
}

async function startMockMCP(): Promise<void> {
  mcpServer = createServer((request, response) => {
    mcpRequests += 1;
    let raw = '';
    request.setEncoding('utf8');
    request.on('data', (chunk) => { raw += chunk; });
    request.on('end', () => {
      let id: string | number | null = null;
      try { id = JSON.parse(raw).id ?? null; } catch { /* preserve a valid JSON-RPC error reply */ }
      response.writeHead(200, { 'Content-Type': 'application/json', 'Mcp-Session-Id': 'toolkits-e2e' });
      response.end(JSON.stringify({ jsonrpc: '2.0', id, result: { protocolVersion: '2024-11-05', capabilities: { tools: {} }, serverInfo: { name: 'toolkits-e2e', version: '1' } } }));
    });
  });
  await new Promise<void>((resolve, reject) => {
    mcpServer?.once('error', reject);
    mcpServer?.listen(0, '127.0.0.1', resolve);
  });
  const address = mcpServer?.address();
  if (!address || typeof address === 'string') throw new Error('mock MCP server did not bind a TCP port');
  mcpEndpoint = `http://127.0.0.1:${address.port}/mcp`;
}

async function openToolkits(page: Page): Promise<void> {
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/');
  const link = page.getByRole('link', { name: 'Toolkits', exact: true });
  if (!(await link.isVisible().catch(() => false))) await page.getByRole('button', { name: 'VIVY', exact: true }).click();
  await link.click();
  await expect(page.getByTestId('toolkit-page')).toBeVisible();
}

function componentSection(page: Page, heading: string) {
  return page.getByRole('heading', { name: heading, exact: true }).locator('xpath=../..');
}

async function catalogState(page: Page): Promise<{ epoch: number; configRevision: number }> {
  const state = page.getByTestId('catalog-epoch');
  await expect(state).toBeVisible();
  const text = await state.innerText();
  const match = text.match(/(\d+).*?(\d+)$/);
  if (!match) throw new Error(`catalog state is not visible: ${text}`);
  return { epoch: Number(match[1]), configRevision: Number(match[2]) };
}

test.beforeAll(async () => {
  fixtureRoot = mkdtempSync(path.join(tmpdir(), 'vivy-toolkit-e2e-'));
  hookMarker = path.join(fixtureRoot, 'hook-executed');
  releaseV1 = path.join(fixtureRoot, 'release-v1');
  await startMockMCP();
  writeRelease(releaseV1, '1.0.0', `printf 'v1' > '${hookMarker}'`);
});

test.afterAll(async () => {
  if (mcpServer) await new Promise<void>((resolve) => mcpServer?.close(() => resolve()));
  if (fixtureRoot) rmSync(fixtureRoot, { recursive: true, force: true });
});

test('installs native PM and Dev Masks, keeps Skills independent, and updates with separate consent', async ({ page }) => {
  await openToolkits(page);
  await expect(page.getByText('No Toolkits are installed.')).toBeVisible();
  const initial = await catalogState(page);

  const source = page.getByLabel('Host directory path');
  await source.fill(path.join(fixtureRoot, 'missing'));
  await page.getByRole('button', { name: 'Preview source', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Preview failed');

  await source.fill(releaseV1);
  await page.getByRole('button', { name: 'Preview source', exact: true }).click();
  const preview = page.getByTestId('toolkit-preview');
  await expect(preview).toContainText('BMAD Experts');
  await expect(preview).toContainText('Release SHA-256:');
  const firstDigestText = await preview.innerText();
  const firstDigest = firstDigestText.match(/Release SHA-256: ([a-f0-9]{64})/)?.[1];
  expect(firstDigest).toBeTruthy();
  await preview.getByRole('button', { name: 'Install disabled', exact: true }).click();

  await expect(page.getByRole('status').filter({ hasText: 'Toolkit installed disabled' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'BMAD Experts', exact: true })).toBeVisible();
  for (const heading of ['Skills', 'Masks', 'Rules', 'Hooks', 'MCP']) {
    await expect(componentSection(page, heading)).toBeVisible();
  }
  const pmRecommendation = page.locator('p').filter({ hasText: /Recommended Skills|推荐 Skill/ }).filter({ hasText: 'create-prd' });
  const devRecommendation = page.locator('p').filter({ hasText: /Recommended Skills|推荐 Skill/ }).filter({ hasText: 'implement-story' });
  await expect(pmRecommendation).toBeVisible();
  await expect(devRecommendation).toBeVisible();
  await expect(pmRecommendation).toContainText(/not enabled automatically|不会自动启用/);
  const consentButtons = page.getByRole('button', { name: 'Confirm authorization', exact: true });
  await expect(consentButtons).toHaveCount(2);
  await expect(consentButtons.first()).toBeDisabled();
  await expect(consentButtons.nth(1)).toBeDisabled();
  await page.getByRole('button', { name: 'View content', exact: true }).first().click();
  await expect(page.getByRole('dialog')).toContainText('Create a concise product requirements document');
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toBeHidden();
  expect(mcpRequests).toBe(0);

  await page.getByRole('button', { name: 'Prepare enable', exact: true }).click();
  const enable = page.getByTestId('prepared-candidate');
  await expect(enable).toBeVisible();
  await enable.getByRole('button', { name: 'Publish change', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Confirm authorization', exact: true }).first()).toBeEnabled();

  const mcp = componentSection(page, 'MCP');
  const mcpBindings = mcp.getByRole('form', { name: /Environment bindings.*mcp\/docs/ });
  await mcpBindings.getByLabel('authEnv').fill('VIVY_TOOLKIT_E2E_DOCS_TOKEN');
  await mcpBindings.getByRole('button', { name: 'Save bindings', exact: true }).click();
  await expect(page.getByRole('status').filter({ hasText: 'Host environment references saved' })).toBeVisible();
  await mcp.getByRole('button', { name: 'Confirm authorization', exact: true }).click();
  await expect(mcp).toContainText('authorized for current declaration');
  const hook = componentSection(page, 'Hooks');
  await hook.getByRole('button', { name: 'Confirm authorization', exact: true }).click();
  await expect(hook).toContainText('authorized for current declaration');
  expect(mcpRequests).toBe(0);
  await expect(page.getByTestId('catalog-epoch')).toContainText(`Catalog epoch ${initial.epoch + 1}`);

  writeRelease(releaseV1, '2.0.0', `printf 'v2' > '${hookMarker}'`);
  await source.fill(releaseV1);
  await page.getByRole('button', { name: 'Prepare update', exact: true }).click();
  const update = page.getByTestId('prepared-candidate');
  await expect(update).toContainText('hook/before-tool: changed');
  await expect(update).toContainText('fresh consent required');
  await update.getByRole('button', { name: 'Publish change', exact: true }).click();
  await expect(hook).toContainText('old consent is stale; confirm again');
  await expect(page.getByTestId('catalog-epoch')).toContainText(`Catalog epoch ${initial.epoch + 2}`);
  const updatedDigestText = await page.getByText(/Release SHA-256:/).innerText();
  const updatedDigest = updatedDigestText.match(/Release SHA-256: ([a-f0-9]{64})/)?.[1];
  expect(updatedDigest).toBeTruthy();
  expect(updatedDigest).not.toBe(firstDigest);
  await hook.getByRole('button', { name: 'Confirm current authorization', exact: true }).click();
  await expect(hook).toContainText('authorized for current declaration');

  const revisionAfterBinding = await catalogState(page);
  expect(revisionAfterBinding.configRevision).toBeGreaterThan(initial.configRevision);
  expect(mcpRequests).toBe(0);
  await page.getByRole('button', { name: 'Prepare disable', exact: true }).click();
  const disable = page.getByTestId('prepared-candidate');
  await disable.getByRole('button', { name: 'Publish change', exact: true }).click();
  await page.getByRole('button', { name: 'Prepare uninstall', exact: true }).click();
  const uninstall = page.getByTestId('prepared-candidate');
  await uninstall.getByRole('button', { name: 'Publish change', exact: true }).click();
  await expect(page.getByText('example/bmad-experts · orphaned')).toBeVisible();
  expect(mcpRequests).toBe(0);
});
