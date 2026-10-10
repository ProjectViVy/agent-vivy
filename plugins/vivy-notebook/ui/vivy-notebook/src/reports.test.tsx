import { beforeEach, describe, expect, it } from 'vitest';
import { NotebookError, ReportsClient, type NotebookActionTransport } from './api';
import {
  NOTEBOOK_MODULE_ID, REPORT_ACTIONS, REPORTS_MODULE_ID,
  type ReportOutcome,
} from './types';

function transport(handler: (req: { moduleId: string; actionId: string; input: unknown }) => unknown): NotebookActionTransport {
  return {
    invoke: async (request) => handler(request) as never,
  };
}

const calls: { moduleId: string; actionId: string; input: unknown }[] = [];

beforeEach(() => {
  calls.length = 0;
});

describe('ReportsClient', () => {
  it('pins the sealed generate envelope (period/window/op_key/target, never scope/actor)', async () => {
    const actions = transport((req) => {
      calls.push(req);
      return { status: 'ok', result: { run_id: 'run-1', created: true, rejoined: false, busy: false } } satisfies ReportOutcome<unknown>;
    });
    const client = new ReportsClient(actions);
    const admission = await client.generate({
      period: 'daily', window: 'completed', operationKey: 'op-1',
      target: { entry_id: 'entry-9' },
    });
    expect(admission.run_id).toBe('run-1');
    expect(calls).toEqual([{
      moduleId: REPORTS_MODULE_ID,
      actionId: REPORT_ACTIONS.generate,
      input: { operation_key: 'op-1', period: 'daily', window: 'completed', target: { entry_id: 'entry-9' } },
    }]);
  });

  it('pins get/cancel/settings.read action ids and request shapes', async () => {
    const actions = transport((req) => {
      calls.push(req);
      if (req.actionId === REPORT_ACTIONS.get) {
        return { status: 'ok', result: { run_id: 'run-1', status: 'active' } } satisfies ReportOutcome<unknown>;
      }
      if (req.actionId === REPORT_ACTIONS.settingsRead) {
        return { status: 'ok', result: { scope: 's', period: 'daily', timezone: 'UTC', section_id: 'x', enabled: false, revision: 1 } } satisfies ReportOutcome<unknown>;
      }
      return { status: 'ok', result: {} } satisfies ReportOutcome<unknown>;
    });
    const client = new ReportsClient(actions);
    const got = await client.get({ run_id: 'run-1' });
    expect(got.status).toBe('active');
    await client.cancel({ run_id: 'run-1' });
    const settings = await client.readSettings({ period: 'daily' });
    expect(settings.revision).toBe(1);
    expect(calls.map((c) => [c.actionId, c.input])).toEqual([
      [REPORT_ACTIONS.get, { run_id: 'run-1' }],
      [REPORT_ACTIONS.cancel, { run_id: 'run-1' }],
      [REPORT_ACTIONS.settingsRead, { period: 'daily' }],
    ]);
  });

  it('never targets the notebook module id', async () => {
    const actions = transport((req) => {
      expect(req.moduleId).not.toBe(NOTEBOOK_MODULE_ID);
      return { status: 'ok', result: {} } satisfies ReportOutcome<unknown>;
    });
    await new ReportsClient(actions).get({ run_id: 'run-1' });
  });

  it('maps transport capability errors to capability_unavailable', async () => {
    const actions = transport(() => {
      const cause = new Error('action not found') as Error & { code: number };
      cause.code = -32004;
      throw cause;
    });
    const client = new ReportsClient(actions);
    await expect(client.readSettings({ period: 'daily' })).rejects.toMatchObject({
      name: 'NotebookError', code: 'capability_unavailable', retryable: false,
    });
  });

  it('decodes typed outcome errors with retryable', async () => {
    const actions = transport(() => ({
      status: 'error',
      error: { code: 'outcome_unknown', message: 'run interrupted', retryable: true },
    }) satisfies ReportOutcome<unknown>);
    const client = new ReportsClient(actions);
    await expect(client.get({ run_id: 'run-1' })).rejects.toMatchObject({
      name: 'NotebookError', code: 'outcome_unknown', retryable: true,
    });
  });

  it('rejects malformed envelopes as outcome_unknown', async () => {
    const actions = transport(() => ({ status: 'weird' }));
    await expect(new ReportsClient(actions).get({ run_id: 'x' })).rejects.toBeInstanceOf(NotebookError);
  });
});
