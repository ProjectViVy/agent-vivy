import { expect, test, type Page } from '@playwright/test';

const SESSION = 'issue58-recap-main';
const ORIGINAL = '完整的测试回顾\n\n第二段保持原样。\n';
type Outcome = {status: string; reason?: string; head?: {revision: number; text: string}};

// Real authenticated peer, current session bind and actual ActionHost/C1.
// No routing, action response, database, or owner implementation is mocked.
async function action(page: Page, name: string, input: Record<string, unknown> = {}): Promise<Outcome> {
 return page.evaluate(async ({name,input,session}) => {
  const bootstrap = await (await fetch('/rpc/bootstrap', {cache:'no-store'})).json();
  const url = new URL(bootstrap.websocket_path, location.origin);url.protocol='ws:';url.searchParams.set('token',bootstrap.token);
  const ws = new WebSocket(url);
  await new Promise<void>((resolve,reject)=>{ws.onopen=()=>resolve();ws.onerror=()=>reject(new Error('WebSocket unavailable'))});
  let id=0;
  const call=(method:string,params:Record<string,unknown>)=>new Promise<unknown>((resolve,reject)=>{
   const next=++id;const timer=window.setTimeout(()=>reject(new Error('RPC timed out')),8000);
   ws.onmessage=event=>{const reply=JSON.parse(String(event.data));if(reply.id!==next)return;window.clearTimeout(timer);reply.error?reject(new Error(reply.error.message)):resolve(reply.result)};
   ws.send(JSON.stringify({jsonrpc:'2.0',id:next,method,params}));
  });
  try {
   await call('initialize',{protocol_version:bootstrap.protocol_version});
   await call('session/get',{session_id:session});
   return await call('module.action.invoke',{module_id:'vivy/diva-cognitive',action_id:`vivy.recap.${name}`,input:{session_id:session,...input}}) as Outcome;
  } finally {ws.close()}
 }, {name,input,session:SESSION});
}

test('governed Recap keeps full drafts through conflicts, history, sources and clear on the real split backend',async({page},testInfo)=>{
 test.skip(process.env.VIVY_ISSUE58_SPLIT_TEST!=='1','run with playwright.issue58.config.ts and its disposable actual backend');
 const errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
 await page.addInitScript(()=>{
  localStorage.setItem('vivy.ui.welcome.completed','1');
  localStorage.setItem('vivy.ui.activeSession','issue58-recap-main');
 });
 await page.goto('/memory');
 await expect(page.getByTestId('recap-full-text')).toHaveText(ORIGINAL);
 expect(await page.getByTestId('recap-full-text').textContent()).toBe(ORIGINAL);
 await page.screenshot({path:testInfo.outputPath('recap-read.png'),fullPage:true});
 await page.getByRole('button',{name:'Edit',exact:true}).click();
 const draft=page.getByRole('textbox',{name:/Your draft/});
 const text='我的完整草稿\n\n第二段不应丢失。\n';await draft.fill(text);
 const current=await action(page,'read');
 expect((await action(page,'edit',{base_revision:current.head!.revision,operation_id:crypto.randomUUID(),text:'Other authorized writer\n'})).status).toBe('applied');
 await page.getByRole('button',{name:'Save',exact:true}).click();
 const conflict=page.getByRole('region',{name:'The saved Recap changed'});
 await expect(conflict).toBeVisible();await expect(conflict).toContainText('Other authorized writer');await expect(draft).toHaveValue(text);
 await page.screenshot({path:testInfo.outputPath('recap-conflict.png'),fullPage:true});
 await page.getByRole('button',{name:'Edit against latest',exact:true}).click();await expect(draft).toHaveValue(text);
 await page.getByRole('button',{name:'Save',exact:true}).click();await expect(page.getByTestId('recap-full-text')).toHaveText(text);
 expect((await action(page,'read')).head?.text).toBe(text);
 await page.getByRole('tab',{name:'History',exact:true}).click();await page.locator('button[data-revision="1"]').click();
 await expect(page.getByTestId('recap-history-text')).toHaveText(ORIGINAL);
 expect(await page.getByTestId('recap-history-text').textContent()).toBe(ORIGINAL);
 await page.screenshot({path:testInfo.outputPath('recap-history.png'),fullPage:true});
 await page.getByRole('button',{name:'View covered sources',exact:true}).click();
 await expect(page.locator('[data-reason="recap_sources_unavailable"]')).toBeVisible();
 await expect(page.getByTestId('recap-source-message')).toHaveCount(0);
 await page.screenshot({path:testInfo.outputPath('recap-sources-unavailable.png'),fullPage:true});
 await page.getByRole('button',{name:'Restore this revision',exact:true}).click();
 await page.getByRole('button',{name:'Create restored revision',exact:true}).click();
 await page.getByRole('tab',{name:'Active Recap',exact:true}).click();await expect(page.getByTestId('recap-full-text')).toHaveText(ORIGINAL);
 const beforeClear=(await action(page,'read')).head!.revision;
 await page.getByRole('button',{name:'Clear active Recap',exact:true}).click();
 await page.getByRole('dialog').press('Escape');expect((await action(page,'read')).head!.revision).toBe(beforeClear);
 await page.getByRole('button',{name:'Clear active Recap',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText('History and original sources remain');
 await page.screenshot({path:testInfo.outputPath('recap-clear-confirm.png'),fullPage:true});
 await page.getByRole('button',{name:'Confirm clear',exact:true}).click();await expect(page.getByTestId('recap-full-text')).toHaveText('');
 expect((await action(page,'read')).head?.text).toBe('');
 await page.getByRole('tab',{name:'Long-term records',exact:true}).click();
 await expect(page.locator('input')).not.toHaveCount(0);
 expect(errors).toEqual([]);
});

test('historical sources survive removal of the derived BML record on the real split backend',async({page})=>{
 test.skip(process.env.VIVY_ISSUE58_SPLIT_TEST!=='1','requires disposable actual backend');
 await page.addInitScript(()=>{
  localStorage.setItem('vivy.ui.welcome.completed','1');
  localStorage.setItem('vivy.ui.activeSession','issue58-recap-main');
 });
 await page.goto('/memory');
 await expect(page.getByTestId('recap-full-text')).toBeVisible();
 const onboarding=page.getByRole('dialog',{name:'Persona onboarding'});
 if(await onboarding.isVisible()) await onboarding.getByRole('button',{name:'Not now',exact:true}).click();
 await page.getByRole('tab',{name:'History',exact:true}).click();
 await page.locator('button[data-revision="2"]').click();
 await page.getByRole('button',{name:'View covered sources',exact:true}).click();
 const originals=page.getByTestId('recap-source-message');
 await expect(originals).toHaveCount(1);
 await expect(originals).toContainText('exact-browser-original '.repeat(1000).trim());
 await originals.locator('summary').click();
 await expect(originals).toContainText('immutable browser file snapshot');
 await expect(page.locator('[data-reason="recap_sources_unavailable"]')).toHaveCount(0);
});
