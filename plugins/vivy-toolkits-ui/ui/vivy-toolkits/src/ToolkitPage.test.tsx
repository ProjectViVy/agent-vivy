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
    // Without a host the translator returns the catalog key, so these assertions
    // also prove the page asks for a cataloged key instead of inlining copy.
    expect(container.textContent).toContain('plugin.vivy/toolkits-ui.ownerUnavailable');
    for (const kind of ['skills', 'masks', 'rules', 'hooks', 'mcp']) {
      expect(container.textContent).toContain(`plugin.vivy/toolkits-ui.${kind}`);
    }
  });

  it('keeps the library visible while a refresh is in flight', async () => {
    const slow = deferred<ToolkitPageResult>();
    const client = { list: vi.fn().mockResolvedValueOnce(page(item)).mockReturnValueOnce(slow.promise), inspect: vi.fn() } as unknown as ToolkitClient;
    await act(async () => root.render(<ToolkitPage client={client} />));
    expect(container.textContent).toContain('bmad/experts');
    await act(async () => { window.dispatchEvent(new Event('focus')); await Promise.resolve(); });
    // A refresh must not replace the list or add a second status region.
    expect(container.textContent).toContain('bmad/experts');
    expect(container.querySelectorAll('[role="status"]')).toHaveLength(0);
    const refresh = container.querySelector<HTMLButtonElement>('[data-action="refresh"]');
    expect(refresh?.getAttribute('aria-busy')).toBe('true');
    await act(async () => { slow.resolve(page(item)); await Promise.resolve(); });
    expect(refresh?.getAttribute('aria-busy')).toBe('false');
  });

  it('serializes catalog mutations so an in-flight change cannot resend a stale catalog', async () => {
    const slow = deferred<{ id: string; state: string; revision: number; kind: string }>();
    const inspection = {
      installation: item,
      release: { install_id: item.install_id, bundle_id: item.bundle_id, release_digest: item.release_digest, release_sequence: 1 },
      releases: [],
      manifest: {
        id: 'bmad/experts', version: '1', name: 'BMAD Experts', resources: [],
        components: { skills: [], masks: [], rules: [], hooks: [], mcp: [{ id: 'docs', endpoint: 'http://127.0.0.1:1/mcp', authEnv: 'authEnv' }] },
      },
      consents: [],
      executables: [{ kind: 'mcp', local_id: 'docs', release_digest: item.release_digest, declaration_digest: 'c'.repeat(64), consent_allowed: false, consent_current: false, missing_bindings: [], runtime_state: 'inactive', runtime_tool_count: -1 }],
      bindings: [],
    };
    const available = { ...page(item), capabilities: { owners: { skill: true, mask: true, rule: true, hook: true, mcp: true } } };
    const client = {
      list: vi.fn().mockResolvedValue(available),
      inspect: vi.fn().mockResolvedValue(inspection),
      setBindings: vi.fn().mockReturnValue(slow.promise),
      consent: vi.fn(),
    } as unknown as ToolkitClient;
    await act(async () => root.render(<ToolkitPage client={client} />));
    await act(async () => { container.querySelector<HTMLButtonElement>('ul button')!.click(); await Promise.resolve(); });
    const consent = container.querySelector<HTMLButtonElement>('[data-action="consent-grant"]');
    expect(consent?.disabled).toBe(false);
    const input = container.querySelector<HTMLInputElement>('#binding-mcp-docs-authEnv')!;
    await act(async () => { input.value = 'HOST_TOKEN'; input.dispatchEvent(new Event('input', { bubbles: true })); });
    await act(async () => { container.querySelector<HTMLFormElement>('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })); await Promise.resolve(); });
    // The catalog identity is shared: a second mutation cannot start while the
    // binding write is still in flight, or it would send the stale revision.
    expect(client.consent).not.toHaveBeenCalled();
    expect(container.querySelector<HTMLButtonElement>('[data-action="consent-grant"]')?.disabled).toBe(true);
    await act(async () => { slow.resolve({ id: 'op-bindings', state: 'committed', revision: 2, kind: 'bindings' }); await Promise.resolve(); await Promise.resolve(); });
    expect(container.querySelector<HTMLButtonElement>('[data-action="consent-grant"]')?.disabled).toBe(false);
  });
});

function deferred<T>() { let resolve!: (value:T)=>void; const promise=new Promise<T>((done)=>{resolve=done;}); return {promise,resolve}; }
