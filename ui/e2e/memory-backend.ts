import { spawn, type ChildProcess } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import type { Page } from '@playwright/test';
import { BACKEND } from '../playwright.memory.config';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

async function healthy(): Promise<boolean> {
  try {
    const reply = await fetch(`http://${BACKEND}/healthz`, { cache: 'no-store' });
    return reply.ok;
  } catch {
    return false;
  }
}

/**
 * Start one packed backend on BACKEND with the disposable e2e config. A
 * listener already answering before spawn is a hard error: the spec must own
 * the process it restarts, never a stray daily or lane server.
 */
export async function startBackend(binary: string, configPath: string): Promise<ChildProcess> {
  if (await healthy()) {
    throw new Error(`memory e2e: ${BACKEND} is already serving before start; refusing to reuse an unknown backend`);
  }
  const child = spawn(binary, [], {
    cwd: repoRoot,
    env: { ...process.env, VIVY_CONFIG: configPath },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let stderr = '';
  child.stderr?.on('data', (chunk) => {
    stderr += String(chunk);
    if (process.env.VIVY_BML_E2E_VERBOSE) process.stderr.write(chunk);
  });
  child.stdout?.on('data', (chunk) => {
    if (process.env.VIVY_BML_E2E_VERBOSE) process.stdout.write(chunk);
  });
  const deadline = Date.now() + 60_000;
  for (;;) {
    if (await healthy()) return child;
    if (child.exitCode !== null) {
      throw new Error(`memory e2e: backend exited with ${child.exitCode}: ${stderr.slice(-2000)}`);
    }
    if (Date.now() > deadline) {
      child.kill('SIGKILL');
      throw new Error(`memory e2e: backend did not become healthy: ${stderr.slice(-2000)}`);
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
}

export async function stopBackend(child: ChildProcess | null): Promise<void> {
  if (!child || child.exitCode !== null) return;
  child.kill('SIGTERM');
  const exited = await new Promise<boolean>((resolve) => {
    const timer = setTimeout(() => resolve(false), 15_000);
    child.once('exit', () => {
      clearTimeout(timer);
      resolve(true);
    });
  });
  if (!exited) {
    child.kill('SIGKILL');
    throw new Error('memory e2e: backend ignored SIGTERM for 15s');
  }
}

/**
 * Invoke one BML control action over a real control WebSocket through the Vite
 * /rpc proxy, exactly like the page does. The returned object must be a memory
 * outcome, never a transport envelope or an empty success.
 */
export async function invokeMemory(page: Page, actionId: string, input: unknown): Promise<Record<string, unknown>> {
  return page.evaluate(async ([action, payload]) => {
    const boot = await fetch('/rpc/bootstrap', { cache: 'no-store' });
    if (!boot.ok) throw new Error(`bootstrap ${boot.status}`);
    const info = await boot.json();
    const url = new URL(info.websocket_path, location.origin);
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    url.searchParams.set('token', info.token);
    const socket = new WebSocket(url.toString());
    return await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('rpc timeout')), 15_000);
      socket.onopen = () =>
        socket.send(JSON.stringify({ jsonrpc: '2.0', id: 'init', method: 'initialize', params: { protocol_version: info.protocol_version } }));
      socket.onmessage = (event) => {
        const data = JSON.parse(String(event.data));
        if (data.id === 'init') {
          socket.send(
            JSON.stringify({
              jsonrpc: '2.0',
              id: 'mem1',
              method: 'module.action.invoke',
              params: { module_id: 'vivy/memory-bml', action_id: action, input: payload },
            }),
          );
        } else if (data.id === 'mem1') {
          clearTimeout(timer);
          socket.close();
          if (data.error) reject(new Error(JSON.stringify(data.error)));
          else resolve(data.result ?? {});
        }
      };
      socket.onerror = () => reject(new Error('ws error'));
    });
  }, [actionId, input] as const);
}
