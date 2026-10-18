// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ToolkitCandidate } from './ToolkitCandidate';
import type { PreparedCandidate, ToolkitOperation } from './toolkit-client';
import type { CandidateTarget } from './toolkit-view';

const catalog = { epoch: 3, config_revision: 2, effective_digest: 'a'.repeat(64), generation_id: 'test-gen' };
const target: CandidateTarget = { mutationKind: 'enable', installId: '10000000-0000-4000-8000-000000000001', name: 'bmad/experts' };

function candidate(blockers: ToolkitOperation['blockers']): PreparedCandidate {
  return {
    operation: {
      id: '30000000-0000-4000-8000-000000000001', kind: 'enable', state: 'pending', revision: 2,
      expected_epoch: 3, committed_epoch: 0, retryable: false, blockers,
      prepared: {
        input_digest: 'b'.repeat(64), expected_catalog: catalog, candidate_digest: 'c'.repeat(64),
        releases: [], new_releases: [], installations: [], mutation_kind: 'enable',
        mutation_install_id: target.installId,
        changes: [{ kind: 'skills', local_id: 'pm', change: 'added', after_digest: 'd'.repeat(64) }],
      },
    },
    candidate_digest: 'c'.repeat(64),
    expected_catalog: catalog,
    releases: [],
  };
}

describe('ToolkitCandidate', () => {
  let container: HTMLDivElement;
  let root: Root;
  const t = (key: string) => key;
  beforeEach(() => { vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true); container = document.createElement('div'); document.body.append(container); root = createRoot(container); });
  afterEach(async () => { await act(async () => root.unmount()); container.remove(); vi.restoreAllMocks(); });

  it('shows pending-publication blockers with the runs that hold the catalog', async () => {
    await act(async () => root.render(
      <ToolkitCandidate
        candidate={candidate([{ run_id: 'run-active-1', session_id: 'session-active-1', status: 'active' }])}
        target={target}
        pending={null}
        onPublish={() => {}}
        onCancel={() => {}}
        t={t}
      />,
    ));
    expect(container.textContent).toContain('plugin.vivy/toolkits-ui.blockers');
    expect(container.textContent).toContain('run-active-1');
    expect(container.textContent).toContain('session-active-1');
    expect(container.textContent).toContain('plugin.vivy/toolkits-ui.blockerHint');
    expect(container.querySelector('[data-action="publish-candidate"]')).not.toBeNull();
  });

  it('omits the blocker section for a publishable candidate', async () => {
    await act(async () => root.render(
      <ToolkitCandidate
        candidate={candidate([])}
        target={target}
        pending={null}
        onPublish={() => {}}
        onCancel={() => {}}
        t={t}
      />,
    ));
    expect(container.textContent).not.toContain('plugin.vivy/toolkits-ui.blockers');
    expect(container.textContent).toContain('plugin.vivy/toolkits-ui.candidateMutationLock');
  });
});
