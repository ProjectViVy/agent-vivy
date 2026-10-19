import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// Workflow correctness E2E harness: an isolated split pair on its own ports,
// a disposable data root, and the loopback model from continuity-setup. The
// suite never reuses a developer server: workflow launch authority depends on
// the live parent Run, so an unrelated session on :3016 would not be the same
// evidence.
const here = path.dirname(fileURLToPath(import.meta.url));

function addr(env: string, fallback: string): string {
  return process.env[env]?.trim() || fallback;
}

// 18987/3195/18988 keep clear of the documented dev pair (8788/3016), the
// manual pair (8787/3015), and the continuity suite (8798/3015).
export const WORKFLOW_BACKEND_ADDR = addr('VIVY_WORKFLOW_E2E_BACKEND_ADDR', '127.0.0.1:18987');
export const WORKFLOW_UI_ADDR = addr('VIVY_WORKFLOW_E2E_UI_ADDR', '127.0.0.1:3195');
export const WORKFLOW_MODEL_ADDR = addr('VIVY_WORKFLOW_E2E_MODEL_ADDR', '127.0.0.1:18988');

export const workflowWorkdir = path.resolve(here, '..', '.workflow-e2e');
export const workflowConfig = path.join(workflowWorkdir, 'config.yaml');
export const workflowWorkspaceRoot = path.join(workflowWorkdir, 'workspaces');

const WORKFLOW_TOOLS = ['echo_info', 'write_note', 'ask_user'];

export function prepareWorkflowWorkdir(): void {
  fs.rmSync(workflowWorkdir, { recursive: true, force: true });
  fs.mkdirSync(path.join(workflowWorkdir, 'state'), { recursive: true });
  const dbPath = path.join(workflowWorkdir, 'state', 'workflow.db').replace(/\\/g, '/');
  const workspaceRoot = workflowWorkspaceRoot.replace(/\\/g, '/');
  fs.writeFileSync(
    workflowConfig,
    [
      'server:',
      `  addr: "${WORKFLOW_BACKEND_ADDR}"`,
      'storage:',
      '  backend: sqlite',
      '  sqlite:',
      `    path: "${dbPath}"`,
      'providers:',
      '  active: deepseek',
      'runtime:',
      `  workspace_root: "${workspaceRoot}"`,
      '  stream_buffer: 256',
      '  max_event_payload_bytes: 65536',
      'tools:',
      '  enabled:',
      ...WORKFLOW_TOOLS.map((name) => `    - ${name}`),
      '  approval:',
      '    expiration: 5m',
      'governance:',
      '  profile: full_auto',
      '',
    ].join('\n'),
    'utf8',
  );
}
