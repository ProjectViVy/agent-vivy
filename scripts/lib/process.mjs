import { spawnSync } from 'node:child_process';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

export const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');

export function executable(name) {
  if (process.platform === 'win32' && (name === 'pnpm' || name === 'corepack')) {
    return `${name}.cmd`;
  }
  return name;
}

function quoteForCmd(value) {
  return /[\s"]/u.test(value) ? `"${value.replaceAll('"', '""')}"` : value;
}

function spawnExecutable(name, args, options) {
  const command = executable(name);
  if (process.platform === 'win32' && command.endsWith('.cmd')) {
    const commandLine = [command, ...args].map(quoteForCmd).join(' ');
    return spawnSync(process.env.ComSpec || 'cmd.exe', ['/d', '/s', '/c', commandLine], {
      cwd: repoRoot,
      ...options,
    });
  }
  return spawnSync(command, args, { cwd: repoRoot, ...options });
}

export function run(name, args, options = {}) {
  return spawnExecutable(name, args, { encoding: 'utf8', ...options });
}

export function runInherit(name, args, options = {}) {
  return spawnExecutable(name, args, { stdio: 'inherit', ...options });
}

export function fail(prefix, message) {
  console.error(`${prefix}: ${message}`);
  process.exit(1);
}

export function requireSuccess(prefix, result, label) {
  if (result.error) {
    fail(prefix, `${label} could not start: ${result.error.message}`);
  }
  if (result.status !== 0) {
    fail(prefix, `${label} failed (exit ${result.status})`);
  }
  return result;
}
