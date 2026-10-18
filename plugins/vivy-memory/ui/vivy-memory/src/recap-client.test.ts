import { describe, expect, it } from 'vitest';
import type { FaceClientRPC } from '@vivy/ui-sdk';
import { RecapClient } from './recap-client';

describe('RecapClient authenticated session binding', () => {
 it('waits for the same transport to bind the selected session before invoking an action', async () => {
  const calls: {method: string; params: unknown}[] = [];
  let release!: () => void;
  let bound = false;
  const binding = new Promise<void>(resolve => { release = resolve; });
  const rpc = {async call(method: string, params: unknown) {
   calls.push({method, params});
   if (method === 'session/get') { await binding; bound = true; return {session: {id: 'main'}}; }
   if (!bound) throw new Error('unbound peer');
   return {status: 'listed'};
  }} as unknown as FaceClientRPC;
  const outcome = RecapClient.fromRPC(rpc).invoke('read', 'main').then(value => value, error => error);
  await Promise.resolve();
  expect(calls).toEqual([{method: 'session/get', params: {session_id: 'main'}}]);
  release();
  expect(await outcome).toEqual({status: 'listed'});
  expect(calls[1]).toEqual({method: 'module.action.invoke', params: {module_id: 'vivy/diva-cognitive', action_id: 'vivy.recap.read', input: {session_id: 'main'}}});
 });
 it('does not invoke a write when the server refuses session binding', async () => {
  const calls: string[] = [];
  const rpc = {async call(method: string) {
   calls.push(method);
   if (method === 'session/get') throw new Error('session unavailable');
   return {status: 'applied'};
  }} as unknown as FaceClientRPC;
  await expect(RecapClient.fromRPC(rpc).invoke('clear', 'missing', {base_revision: 1, operation_id: 'clear'})).rejects.toThrow('session unavailable');
  expect(calls).toEqual(['session/get']);
 });
});
