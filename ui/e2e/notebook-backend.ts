import { spawn, type ChildProcess } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { BACKEND } from '../playwright.notebook.config';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

/** Start one packed backend on BACKEND with the disposable e2e config. */
export async function startBackend(binary: string, configPath: string): Promise<ChildProcess> {
  const child = spawn(binary, [], {
    cwd: repoRoot,
    env: { ...process.env, VIVY_CONFIG: configPath },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  child.stderr?.on('data', (chunk) => {
    if (process.env.VIVY_NB_E2E_VERBOSE) process.stderr.write(chunk);
  });
  const deadline = Date.now() + 60_000;
  for (;;) {
    try {
      const reply = await fetch(`http://${BACKEND}/healthz`, { cache: 'no-store' });
      if (reply.ok) return child;
    } catch {
      // not up yet
    }
    if (Date.now() > deadline) {
      child.kill('SIGKILL');
      throw new Error('notebook e2e backend did not become healthy');
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
}

export async function stopBackend(child: ChildProcess | null): Promise<void> {
  if (!child || child.exitCode !== null) return;
  child.kill('SIGTERM');
  await new Promise<void>((resolve) => {
    const timer = setTimeout(() => {
      child.kill('SIGKILL');
      resolve();
    }, 10_000);
    child.once('exit', () => {
      clearTimeout(timer);
      resolve();
    });
  });
}
