// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { expect, it, vi } from 'vitest';
import type { UITranslator } from '@vivy/ui-sdk';
import { MemoryClient } from './memory-client';
import { MemoryRulesPanel } from './rules';

it('retains a conflicted draft and refreshes exact content and existence before retry', async () => {
 vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
 const container = document.createElement('div'); document.body.append(container);
 const root = createRoot(container);
 let reads = 0;
 const invoke = vi.fn(async ({ actionId }: { actionId: string }) => {
  if (actionId.endsWith('.read')) { reads++; return { status: 'listed', content: reads === 1 ? 'original' : 'external', source: 'default', exists: false }; }
  return { status: 'failed', reason: 'memory_revision_conflict' };
 });
 const client = new MemoryClient({ invoke } as never);
 const button = (label: string) => [...container.querySelectorAll('button')].find(b => b.textContent?.includes(label))!;
 try {
  await act(async () => root.render(<MemoryRulesPanel client={client} t={((key: string) => key) as UITranslator} />));
  await act(async () => button('common.edit').click());
  const textarea = container.querySelector('textarea')!;
  await act(async () => {
   Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(textarea, 'my draft');
   textarea.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await act(async () => button('common.save').click());
  expect(invoke).toHaveBeenLastCalledWith(expect.objectContaining({ input: { content: 'my draft', expected_content: 'original', expected_exists: false } }));
  expect(textarea.value).toBe('my draft');
  expect(container.textContent).toContain('plugin.vivy/memory.conflict');
  await act(async () => button('common.refresh').click());
  expect(container.querySelector('textarea')).toBeNull();
  expect(container.textContent).toContain('external');
  await act(async () => button('common.edit').click());
  await act(async () => button('common.save').click());
  expect(invoke).toHaveBeenLastCalledWith(expect.objectContaining({ input: { content: 'external', expected_content: 'external', expected_exists: false } }));
 } finally { await act(async () => root.unmount()); container.remove(); vi.unstubAllGlobals(); }
});
