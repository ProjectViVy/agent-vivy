import { existsSync, rmdirSync, unlinkSync, writeFileSync } from 'node:fs';
import path from 'node:path';

// The marker is created by this config in its own OS temp directory. The Go
// fixture observes it and runs App's normal shutdown, including on Windows.
export default async function teardown() {
 const marker=process.env.VIVY_ISSUE58_SPLIT_STOP_FILE;
 if (!marker || path.basename(marker)!=='stop' || !path.basename(path.dirname(marker)).startsWith('vivy-issue58-split-')) return;
 writeFileSync(marker,'');
 const deadline=Date.now()+10_000;
 while(Date.now()<deadline) {
  try {await fetch('http://127.0.0.1:8787/healthz',{signal:AbortSignal.timeout(300)});}
  catch {break;}
  await new Promise(resolve=>setTimeout(resolve,100));
 }
 if (existsSync(marker)) unlinkSync(marker);
 // Never recursively remove another process's data directory.
 try {rmdirSync(path.dirname(marker));} catch { /* The test owns any remaining artifact. */ }
}
