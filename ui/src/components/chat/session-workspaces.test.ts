import { describe, expect, it } from 'vitest';
import type { Session } from '@/lib/api';
import { groupSessionsByWorkspace, workspaceDisplayName } from './session-workspaces';

const session = (id: string, workspace_path = ''): Session => ({
  id,
  title: id,
  created_at: Number(id.replace(/\D/g, '')) || 1,
  workspace_path,
});

describe('session workspace groups', () => {
  it('groups by authoritative path with the default workspace first', () => {
    const groups = groupSessionsByWorkspace([
      session('s3', '/work/beta'),
      session('s2', '/work/alpha'),
      session('s1'),
      session('s4', '/work/alpha'),
    ], 'Default workspace');

    expect(groups.map((group) => ({ key: group.key, label: group.label, ids: group.sessions.map((item) => item.id) }))).toEqual([
      { key: '', label: 'Default workspace', ids: ['s1'] },
      { key: '/work/alpha', label: 'alpha', ids: ['s2', 's4'] },
      { key: '/work/beta', label: 'beta', ids: ['s3'] },
    ]);
  });

  it('renders both POSIX and Windows directory names without merging equal basenames', () => {
		expect(workspaceDisplayName('/')).toBe('/');
		expect(workspaceDisplayName('C:\\')).toBe('C:\\');
    expect(workspaceDisplayName('/projects/vivy')).toBe('vivy');
    expect(workspaceDisplayName('C:\\code\\vivy')).toBe('vivy');
    const groups = groupSessionsByWorkspace([
      session('s1', '/one/vivy'),
      session('s2', '/two/vivy'),
    ], 'Default');
    expect(groups).toHaveLength(2);
    expect(groups.map((group) => group.key)).toEqual(['/one/vivy', '/two/vivy']);
  });
});
