import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const E2E_ADDR = '127.0.0.1:8799';
const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '..', '..');
export const e2eWorkdir = path.resolve(here, '..', '.e2e-workdir');
export const e2eConfig = path.join(e2eWorkdir, 'config.yaml');

export const hasRealProvider = Boolean(process.env.OPENAI_API_KEY || process.env.ANTHROPIC_API_KEY);

export function prepareE2eWorkdir(): void {
  fs.rmSync(e2eWorkdir, { recursive: true, force: true });
  fs.mkdirSync(path.join(e2eWorkdir, 'state'), { recursive: true });
  const fixtureDir = path.join(repoRoot, 'fixtures', 'provider').replace(/\\/g, '/');
  const dbPath = path.join(e2eWorkdir, 'state', 'e2e.db').replace(/\\/g, '/');
  fs.writeFileSync(e2eConfig, [
    'server:', `  addr: "${E2E_ADDR}"`, 'storage:', '  backend: sqlite', '  sqlite:', `    path: "${dbPath}"`,
    'providers:', '  active: openai', `  bundle_dir: "${fixtureDir}"`, '  openai:', '    env_key: OPENAI_API_KEY', '    default_model: gpt-4o-mini',
    'runtime:', '  stream_buffer: 256', '  max_event_payload_bytes: 65536',
    'tools:', '  enabled:', '    - echo_info', '    - write_note', '    - ask_user', '  approval:', '    expiration: 5m', '',
  ].join('\n'), 'utf8');
}
