import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const E2E_ADDR = '127.0.0.1:8799';
const here = path.dirname(fileURLToPath(import.meta.url));
export const e2eWorkdir = path.resolve(here, '..', '.e2e-workdir');
export const e2eConfig = path.join(e2eWorkdir, 'config.yaml');

export const hasRealProvider = Boolean(process.env.DEEPSEEK_API_KEY || process.env.OPENAI_API_KEY || process.env.ANTHROPIC_API_KEY);

export function prepareE2eWorkdir(): void {
  fs.rmSync(e2eWorkdir, { recursive: true, force: true });
  fs.mkdirSync(path.join(e2eWorkdir, 'state'), { recursive: true });
  const dbPath = path.join(e2eWorkdir, 'state', 'e2e.db').replace(/\\/g, '/');
  const workspaceRoot = path.join(e2eWorkdir, 'workspace').replace(/\\/g, '/');
  fs.writeFileSync(e2eConfig, [
    'server:', `  addr: "${E2E_ADDR}"`, 'storage:', '  backend: sqlite', '  sqlite:', `    path: "${dbPath}"`,
    // PROV-P3 removed the per-vendor config block (strict decoding rejects it):
    // provider metadata is embedded data, and the credential comes from each
    // vendor's own declared environment variable. `active` names the vendor
    // whose declared endpoint the runtime falls back to.
    'providers:', '  active: deepseek',
    'runtime:', `  workspace_root: "${workspaceRoot}"`, '  stream_buffer: 256', '  max_event_payload_bytes: 65536',
    'tools:', '  enabled:', '    - echo_info', '    - write_note', '    - ask_user', '  approval:', '    expiration: 5m', '',
  ].join('\n'), 'utf8');
}
