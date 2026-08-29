import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const E2E_ADDR = '127.0.0.1:8799';
export const E2E_STUB_ADDR = '127.0.0.1:8800';
const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '..', '..');
export const e2eWorkdir = path.resolve(here, '..', '.e2e-workdir');
export const e2eHome = path.join(e2eWorkdir, 'home');
export const e2eConfig = path.join(e2eWorkdir, 'config.yaml');

export function prepareE2eWorkdir(): void {
  fs.rmSync(e2eWorkdir, { recursive: true, force: true });
  fs.mkdirSync(e2eHome, { recursive: true });
  const fixtureDir = path.join(repoRoot, 'fixtures', 'provider').replace(/\\/g, '/');
  const dbPath = path.join(e2eHome, 'vivy.db').replace(/\\/g, '/');
  const stub = `http://${E2E_STUB_ADDR}/v1`;
  fs.writeFileSync(e2eConfig, [
    'server:', `  addr: "${E2E_ADDR}"`,
    'storage:', '  backend: sqlite', '  sqlite:', `    path: "${dbPath}"`,
    'providers:', '  active: openai', `  bundle_dir: "${fixtureDir}"`,
    '  openai:', '    env_key: OPENAI_API_KEY', '    default_model: gpt-4o-mini',
    '  anthropic:', '    env_key: ANTHROPIC_API_KEY', '    default_model: claude-sonnet-4-5',
    'runtime:', '  stream_buffer: 256', '  max_event_payload_bytes: 65536',
    `  workspace_root: "${path.join(e2eHome, 'workspace').replace(/\\/g, '/')}"`,
    `  skills_root: "${path.join(e2eHome, 'skills').replace(/\\/g, '/')}"`,
    'tools:', '  enabled:', '    - echo_info', '    - write_note', '    - ask_user',
    '  approval:', '    expiration: 5m', '',
  ].join('\n'), 'utf8');
  fs.writeFileSync(path.join(e2eHome, 'settings.yaml'), [
    'provider: openai',
    'default_model: gpt-4o-mini',
    `base_url: ${stub}`,
    'providers:',
    '  - id: e2e-stub',
    '    display_name: E2E Stub',
    '    bundle: openai',
    `    base_url: ${stub}`,
    '    default_model: gpt-4o-mini',
    '    models:',
    '      - gpt-4o-mini',
    '    api_key: sk-e2e-stub',
    '',
  ].join('\n'), 'utf8');
}
