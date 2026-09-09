import { hydrateLocale, resetLocaleForTests } from '@/i18n';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
// Demo records are seeded at module evaluation, so select their fixture locale first.
hydrateLocale('zh');
const {
  acceptSkillRequest,
  deleteSkill,
  getAutoDreamRunEvents,
  getSkillDocument,
  getSkillHistory,
  getSkillRequests,
  listAutoDreamRuns,
  listSkills,
  rejectSkillRequest,
  updateSkillDocument,
} = await import('./demo-api');

const values = new Map<string, string>();

beforeEach(() => {
  // Seed the existing Chinese demo fixtures explicitly; do not translate stored data.
  hydrateLocale('zh');
  values.clear();
  vi.useFakeTimers();
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
    clear: () => values.clear(),
  });
});

afterEach(() => { resetLocaleForTests(); vi.useRealTimers(); vi.unstubAllGlobals(); });

async function settle<T>(promise: Promise<T>): Promise<T> {
  // flush 期间 promise 可能先行 reject，提前挂接避免未处理拒绝告警
  void promise.catch(() => {});
  await vi.runAllTimersAsync();
  return promise;
}

describe('evolution demo governance', () => {
  it('seeds evolution-managed skills, review requests and autodream runs under vivy.demo.* keys', async () => {
    const skills = await settle(listSkills());
    expect(skills.filter((skill) => skill.evolution_managed).map((skill) => skill.slug)).toEqual(
      expect.arrayContaining(['vivy-code-review', 'vivy-doc-sync']),
    );

    const requests = await settle(getSkillRequests());
    expect(requests.map((request) => request.status).sort()).toEqual(['accepted', 'pending', 'stale']);

    const runs = await settle(listAutoDreamRuns());
    expect(runs).toHaveLength(3);
    const events = await settle(getAutoDreamRunEvents('run-demo-1'));
    expect(events.length).toBeGreaterThan(0);
    expect([...values.keys()].every((key) => key.startsWith('vivy.demo.'))).toBe(true);
  });

  it('accepting a pending request writes the proposal into the skill head and keeps history', async () => {
    const before = await settle(getSkillDocument('vivy-doc-sync'));
    expect(before?.markdown).not.toContain('## 回滚');

    const accepted = await settle(acceptSkillRequest('req-demo-pending'));
    expect(accepted.status).toBe('accepted');

    const after = await settle(getSkillDocument('vivy-doc-sync'));
    expect(after?.markdown).toContain('## 回滚');
    expect(after?.content_hash).not.toBe(before?.content_hash);
    expect(after?.evolution_managed).toBe(true);

    const history = await settle(getSkillHistory('vivy-doc-sync'));
    expect(history).toHaveLength(1);
  });

  it('marks a request stale and refuses to accept when the skill head moved on', async () => {
    const doc = await settle(getSkillDocument('vivy-doc-sync'));
    await settle(updateSkillDocument('vivy-doc-sync', '# 文档同步\n\n编辑后的新头。\n', doc!.content_hash));

    await expect(settle(acceptSkillRequest('req-demo-pending'))).rejects.toThrow();
    const requests = await settle(getSkillRequests());
    expect(requests.find((request) => request.id === 'req-demo-pending')?.status).toBe('stale');
  });

  it('rejects a pending request once, then refuses repeated decisions', async () => {
    const rejected = await settle(rejectSkillRequest('req-demo-pending'));
    expect(rejected.status).toBe('rejected');
    await expect(settle(rejectSkillRequest('req-demo-pending'))).rejects.toThrow();
    await expect(settle(acceptSkillRequest('req-demo-pending'))).rejects.toThrow();
  });

  it('refuses skill saves with a stale base hash (CAS)', async () => {
    const doc = await settle(getSkillDocument('vivy-doc-sync'));
    await expect(
      settle(updateSkillDocument('vivy-doc-sync', '# 改动\n', 'not-the-current-hash')),
    ).rejects.toThrow();
    const unchanged = await settle(getSkillDocument('vivy-doc-sync'));
    expect(unchanged?.content_hash).toBe(doc?.content_hash);
  });

  it('hard-deletes only skills that allow it', async () => {
    await expect(settle(deleteSkill('react-design'))).rejects.toThrow();
    await settle(deleteSkill('vivy-doc-sync'));
    const skills = await settle(listSkills());
    expect(skills.find((skill) => skill.slug === 'vivy-doc-sync')).toBeUndefined();
    expect(await settle(getSkillHistory('vivy-doc-sync'))).toEqual([]);
  });
});
