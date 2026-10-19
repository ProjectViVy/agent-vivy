// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PluginHostProvider, type FaceStoreState, type FullUIHost } from '@vivy/ui-sdk';
import { NotebookView } from './view';
import { REPORT_ACTIONS } from './types';
import type { EntryView, Section } from './types';

const SECTIONS: readonly Section[] = [
  { id: 'section-notes', title: 'Notes', system_role: 'notes', version: 1, created_at: 1, updated_at: 1 },
];

function reportEntry(): EntryView {
  return {
    entry: {
      id: 'entry-r1', section_id: 'section-notes', kind: 'report', title: 'Daily report',
      head_revision_id: 'rev-1', version: 1,
      report_series_id: 'series-daily', report_window_id: 'win-1',
      created_at: 1, updated_at: 1,
    },
    revision: {
      id: 'rev-1', entry_id: 'entry-r1', sequence: 1, title: 'Daily report',
      markdown: '# report', origin: 'generated', actor: 'report/v1', created_at: 1,
    },
  };
}

function noteEntry(): EntryView {
  return {
    entry: {
      id: 'entry-note', section_id: 'section-notes', kind: 'note', title: 'Note',
      head_revision_id: 'rev-note', version: 1, created_at: 1, updated_at: 2,
    },
    revision: {
      id: 'rev-note', entry_id: 'entry-note', sequence: 1, title: 'Note',
      markdown: '# note', origin: 'human', actor: 'local:operator', created_at: 1,
    },
  };
}

function generatedEntry(): EntryView {
  return {
    entry: {
      id: 'entry-new', section_id: 'section-notes', kind: 'report', title: 'Daily report',
      head_revision_id: 'rev-new', version: 1,
      report_series_id: 'series-daily', report_window_id: 'win-1',
      created_at: 2, updated_at: 2,
    },
    revision: {
      id: 'rev-new', entry_id: 'entry-new', sequence: 1, title: 'Daily report',
      markdown: '# generated', origin: 'generated', actor: 'report/v1', created_at: 2,
    },
  };
}

function dailyEntry(): EntryView {
  return {
    entry: {
      id: 'entry-daily', section_id: 'section-daily', kind: 'report', title: 'Daily report',
      head_revision_id: 'rev-daily', version: 1,
      report_series_id: 'series-daily', report_window_id: 'win-daily',
      created_at: 3, updated_at: 3,
    },
    revision: {
      id: 'rev-daily', entry_id: 'entry-daily', sequence: 1, title: 'Daily report',
      markdown: '# daily', origin: 'generated', actor: 'report/v1', created_at: 3,
    },
  };
}

const GENERATION = {
  run_id: 'run-1',
  scope: 's',
  series_id: 'series-daily',
  window_id: 'win-1',
  config_revision: 1,
  timezone: 'UTC',
  start_ms: 0,
  end_ms: 0,
  as_of_ms: 0,
  input_digest: 'input',
  facts_digest: 'facts',
  outcome_mode: 'fallback',
  entry_id: 'entry-new',
  revision_id: 'rev-new',
};

interface ReportStub {
  generate: (input?: unknown) => unknown;
  get: (input?: unknown) => unknown;
  cancel: (input?: unknown) => unknown;
  settings: (input?: unknown) => unknown;
  /** When set, reports calls throw this instead of returning an envelope. */
  throwOn?: string;
}

function makeReports(overrides: Partial<ReportStub> = {}): ReportStub {
  return {
    generate: vi.fn(async () => ({ run_id: 'run-1', created: true, rejoined: false, busy: false })),
    get: vi.fn(async () => ({ run_id: 'run-1', status: 'completed', generation: { entry_id: 'entry-r1', revision_id: 'rev-2' } })),
    cancel: vi.fn(async () => ({})),
    settings: vi.fn(async () => ({ scope: 's', period: 'daily', timezone: 'UTC', section_id: 'x', enabled: false, revision: 1 })),
    ...overrides,
  };
}

function makeHost(
  reports: ReportStub,
  notebookFail = false,
  notebookOverrides: Record<string, (input?: unknown) => unknown> = {},
): FullUIHost {
  const state = () => ({ activeSessionId: 's', currentRun: null, connection: 'connected' }) as unknown as FaceStoreState;
  const notebook: Record<string, (input?: unknown) => unknown> = {
    'vivy.notebook.sections.list': async () => ({ sections: SECTIONS, next_cursor: '' }),
    'vivy.notebook.entries.list': async () => ({ entries: [reportEntry().entry], next_cursor: '' }),
    'vivy.notebook.entries.get': async () => reportEntry(),
    'vivy.notebook.revisions.list': async () => ({ revisions: [], next_cursor: '' }),
    'vivy.notebook.comments.list': async () => ({ comments: [], next_cursor: '' }),
    ...notebookOverrides,
  };
  const reportMap: Record<string, keyof ReportStub> = {
    [REPORT_ACTIONS.generate]: 'generate',
    [REPORT_ACTIONS.get]: 'get',
    [REPORT_ACTIONS.cancel]: 'cancel',
    [REPORT_ACTIONS.settingsRead]: 'settings',
  };
  const rpc = {
    capabilities: { protocol_version: 'vivy.rpc.v1', capabilities: ['module.action.invoke'] },
    call: vi.fn(async (_method: string, params?: { module_id?: string; action_id?: string; input?: unknown }) => {
      const action = params?.action_id ?? '';
      const reportFn = reportMap[action];
      if (reportFn) {
        if (reports.throwOn === 'all' || reports.throwOn === action) {
          const cause = new Error('action not found') as Error & { code: number };
          cause.code = -32004;
          throw cause;
        }
        const fn = reports[reportFn] as (input?: unknown) => unknown;
        try {
          return { status: 'ok', result: await fn(params?.input) };
        } catch (cause) {
          const typed = cause as { code?: string; message?: string; retryable?: boolean };
          return { status: 'error', error: { code: typed.code ?? 'storage_unavailable', message: typed.message ?? String(cause), retryable: typed.retryable ?? true } };
        }
      }
      const fn = notebook[action];
      if (!fn || notebookFail) {
        return { status: 'error', error: { code: 'invalid_request', message: `no stub for ${action}`, retryable: false } };
      }
      return { status: 'ok', data: await fn(params?.input) };
    }),
    onNotification: vi.fn(() => () => undefined),
    onClose: vi.fn(() => () => undefined),
    close: vi.fn(),
  } as unknown as FullUIHost['rpc'];
  return {
    api: {} as FullUIHost['api'],
    rpc,
    store: { getState: state, getInitialState: state, setState: vi.fn(), subscribe: () => () => undefined },
    router: { navigate: vi.fn().mockResolvedValue(undefined), invalidate: vi.fn().mockResolvedValue(undefined) },
    composition: {} as FullUIHost['composition'],
    t: (key) => key,
    registerCleanup: () => ({ active: true, dispose: vi.fn() }),
  };
}

async function flush() {
  await act(async () => { await Promise.resolve(); });
}

/** Drain the report submission → status read → destination resolution chain. */
async function settle(rounds = 6) {
  for (let index = 0; index < rounds; index += 1) await flush();
}

describe('NotebookView reports panel', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    vi.useRealTimers();
  });

  async function mount(
    reports: ReportStub,
    overrides: Record<string, (input?: unknown) => unknown>,
    selectRow: boolean,
  ) {
    const host = makeHost(reports, false, overrides);
    await act(async () => root.render(<PluginHostProvider host={host}><NotebookView /></PluginHostProvider>));
    await flush();
    if (selectRow) {
      // The selected-entry panel lives in the detail header: select the row first.
      const row = container.querySelector<HTMLButtonElement>('[data-testid="notebook-entry"]');
      if (row) {
        await act(async () => row.click());
        await flush();
      }
    }
    return { host, reports };
  }

  async function render(reports = makeReports()) {
    return mount(reports, {}, true);
  }

  /** First use: no entry is selected, so the panel renders in the empty detail. */
  async function renderEmpty(
    reports = makeReports(),
    overrides: Record<string, (input?: unknown) => unknown> = {},
  ) {
    return mount(reports, overrides, false);
  }

  it('offers generation and settings before the first entry', async () => {
    let published = false;
    const reports = makeReports({
      generate: vi.fn(async () => {
        published = true;
        return { run_id: 'run-1', created: true, rejoined: false, busy: false };
      }),
      get: vi.fn(async () => ({ run_id: 'run-1', status: 'completed', generation: GENERATION })),
    });
    await renderEmpty(reports, {
      'vivy.notebook.entries.list': async () => ({
        entries: published ? [generatedEntry().entry] : [],
        next_cursor: '',
      }),
      'vivy.notebook.entries.get': async (input?: unknown) => {
        const id = (input as { id?: string } | undefined)?.id;
        return id === 'entry-new' ? generatedEntry() : reportEntry();
      },
    });

    expect(container.querySelector('[data-testid="notebook-entries-empty"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="notebook-empty-reports"]')).not.toBeNull();
    expect(container.querySelectorAll('[data-testid="notebook-reports"]')).toHaveLength(1);
    expect(container.querySelector('[data-testid="report-settings-open"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="notebook-editor"]')).toBeNull();

    await act(async () => container.querySelector<HTMLButtonElement>('[data-testid="report-generate"]')!.click());
    await settle();

    const input = (reports.generate as ReturnType<typeof vi.fn>).mock.calls[0][0] as Record<string, unknown>;
    expect(input.period).toBe('daily');
    expect(input.target).toBeUndefined();

    // The authoritative destination entry is selected and its provenance shows.
    expect(container.querySelector('[data-testid="notebook-editor"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="notebook-empty-reports"]')).toBeNull();
    expect(container.querySelector('[data-testid="report-provenance"]')).not.toBeNull();
  });

  it('refreshes the report destination after empty-state generation', async () => {
    let published = false;
    const dailySection: Section = {
      id: 'section-daily', title: 'Daily', system_role: 'daily', version: 1, created_at: 2, updated_at: 2,
    };
    const reports = makeReports({
      generate: vi.fn(async () => {
        published = true;
        return { run_id: 'run-1', created: true, rejoined: false, busy: false };
      }),
      get: vi.fn(async () => ({
        run_id: 'run-1', status: 'completed',
        generation: { ...GENERATION, entry_id: 'entry-daily', revision_id: 'rev-daily', window_id: 'win-daily' },
      })),
    });
    await renderEmpty(reports, {
      'vivy.notebook.sections.list': async () => ({
        sections: published ? [SECTIONS[0], dailySection] : [SECTIONS[0]],
        next_cursor: '',
      }),
      'vivy.notebook.entries.list': async (input?: unknown) => {
        const section = (input as { section_id?: string } | undefined)?.section_id;
        return {
          entries: published && section === 'section-daily' ? [dailyEntry().entry] : [],
          next_cursor: '',
        };
      },
      'vivy.notebook.entries.get': async (input?: unknown) => {
        const id = (input as { id?: string } | undefined)?.id;
        return id === 'entry-daily' ? dailyEntry() : reportEntry();
      },
    });

    expect(container.querySelectorAll('[data-testid="notebook-reports"]')).toHaveLength(1);
    await act(async () => container.querySelector<HTMLButtonElement>('[data-testid="report-generate"]')!.click());
    await settle();

    // The generation output names a section the picker had not loaded; the
    // view resolves it through the authoritative entry instead of assuming
    // the currently filtered section.
    expect(container.querySelector<HTMLSelectElement>('[data-testid="notebook-section-select"]')!.value).toBe('section-daily');
    expect(container.querySelectorAll('[data-testid="notebook-entry"]')).toHaveLength(1);
    expect(container.querySelector('[data-testid="notebook-editor"]')).not.toBeNull();
    expect(container.querySelector<HTMLTextAreaElement>('[data-testid="notebook-draft"]')!.value).toBe('# daily');
    expect(container.querySelectorAll('[data-testid="notebook-reports"]')).toHaveLength(1);
  });

  it('keeps the empty-state panel hidden when the reports capability is absent', async () => {
    await renderEmpty(makeReports({ throwOn: 'all' }), {
      'vivy.notebook.entries.list': async () => ({ entries: [], next_cursor: '' }),
    });
    expect(container.querySelector('[data-testid="notebook-empty-reports"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="notebook-reports"]')).toBeNull();
    expect(container.textContent).toContain('plugin.vivy/notebook.emptySection');
  });

  it('keeps generation available after a settings read error and recovers settings on the next probe', async () => {
    let reads = 0;
    const reports = makeReports({
      settings: vi.fn(async () => {
        reads += 1;
        if (reads === 1) {
          throw Object.assign(new Error('settings read failed'), { code: 'storage_unavailable', retryable: true });
        }
        return { scope: 's', period: 'weekly', timezone: 'UTC', section_id: 'section-notes', enabled: false, revision: 1 };
      }),
    });
    await renderEmpty(reports, {
      'vivy.notebook.entries.list': async () => ({ entries: [], next_cursor: '' }),
    });

    expect(container.querySelector('[data-testid="notebook-reports"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="report-generate"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="report-settings-open"]')).toBeNull();

    const select = container.querySelector<HTMLSelectElement>('[data-testid="report-period"]')!;
    await act(async () => {
      select.value = 'weekly';
      select.dispatchEvent(new Event('change', { bubbles: true }));
    });
    await settle();

    expect(reads).toBe(2);
    expect(container.querySelector('[data-testid="report-settings-open"]')).not.toBeNull();
  });

  it('does not replace a dirty editor when a report completes', async () => {
    let published = false;
    const reports = makeReports({
      generate: vi.fn(async () => {
        published = true;
        return { run_id: 'run-1', created: true, rejoined: false, busy: false };
      }),
      get: vi.fn(async () => ({ run_id: 'run-1', status: 'completed', generation: GENERATION })),
    });
    await mount(reports, {
      'vivy.notebook.entries.list': async () => ({
        entries: published ? [noteEntry().entry, generatedEntry().entry] : [noteEntry().entry],
        next_cursor: '',
      }),
      'vivy.notebook.entries.get': async (input?: unknown) => {
        const id = (input as { id?: string } | undefined)?.id;
        return id === 'entry-new' ? generatedEntry() : noteEntry();
      },
    }, true);

    const draft = container.querySelector<HTMLTextAreaElement>('[data-testid="notebook-draft"]')!;
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!;
      setter.call(draft, 'unsaved draft');
      draft.dispatchEvent(new Event('input', { bubbles: true }));
    });
    await flush();

    await act(async () => container.querySelector<HTMLButtonElement>('[data-testid="report-generate"]')!.click());
    await settle();

    // Completion refreshed the list but must not steal the operator's buffer.
    expect(container.querySelectorAll('[data-testid="notebook-entry"]')).toHaveLength(2);
    expect(container.querySelector<HTMLTextAreaElement>('[data-testid="notebook-draft"]')!.value).toBe('unsaved draft');
    const inputs = (reports.generate as ReturnType<typeof vi.fn>).mock.calls[0][0] as Record<string, unknown>;
    expect(inputs.target).toBeUndefined();
  });

  it('hides the panel when the reports capability is absent', async () => {
    await render(makeReports({ throwOn: 'all' }));
    expect(container.querySelector('[data-testid="notebook-reports"]')).toBeNull();
    expect(container.querySelector('[data-testid="notebook-entry"]')).not.toBeNull();
  });

  it('admits a run with a stable operation key and report-entry target', async () => {
    const { reports } = await render();
    await flush();
    const button = container.querySelector<HTMLButtonElement>('[data-testid="report-generate"]');
    expect(button).not.toBeNull();
    await act(async () => button!.click());
    await flush();
    expect(reports.generate).toHaveBeenCalledTimes(1);
    const input = (reports.generate as ReturnType<typeof vi.fn>).mock.calls[0][0] as Record<string, unknown>;
    expect(input.period).toBe('daily');
    expect(input.window).toBe('completed');
    expect(typeof input.operation_key).toBe('string');
    expect(input.target).toEqual({ entry_id: 'entry-r1' });
    expect(input).not.toHaveProperty('scope');
    expect(input).not.toHaveProperty('actor');
    // Terminal result from the first status read fires the refresh callback.
    const badge = container.querySelector('[data-testid="report-status"]');
    expect(badge?.textContent).toContain('completed');
  });

  it('ignores duplicate clicks while a run is active', async () => {
    const reports = makeReports({
      get: vi.fn(async () => ({ run_id: 'run-1', status: 'active' })),
    });
    await render(reports);
    await flush();
    const button = container.querySelector<HTMLButtonElement>('[data-testid="report-generate"]')!;
    await act(async () => button.click());
    await flush();
    await act(async () => button.click());
    await flush();
    expect(reports.generate).toHaveBeenCalledTimes(1);
    const cancel = container.querySelector('[data-testid="report-cancel"]');
    expect(cancel).not.toBeNull();
  });

  it('surfaces a busy admission without minting a second run', async () => {
    const reports = makeReports({
      generate: vi.fn(async () => ({ run_id: 'run-9', created: false, rejoined: true, busy: true })),
      get: vi.fn(async () => ({ run_id: 'run-9', status: 'active' })),
    });
    await render(reports);
    await flush();
    await act(async () => container.querySelector<HTMLButtonElement>('[data-testid="report-generate"]')!.click());
    await flush();
    expect(container.textContent).toContain('reports.busy');
    expect(reports.generate).toHaveBeenCalledTimes(1);
  });

  it('reuses the same operation key after a lost acknowledgement', async () => {
    const retryable = Object.assign(new Error('request interrupted'), { code: 'outcome_unknown', retryable: true });
    const reports = makeReports({
      generate: vi.fn()
        .mockRejectedValueOnce(retryable)
        .mockResolvedValueOnce({ run_id: 'run-1', created: false, rejoined: true, busy: false }),
      get: vi.fn(async () => ({ run_id: 'run-1', status: 'completed', generation: { entry_id: 'e', revision_id: 'r' } })),
    });
    await render(reports);
    await flush();
    const button = container.querySelector<HTMLButtonElement>('[data-testid="report-generate"]')!;
    await act(async () => button.click());
    await flush();
    expect(container.textContent).toContain('retryAfterLostAck');
    await act(async () => button.click());
    await flush();
    const keys = (reports.generate as ReturnType<typeof vi.fn>).mock.calls.map(
      (call) => (call[0] as Record<string, unknown>).operation_key,
    );
    expect(keys).toHaveLength(2);
    expect(keys[0]).toBe(keys[1]);
  });

  it('cancels the admitted run by run id', async () => {
    const reports = makeReports({
      get: vi.fn(async () => ({ run_id: 'run-1', status: 'active' })),
    });
    await render(reports);
    await flush();
    await act(async () => container.querySelector<HTMLButtonElement>('[data-testid="report-generate"]')!.click());
    await flush();
    await act(async () => container.querySelector<HTMLButtonElement>('[data-testid="report-cancel"]')!.click());
    await flush();
    expect(reports.cancel).toHaveBeenCalledWith({ run_id: 'run-1' });
    expect(container.querySelector('[data-testid="report-status"]')?.textContent).toContain('cancelled');
  });
});
