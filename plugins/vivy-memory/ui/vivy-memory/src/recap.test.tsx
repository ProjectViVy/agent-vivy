// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PluginHostProvider, type FaceStoreState, type FullUIHost } from '@vivy/ui-sdk';
import { RecapWorkspace } from './recap';

const HEAD = { revision: 7, text: '完整的中文 Recap\r\n\n# 第二段\n---\n', cause: 'reconcile', covered_through: 31, invalidation_epoch: 2, updated_at: '2026-10-15T12:00:00Z' };
const key = (name: string) => `plugin.vivy/memory.recap.${name}`;

describe('RecapWorkspace', () => {
 let container: HTMLDivElement;
 let root: Root;
 beforeEach(() => {vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT',true); container=document.createElement('div');document.body.append(container);root=createRoot(container)});
 afterEach(async()=>{await act(async()=>root.unmount());container.remove();vi.unstubAllGlobals()});
 const button=(name:string)=>[...document.body.querySelectorAll('button')].find(item=>item.textContent===key(name)) as HTMLButtonElement;
 async function click(name:string){expect(button(name)).toBeTruthy();await act(async()=>button(name).click())}
 async function edit(text:string){const input=container.querySelector('textarea')!;expect(input).toBeTruthy();await act(async()=>{Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value')!.set!.call(input,text);input.dispatchEvent(new Event('input',{bubbles:true}))})}
 async function mount(call:(id:string,input:Record<string,unknown>)=>Promise<unknown>){const host=makeHost(call);await act(async()=>root.render(<PluginHostProvider host={host}><RecapWorkspace/></PluginHostProvider>));return host}

 it('shows the complete owner text, actual revision/correction/coverage and explicit unavailable loaded status',async()=>{
  await mount(async()=>({status:'listed',head:HEAD,activation:'next_run',loaded_status:'unavailable'}));
  expect(container.querySelector('[data-testid="recap-full-text"]')?.textContent).toBe(HEAD.text);
  expect(container.textContent).toContain('7');expect(container.textContent).toContain('31');expect(container.textContent).toContain(key('cause.reconcile'));
  expect(container.textContent).toContain(key('nextRun'));expect(container.textContent).toContain(key('loadedUnavailable'));
 });
 it('shows the admitted model revision independently from a newer saved head',async()=>{
  await mount(async()=>({status:'listed',head:HEAD,loaded_status:'loaded',loaded:{status:'loaded',run_id:'current-model-run',revision:4,invalidation_epoch:1}}));
  expect(container.textContent).toContain(`${key('loadedRevision')} 4 · current-model-run`);
  expect(container.querySelector('[data-testid="recap-full-text"]')?.textContent).toBe(HEAD.text);
  expect(container.textContent).not.toContain(key('loadedUnavailable'));
 });
 it('retains a dirty draft and original base across background refresh, failed save and cancel',async()=>{
  let head=HEAD;const writes:Record<string,unknown>[]=[];
  await mount(async(id,input)=>{if(id==='vivy.recap.edit'){writes.push(input);throw new Error('connection lost')};return {status:'listed',head}});
  await click('edit');await edit('未保存全文\n\n中文第二行\n');head={...HEAD,revision:8,text:'new remote text'};
  await click('refresh');expect(container.querySelector('textarea')?.value).toBe('未保存全文\n\n中文第二行\n');
  await click('save');expect(writes[0].base_revision).toBe(7);expect(container.querySelector('textarea')?.value).toBe('未保存全文\n\n中文第二行\n');
  expect(container.textContent).toContain('connection lost');await click('cancel');await click('edit');expect(container.querySelector('textarea')?.value).toBe('未保存全文\n\n中文第二行\n');
 });
 it('compares stale save with latest head and rebases only on explicit edit-latest without losing draft',async()=>{
  const writes:Record<string,unknown>[]=[];let latest={...HEAD,revision:8,text:'现在的保存内容\n'};
  await mount(async(id,input)=>{if(id==='vivy.recap.edit'){writes.push(input);return writes.length===1?{status:'conflict',head:latest,reason:'recap_conflict'}:{status:'applied',head:{...latest,revision:9,text:input.text},activation:'next_run'}};return {status:'listed',head:HEAD}});
  await click('edit');await edit('我的完整草稿\n');await click('save');
  expect(container.textContent).toContain('现在的保存内容');expect(container.querySelector('textarea')?.value).toBe('我的完整草稿\n');expect(writes).toHaveLength(1);
  await click('editLatest');await click('save');expect(writes[1].base_revision).toBe(8);expect(writes[1].text).toBe('我的完整草稿\n');
  expect(container.querySelector('[data-testid="recap-full-text"]')?.textContent).toBe('我的完整草稿\n');
 });
 it('requires clear confirmation and retains exact history preview before restoring as a new revision',async()=>{
  const writes:string[]=[];
  await mount(async(id,input)=>{if(id==='vivy.recap.history')return {status:'listed',revisions:[HEAD,{...HEAD,revision:6,text:'修正前的完整文字\n',cause:'summary'}]};if(id==='vivy.recap.revision')return {status:'listed',head:{...HEAD,revision:input.revision,text:input.revision===6?'修正前的完整文字\n':HEAD.text,cause:input.revision===6?'summary':'reconcile'}};if(id==='vivy.recap.clear'){writes.push(id);return {status:'applied',head:{...HEAD,revision:8,text:'',cause:'clear'},activation:'next_safe_boundary'}};if(id==='vivy.recap.restore'){writes.push(id);expect(input.revision).toBe(6);expect(input.base_revision).toBe(8);return {status:'applied',head:{...HEAD,revision:9,text:'修正前的完整文字\n',cause:'restore'},activation:'next_run'}};return {status:'listed',head:HEAD}});
  await click('clear');expect(writes).toHaveLength(0);expect(document.body.textContent).toContain(key('clearExplanation'));await click('dismiss');expect(writes).toHaveLength(0);
  await click('clear');await click('confirmClear');expect(writes).toEqual(['vivy.recap.clear']);
  await click('history');const revision=[...container.querySelectorAll('button')].find(item=>item.dataset.revision==='6')!;await act(async()=>revision.click());
  expect(container.querySelector('[data-testid="recap-history-text"]')?.textContent).toBe('修正前的完整文字\n');await click('restore');await click('confirmRestore');expect(writes).toEqual(['vivy.recap.clear','vivy.recap.restore']);
 });

 it('translates a denied read instead of showing the backend reason code',async()=>{
  await mount(async()=>({status:'failed',reason:'recap_scope_denied'}));
  expect(container.textContent).toContain(key('reason.recap_scope_denied'));expect(container.textContent).not.toContain('recap_scope_denied ');
  expect(container.textContent).not.toMatch(/(^|[^.])recap_scope_denied/);
 });
 it('maps an unknown backend reason to the generic message',async()=>{
  await mount(async()=>({status:'failed',reason:'some_internal_code'}));
  expect(container.textContent).toContain(key('reason.generic'));expect(container.textContent).not.toContain('some_internal_code');
 });
 it('keeps a dirty draft accessible when a later authorized read becomes unavailable',async()=>{
  let unavailable=false;
  await mount(async()=>unavailable?{status:'failed',reason:'recap_unavailable'}:{status:'listed',head:HEAD});
  await click('edit');await edit('网络失败也保留的草稿\n');unavailable=true;
  await act(async()=>window.dispatchEvent(new Event('focus')));
  expect(container.querySelector('textarea')?.value).toBe('网络失败也保留的草稿\n');
  expect(button('save')?.disabled).toBe(true);
 });
 it('keeps the established BML record and handbook controls available',async()=>{
  await mount(async id=>id.startsWith('vivy.memory.')?{status:'listed',entries:[]}:{status:'listed',head:HEAD});
  await click('longTerm');expect(container.textContent).toContain('plugin.vivy/memory.addTitle');
  expect(container.querySelector('input')).not.toBeNull();
 });

 it('pins a clear dialog base despite a background refresh and shows the conflict without clearing newer text',async()=>{
  let latest=HEAD;const writes:Record<string,unknown>[]=[];
  await mount(async(id,input)=>{if(id==='vivy.recap.clear'){writes.push(input);return input.base_revision===7?{status:'conflict',head:latest,reason:'recap_conflict'}:{status:'applied',head:{...latest,text:'',revision:9}}};return {status:'listed',head:latest}});
  await click('clear');latest={...HEAD,revision:8,text:'后来保存的重要内容\n'};
  await act(async()=>window.dispatchEvent(new Event('focus')));await click('confirmClear');
  expect(writes[0].base_revision).toBe(7);expect(container.textContent).toContain(key('conflict'));
  expect(container.textContent).toContain('后来保存的重要内容');
 });
 it('ignores an older refresh response after a successful save',async()=>{
  let reads=0;let resolveRead:((value:unknown)=>void)|undefined;
  await mount(async(id,input)=>{if(id==='vivy.recap.edit')return {status:'applied',head:{...HEAD,revision:8,text:input.text}};if(++reads===1)return {status:'listed',head:HEAD};return new Promise(resolve=>{resolveRead=resolve})});
  await click('edit');await edit('已保存的新文本\n');await click('refresh');await click('save');
  await act(async()=>resolveRead?.({status:'listed',head:{...HEAD,revision:6,text:'stale response'}}));
  expect(container.querySelector('[data-testid="recap-full-text"]')?.textContent).toBe('已保存的新文本\n');
 });

 it('pins both restore base and selected historical revision until explicit conflict resolution',async()=>{
  let latest=HEAD;const writes:Record<string,unknown>[]=[];
  await mount(async(id,input)=>{if(id==='vivy.recap.history')return {status:'listed',revisions:[{...HEAD,revision:6}]};if(id==='vivy.recap.revision')return {status:'listed',head:{...HEAD,revision:6}};if(id==='vivy.recap.restore'){writes.push(input);return {status:'conflict',head:latest,reason:'recap_conflict'}};return {status:'listed',head:latest}});
  await click('history');await act(async()=>container.querySelector<HTMLButtonElement>('[data-revision="6"]')!.click());await click('restore');
  latest={...HEAD,revision:8,text:'restoration must not erase this'};
  await act(async()=>window.dispatchEvent(new Event('focus')));await click('confirmRestore');
  expect(writes[0].base_revision).toBe(7);expect(writes[0].revision).toBe(6);
  expect(container.textContent).toContain(key('conflictRestore'));expect(container.textContent).toContain(latest.text);
 });
 it('ignores an old session read and restores its unsaved draft only when that session is selected again',async()=>{
  let reads=0;let resolveRead:((value:unknown)=>void)|undefined;
  const host=await mount(async(_id,input)=>{if(input.session_id==='other')return {status:'failed',reason:'recap_scope_denied'};if(++reads===2)return new Promise(resolve=>{resolveRead=resolve});return {status:'listed',head:HEAD}});
  await click('edit');await edit('main only draft\n');await click('refresh');
  await act(async()=>host.store.setState({activeSessionId:'other'}));
  await act(async()=>resolveRead?.({status:'listed',head:{...HEAD,revision:9,text:'old private reply'}}));
  expect(container.textContent).not.toContain('old private reply');expect(container.querySelector('textarea')).toBeNull();
  await act(async()=>host.store.setState({activeSessionId:'main'}));await click('edit');
  expect(container.querySelector('textarea')?.value).toBe('main only draft\n');
 });

 it('renders certified originals, tool failure, attached snapshots and correction coordinates',async()=>{
  await mount(async id=>id==='vivy.recap.sources'?{status:'listed',sources:[{interaction_id:'original-a',source_revision:3,messages:[{id:'tool-result',role:'tool',content:'unaltered original',tool_name:'read',tool_status:'failed',tool_arguments:'{"path":"old.txt"}',tool_error:'permission denied',attachments:[{name:'evidence.txt',mime_type:'text/plain',data_url:'data:text/plain;base64,eA=='}],file_contexts:[{name:'old.txt',path:'/past/old.txt',content:'immutable file snapshot'}]}],references:[{id:'ref-1',source_workspace:'saved workspace',items:[{author:'user',text:'exact copied reference'}]}],corrections:[{view_session_id:'rewound-view',reason:'rewind',message_ids:['tool-result'],view_message_ids:['view-tool']}]}]}:{status:'listed',head:HEAD});
  await click('sources');
  for(const text of ['unaltered original','permission denied','immutable file snapshot','exact copied reference','rewound-view','view-tool'])expect(container.textContent).toContain(text);
  expect(container.querySelector('a[download="evidence.txt"]')?.getAttribute('href')).toBe('data:text/plain;base64,eA==');
  expect(container.textContent).not.toContain(key('sourceUnavailable'));
 });
 it('reports a pending source read as loading before its availability is known',async()=>{
  let finish:((value:unknown)=>void)|undefined;
  await mount(async id=>id==='vivy.recap.sources'?new Promise(resolve=>{finish=resolve}):{status:'listed',head:HEAD});
  await click('sources');expect(container.textContent).toContain(key('loading'));
  expect(container.textContent).not.toContain(key('sourceUnavailable'));
  await act(async()=>finish?.({status:'failed',reason:'recap_sources_unavailable'}));
  expect(container.textContent).toContain(key('sourceUnavailable'));
 });
 it('renders denied and unavailable sources without inventing excerpts, and keeps the Persona review link',async()=>{
  const host=await mount(async(id)=>id==='vivy.recap.sources'?{status:'failed',reason:'recap_sources_unavailable'}:{status:'listed',head:HEAD});
  await click('sources');expect(container.querySelector('[data-reason="recap_sources_unavailable"]')).not.toBeNull();expect(container.querySelector('[data-testid="recap-source-message"]')).toBeNull();await click('persona');expect(host.router.navigate).toHaveBeenCalledWith({to:'/persona'});
 });
});
function makeHost(call:(id:string,input:Record<string,unknown>)=>Promise<unknown>):FullUIHost {
 let state={activeSessionId:'main',connection:'connected',currentRun:null} as unknown as FaceStoreState;
 const listeners=new Set<()=>void>();
 return {api:{} as FullUIHost['api'],rpc:{capabilities:{protocol_version:'vivy.rpc.v1',capabilities:['module.action.invoke']},call:vi.fn(async(method,params)=>{if(method==='session/get')return {session:{id:(params as {session_id:string}).session_id}};const p=params as {action_id:string,input:Record<string,unknown>};return call(p.action_id,p.input)}),onNotification:vi.fn(()=>()=>{}),onClose:vi.fn(()=>()=>{}),close:vi.fn()} as unknown as FullUIHost['rpc'],store:{getState:()=>state,getInitialState:()=>state,subscribe:(listener:()=>void)=>{listeners.add(listener);return()=>listeners.delete(listener)},setState:(partial:Partial<FaceStoreState>)=>{state={...state,...partial};listeners.forEach(listener=>listener())}} as unknown as FullUIHost['store'],router:{navigate:vi.fn().mockResolvedValue(undefined),invalidate:vi.fn().mockResolvedValue(undefined)},composition:{} as FullUIHost['composition'],t:key=>key,registerCleanup:()=>({active:true,dispose:vi.fn()})};
}
