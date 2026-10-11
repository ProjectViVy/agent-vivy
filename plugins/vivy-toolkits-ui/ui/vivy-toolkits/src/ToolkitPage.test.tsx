// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ToolkitPage } from './ToolkitPage';
import type { ToolkitClient, ToolkitInstallation, ToolkitPageResult } from './toolkit-client';

const catalog = { epoch: 1, config_revision: 0, effective_digest: 'a'.repeat(64), generation_id: 'test-gen' };
const item: ToolkitInstallation = { install_id: '10000000-0000-4000-8000-000000000001', bundle_id: 'bmad/experts', state: 'enabled', source: { kind: 'git', origin: 'https://example.invalid/bmad' }, release_digest: 'b'.repeat(64), revision: 1, tombstone: false };
const page = (bundle: ToolkitInstallation): ToolkitPageResult => ({ items: [bundle], next_after_id: '', catalog, capabilities: { owners: { skill: false, mask: false, rule: false, hook: false, mcp: false } } });

describe('ToolkitPage', () => {
  let container: HTMLDivElement;
  let root: Root;
  beforeEach(() => { vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true); container = document.createElement('div'); document.body.append(container); root = createRoot(container); });
  afterEach(async () => { await act(async () => root.unmount()); container.remove(); vi.restoreAllMocks(); });

  it('ignores a stale Toolkit catalog response that arrives after refresh', async () => {
    const first = deferred<ToolkitPageResult>();
    const second = deferred<ToolkitPageResult>();
    const other = { ...item, install_id: '20000000-0000-4000-8000-000000000002', bundle_id: 'new/toolkit' };
    const client = { list: vi.fn().mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise), inspect: vi.fn() } as unknown as ToolkitClient;
    await act(async () => root.render(<ToolkitPage client={client} />));
    await act(async () => { window.dispatchEvent(new Event('focus')); await Promise.resolve(); });
    await act(async () => { second.resolve(page(other)); await Promise.resolve(); });
    await act(async () => { first.resolve(page(item)); await Promise.resolve(); });
    expect(container.textContent).toContain('new/toolkit');
    expect(container.textContent).not.toContain('bmad/experts');
  });

  it('shows unavailable component owners as staged capabilities in the dedicated page', async () => {
    const client = { list: vi.fn().mockResolvedValue(page(item)), inspect: vi.fn().mockResolvedValue({
      installation: item, release: { install_id: item.install_id, bundle_id: item.bundle_id, release_digest: item.release_digest, release_sequence: 1 }, releases: [],
      manifest: { id: 'bmad/experts', version: '1', name: 'BMAD Experts', components: { skills: [{ id: 'pm', path: 'skills/pm/SKILL.md' }], masks: [], rules: [], hooks: [], mcp: [] }, resources: [] }, consents: [], executables: [],
    }) } as unknown as ToolkitClient;
    await act(async () => root.render(<ToolkitPage client={client} />));
    await act(async () => { container.querySelector<HTMLButtonElement>('ul button')!.click(); await Promise.resolve(); });
    expect(container.textContent).toContain('当前构建暂不可用');
    expect(container.textContent).toContain('Skills');
    expect(container.textContent).toContain('面具');
    expect(container.textContent).toContain('Rules');
    expect(container.textContent).toContain('Hooks');
    expect(container.textContent).toContain('MCP');
  });
});

function deferred<T>() { let resolve!: (value:T)=>void; const promise=new Promise<T>((done)=>{resolve=done;}); return {promise,resolve}; }
