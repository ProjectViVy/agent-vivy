#!/usr/bin/env node
// Host-level readiness analysis for the Vivy development environment.
// Read-only by default. Repo checks are delegated to scripts/doctor.mjs; this
// file adds the Windows/WSL/Docker/gate decisions doctor intentionally omits.
// Installs are opt-in, never elevated, and never change global tool config.
import { spawn, spawnSync } from 'node:child_process';
import { existsSync, readFileSync, statfsSync, statSync } from 'node:fs';
import { cpus, homedir, release, totalmem } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { connect } from 'node:net';

const GiB = 1024 ** 3;
const MIN_DISK_GIB = 15;
const MIN_DOCKER_DISK_GIB = 20;
const args = parseArgs(process.argv.slice(2));

function parseArgs(argv) {
  const parsed = {
    mode: 'check',
    json: false,
    repo: '',
    noDoctor: false,
    offline: false,
    dryRun: false,
    install: '',
    proxy: '',
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--json') parsed.json = true;
    else if (arg === '--check') parsed.mode = 'check';
    else if (arg === '--apply') parsed.mode = 'apply';
    else if (arg === '--install') {
      parsed.install = argv[i + 1] || '';
      parsed.mode = 'install';
      i += 1;
    } else if (arg.startsWith('--install=')) {
      parsed.install = arg.slice('--install='.length);
      parsed.mode = 'install';
    } else if (arg === '--repo') {
      parsed.repo = argv[i + 1] || '';
      i += 1;
    } else if (arg.startsWith('--repo=')) {
      parsed.repo = arg.slice('--repo='.length);
    } else if (arg === '--no-doctor') parsed.noDoctor = true;
    else if (arg === '--offline') parsed.offline = true;
    else if (arg === '--dry-run') parsed.dryRun = true;
    else if (arg === '--proxy') {
      parsed.proxy = argv[i + 1] || '';
      i += 1;
    } else if (arg.startsWith('--proxy=')) {
      parsed.proxy = arg.slice('--proxy='.length);
    } else if (arg === '--help' || arg === '-h') parsed.mode = 'help';
    else usageError(`unknown argument: ${arg}`);
  }
  if (parsed.mode === 'install' && !parsed.install) {
    usageError('--install needs a component: wsl or docker');
  }
  return parsed;
}

function usageError(message) {
  console.error(`advance-dev-env: ${message}`);
  console.error('usage: node advance-dev-env.mjs [--check] [--json] [--repo <path>] [--no-doctor] [--proxy <value>]');
  console.error('       node advance-dev-env.mjs --apply [--offline]');
  console.error('       node advance-dev-env.mjs --install <wsl|docker> [--dry-run]');
  console.error('Go downloads default to direct; --proxy overrides for this process only.');
  process.exit(2);
}

function effectiveProxy() {
  return args.proxy || (process.env.VIVY_GOPROXY || '').trim() || 'direct';
}

function proxyEnv() {
  return { VIVY_GOPROXY: effectiveProxy() };
}

function fail(message) {
  console.error(`advance-dev-env: ${message}`);
  process.exit(1);
}

function decode(value) {
  if (value == null) return '';
  const buffer = Buffer.isBuffer(value) ? value : Buffer.from(String(value));
  const text = buffer.includes(0) ? buffer.toString('utf16le') : buffer.toString('utf8');
  return text.replace(/\u0000/g, '').replace(/^\uFEFF/, '').trim();
}

function firstLine(text) {
  return String(text || '').split(/\r?\n/).map((line) => line.trim()).find(Boolean) || '';
}

function run(command, argv = [], options = {}) {
  const result = spawnSync(command, argv, {
    cwd: options.cwd,
    timeout: options.timeout ?? 5000,
    env: options.env ? { ...process.env, ...options.env } : process.env,
    encoding: 'buffer',
    windowsHide: true,
  });
  return {
    command,
    status: result.status,
    error: result.error ? { code: result.error.code, message: result.error.message } : null,
    stdout: decode(result.stdout),
    stderr: decode(result.stderr),
  };
}

function runInherit(command, argv, options = {}) {
  const result = spawnSync(command, argv, {
    cwd: options.cwd,
    timeout: options.timeout,
    env: options.env ? { ...process.env, ...options.env } : process.env,
    stdio: 'inherit',
    windowsHide: true,
  });
  if (result.error) {
    console.error(`advance-dev-env: ${command} could not start: ${result.error.message}`);
    return 1;
  }
  return result.status ?? 1;
}

function commandExists(name) {
  if (process.platform === 'win32') {
    const result = run('where.exe', [name], { timeout: 3000 });
    return result.status === 0 && Boolean(result.stdout);
  }
  const result = run('sh', ['-c', 'command -v "$1" >/dev/null 2>&1', 'sh', name], { timeout: 3000 });
  return result.status === 0;
}

function round(value, digits = 1) {
  return Number(value.toFixed(digits));
}

function detectOs() {
  if (process.platform === 'win32') return 'windows';
  if (process.platform === 'darwin') return 'macos';
  if (process.platform === 'linux' && existsSync('/proc/version')) {
    const version = readFileSync('/proc/version', 'utf8');
    if (/microsoft/i.test(version)) return 'wsl';
  }
  return process.platform === 'linux' ? 'linux' : process.platform;
}

function windowsBuild(osName) {
  if (osName !== 'windows') return null;
  const match = release().match(/^10\.0\.(\d+)/);
  return match ? Number(match[1]) : null;
}

function diskFreeGiB(path) {
  try {
    const stats = statfsSync(path);
    return round((stats.bavail * stats.bsize) / GiB);
  } catch {
    return null;
  }
}

function detectVirtualization(osName) {
  if (osName === 'wsl') return { state: 'active', source: 'WSL2' };
  if (osName !== 'windows') return { state: 'n/a', source: 'not required' };
  const script =
    '$cs=(Get-CimInstance Win32_ComputerSystem); ' +
    '$p=Get-CimInstance Win32_Processor | Select-Object -First 1; ' +
    'Write-Output ($cs.HypervisorPresent.ToString() + "|" + [string]$p.VirtualizationFirmwareEnabled)';
  const result = run('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', script], { timeout: 15000 });
  if (result.status === 0 && result.stdout.includes('|')) {
    const [hypervisor, firmware] = result.stdout.split('|').map((value) => value.trim().toLowerCase());
    if (hypervisor === 'true') return { state: 'active', source: 'Hyper-V/WSL2' };
    if (firmware === 'true') return { state: 'firmware', source: 'enabled in firmware' };
    if (hypervisor === 'false' && firmware === 'false') return { state: 'disabled', source: 'firmware reports disabled' };
  }
  return { state: 'unknown', source: 'could not query Win32_ComputerSystem' };
}

function detectWsl(osName) {
  if (osName === 'wsl') {
    return {
      applicable: true,
      present: true,
      current: true,
      feature: true,
      distros: [process.env.WSL_DISTRO_NAME || 'unknown'],
    };
  }
  if (osName !== 'windows') {
    return { applicable: false, present: false, current: false, feature: false, distros: [] };
  }
  if (!commandExists('wsl.exe')) {
    return { applicable: true, present: false, current: false, feature: false, distros: [], reason: 'wsl.exe not found' };
  }
  const listed = run('wsl.exe', ['-l', '-q'], { timeout: 15000 });
  const distros = listed.stdout
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  if (listed.status === 0 && distros.length > 0) {
    return { applicable: true, present: true, current: false, feature: true, distros };
  }
  return {
    applicable: true,
    present: false,
    current: false,
    feature: true,
    distros: [],
    reason: firstLine(listed.stderr || listed.stdout) || 'no WSL distribution is installed',
  };
}

function isDevDistro(name) {
  return Boolean(name) && !/^docker-desktop(-data)?$/i.test(name);
}

function detectDocker(host) {
  const result = {
    cli: commandExists('docker'),
    daemon: false,
    version: null,
    error: null,
    desktopInstalled: false,
    viaWsl: null,
    dockerHost: process.env.DOCKER_HOST ? redactUrl(process.env.DOCKER_HOST) : null,
    socket: null,
  };
  if (result.cli) {
    const version = run('docker', ['version', '--format', '{{.Server.Version}}'], { timeout: 12000 });
    if (version.status === 0 && version.stdout) {
      result.daemon = true;
      result.version = firstLine(version.stdout);
    } else {
      result.error = firstLine(version.stderr || version.stdout) || 'daemon unavailable';
    }
  }
  if (host.os === 'windows') {
    result.desktopInstalled = [
      'C:\\Program Files\\Docker\\Docker\\Docker Desktop.exe',
      'C:\\Program Files\\Docker\\Docker\\resources\\bin\\docker.exe',
    ].some((path) => existsSync(path));
  }
  if (!result.daemon && host.os === 'windows' && host.wsl.present) {
    for (const distro of host.wsl.distros.filter(isDevDistro).slice(0, 3)) {
      const version = run(
        'wsl.exe',
        ['-d', distro, '--', 'docker', 'version', '--format', '{{.Server.Version}}'],
        { timeout: 15000 },
      );
      if (version.status === 0 && version.stdout) {
        result.viaWsl = { distro, version: firstLine(version.stdout) };
        break;
      }
    }
  }
  if (host.os === 'linux' || host.os === 'wsl') {
    const groups = run('id', ['-nG'], { timeout: 3000 });
    const memberships = groups.status === 0 ? groups.stdout.split(/\s+/).filter(Boolean) : [];
    let socket = {};
    if (existsSync('/var/run/docker.sock')) {
      try {
        const stats = statSync('/var/run/docker.sock');
        socket = { path: '/var/run/docker.sock', worldWritable: Boolean(stats.mode & 0o002) };
      } catch {
        socket = { path: '/var/run/docker.sock' };
      }
    }
    result.socket = { ...socket, userInDockerGroup: memberships.includes('docker') };
  }
  return result;
}

function detectCc() {
  for (const name of ['cc', 'gcc', 'clang']) {
    if (!commandExists(name)) continue;
    const version = run(name, ['--version'], { timeout: 5000 });
    return {
      present: true,
      name,
      version: firstLine(version.stdout || version.stderr) || name,
    };
  }
  return { present: false };
}

function parseDsn(raw) {
  if (/^postgres(ql)?:\/\//i.test(raw)) {
    try {
      const url = new URL(raw);
      return {
        host: url.hostname || 'localhost',
        port: Number(url.port) || 5432,
        database: url.pathname.replace(/^\//, '') || null,
      };
    } catch {
      return null;
    }
  }
  const pairs = {};
  for (const part of raw.split(/\s+/)) {
    const match = part.match(/^([a-zA-Z_]+)=(.*)$/);
    if (match) pairs[match[1].toLowerCase()] = match[2].replace(/^['"]|['"]$/g, '');
  }
  if (!pairs.host) return null;
  return { host: pairs.host, port: Number(pairs.port) || 5432, database: pairs.dbname || null };
}

function probeTcp(host, port, timeout) {
  return new Promise((done) => {
    const socket = connect(port, host);
    let settled = false;
    const finish = (value) => {
      if (settled) return;
      settled = true;
      socket.destroy();
      done(value);
    };
    socket.setTimeout(timeout);
    socket.once('connect', () => finish(true));
    socket.once('timeout', () => finish(false));
    socket.once('error', () => finish(false));
  });
}

async function detectPostgres() {
  const raw = (process.env.VIVY_POSTGRES_TEST_DSN || '').trim();
  if (!raw) return { dsnSet: false, reachable: null };
  const parsed = parseDsn(raw);
  if (!parsed) return { dsnSet: true, reachable: null, reason: 'could not parse host/port from the DSN' };
  if (parsed.host.startsWith('/')) {
    return { dsnSet: true, host: parsed.host, port: null, database: parsed.database, reachable: null, reason: 'Unix socket; TCP probe skipped' };
  }
  const reachable = await probeTcp(parsed.host, parsed.port, 1500);
  return { dsnSet: true, host: parsed.host, port: parsed.port, database: parsed.database, reachable };
}

function findRepo(start) {
  let dir = resolve(start);
  for (let depth = 0; depth < 10; depth += 1) {
    if (
      existsSync(join(dir, 'justfile')) &&
      existsSync(join(dir, 'go.mod')) &&
      existsSync(join(dir, 'laputa-source.lock.json'))
    ) {
      return dir;
    }
    const parent = dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  return null;
}

function detectRepo() {
  const searchedFrom = resolve(args.repo || process.cwd());
  const root = findRepo(searchedFrom);
  if (!root) return { found: false, searchedFrom };
  const repo = {
    found: true,
    root,
    laputa: existsSync(resolve(root, '..', 'laputa')),
    devConfig: existsSync(join(root, 'scripts', 'dev.config.yaml')),
    uiModules: existsSync(join(root, 'ui', 'node_modules')),
    uiDist: existsSync(join(root, 'ui', 'dist', 'index.html')),
    studio: existsSync(join(root, 'studio', 'dsh-vivy-studio', 'package.json')),
    just: commandExists('just'),
    pnpm: commandExists('pnpm'),
    doctor: null,
    scopedKernel: existsSync(join(root, 'scripts', 'verify-kernel.mjs')),
  };
  if (!args.noDoctor && existsSync(join(root, 'scripts', 'doctor.mjs'))) {
    const result = run(process.execPath, [join(root, 'scripts', 'doctor.mjs')], {
      cwd: root,
      timeout: 90000,
      env: proxyEnv(),
    });
    const output = `${result.stdout}\n${result.stderr}`;
    const summary = output.match(/doctor:\s*(\d+)\s+ok,\s*(\d+)\s+warnings?,\s*(\d+)\s+failures?/i);
    const lines = output.split(/\r?\n/).map((line) => line.trim());
    repo.doctor = {
      ran: true,
      exitCode: result.status,
      summary: summary
        ? { ok: Number(summary[1]), warnings: Number(summary[2]), failures: Number(summary[3]) }
        : null,
      failures: lines.filter((line) => line.startsWith('[FAIL]')),
      warnings: lines.filter((line) => line.startsWith('[WARN]')),
      error: result.error ? result.error.message : null,
    };
  }
  return repo;
}

function doctorOk(repo) {
  if (!repo.found || !repo.doctor || !repo.doctor.summary) return null;
  return repo.doctor.summary.failures === 0;
}

function buildGates(host, repo) {
  const ok = doctorOk(repo);
  const ready = ok === true;
  const dockerReady = Boolean(host.docker.daemon || host.docker.viaWsl);
  const gates = [
    {
      id: 'inner-loop',
      label: 'just dev (backend :8788 + Vite :3016)',
      ready,
      requires: 'doctor with 0 failures',
    },
    {
      id: 'fast',
      label: 'just check-fast',
      ready,
      requires: 'Go + pinned Laputa',
    },
    {
      id: 'full-ci',
      label: 'just ci',
      ready: ready && repo.just && repo.pnpm,
      requires: 'Go, Node 22+, pnpm, just, pinned Laputa',
    },
    {
      id: 'race',
      label: 'just race-ci',
      ready: ready && host.cc.present,
      canonical: host.os === 'wsl' || host.os === 'linux',
      requires: host.cc.present
        ? 'C toolchain present; Linux/WSL is the canonical lane'
        : 'needs a C toolchain; run the canonical lane in Linux CI',
    },
    {
      id: 'postgres',
      label: 'just test-postgres',
      ready: ready && host.postgres.dsnSet && host.postgres.reachable === true,
      requires: 'VIVY_POSTGRES_TEST_DSN and a reachable server; skipped is not a pass',
    },
    {
      id: 'docker',
      label: 'just docker-build --smoke',
      ready: repo.found && dockerReady,
      requires: host.docker.viaWsl
        ? `daemon in WSL ${host.docker.viaWsl.distro}; build from the WSL checkout`
        : 'Docker daemon + pinned Laputa',
    },
    {
      id: 'ui-browser',
      label: 'split UI smoke (:3016)',
      ready: ready && repo.pnpm,
      requires: 'Node + pnpm + a browser on the host or forwarded from Windows',
    },
    {
      id: 'windows-acceptance',
      label: 'Windows .exe acceptance',
      ready: host.os === 'windows',
      external: host.os !== 'windows',
      requires: 'Independent Windows checkout at the same SHA; never claim it from WSL/Linux',
    },
    {
      id: 'studio',
      label: 'Studio shell (optional)',
      ready: repo.found && repo.studio,
      optional: true,
      requires: 'studio/ submodule',
    },
  ];
  if (repo.scopedKernel) {
    const linuxLane = (host.os === 'linux' || host.os === 'wsl') &&
      typeof repo.root === 'string' && repo.root.startsWith('/') && !repo.root.startsWith('/mnt/');
    gates.unshift({ id: 'minimal-kernel', label: 'just wiring-ci + just kernel-ci',
      ready: ready && repo.just && linuxLane, external: !linuxLane, requires: 'Linux-filesystem WSL/Linux checkout; no UI installation' });
    const race = gates.find(gate => gate.id === 'race');
    race.ready = ready && host.cc.present && linuxLane;
    race.external = !linuxLane;
    const full = gates.find(gate => gate.id === 'full-ci');
    if (host.os !== 'windows') {
      full.ready = false;
      full.external = true;
      full.requires = 'Legacy full-product recipes require Windows; use the scoped kernel gates here';
    }
    const fast = gates.find(gate => gate.id === 'fast');
    fast.label = 'just wiring-ci';
    fast.ready = ready && repo.just && linuxLane;
    fast.external = !linuxLane;
    fast.requires = 'Independent Linux-filesystem WSL/Linux checkout';
    const postgres = gates.find(gate => gate.id === 'postgres');
    postgres.ready = postgres.ready && linuxLane;
    postgres.external = !linuxLane;
    const inner = gates.find(gate => gate.id === 'inner-loop');
    inner.label = 'manual split (backend :8787 + Vite :3015)';
    inner.ready = ready && repo.pnpm;
    inner.requires = 'Windows legacy launcher or manual split; explicit disposable home';
    const browser = gates.find(gate => gate.id === 'ui-browser');
    browser.label = 'split UI smoke (:3015)';
    const packaging = gates.find(gate => gate.id === 'docker');
    packaging.ready = false;
    packaging.requires = 'Legacy packaging recipe; --smoke wiring is not supplied by scoped CI';
  }
  if (ok === null) {
    for (const gate of gates) {
      if (!gate.optional && gate.id !== 'windows-acceptance') gate.ready = null;
    }
  }
  return gates;
}

function assessWsl(host) {
  if (host.os !== 'windows') {
    return {
      applicable: false,
      installed: host.os === 'wsl',
      suitable: false,
      reason: host.os === 'wsl' ? 'already running inside WSL' : 'WSL is only relevant on a Windows host',
    };
  }
  if (host.wsl.present && host.wsl.distros.filter(isDevDistro).length > 0) {
    return {
      applicable: true,
      installed: true,
      suitable: true,
      reason: `present (${host.wsl.distros.join(', ')})`,
    };
  }
  const reasons = [];
  if (host.windowsBuild == null || host.windowsBuild < 19041) {
    reasons.push(`Windows build ${host.windowsBuild ?? 'unknown'} is older than 19041`);
  }
  if (!['x64', 'arm64'].includes(host.arch)) reasons.push(`architecture ${host.arch} is not supported`);
  if (host.virtualization.state === 'disabled') reasons.push('virtualization is disabled in firmware');
  if (host.virtualization.state === 'unknown') reasons.push('virtualization state could not be confirmed');
  if (host.freeDiskGiB != null && host.freeDiskGiB < MIN_DISK_GIB) {
    reasons.push(`free disk ${host.freeDiskGiB} GiB is below ${MIN_DISK_GIB} GiB`);
  }
  return {
    applicable: true,
    installed: false,
    suitable: reasons.length === 0,
    reasons,
    command: 'wsl.exe --install -d Debian --no-launch',
    note: 'Run without elevation; approve any UAC prompt yourself. A reboot may be required.',
  };
}

function assessDocker(host) {
  if (host.docker.daemon) {
    return { applicable: true, installed: true, running: true, suitable: true, reason: `daemon ${host.docker.version}` };
  }
  if (host.docker.viaWsl) {
    return {
      applicable: true,
      installed: true,
      running: true,
      suitable: true,
      reason: `daemon in WSL ${host.docker.viaWsl.distro} (${host.docker.viaWsl.version})`,
    };
  }
  if (host.docker.desktopInstalled) {
    return {
      applicable: true,
      installed: true,
      running: false,
      suitable: true,
      start: 'docker-desktop',
      reason: 'Docker Desktop is installed but the daemon is not running',
    };
  }
  if (host.os === 'windows') {
    const reasons = [];
    if (host.virtualization.state === 'disabled') reasons.push('virtualization is disabled in firmware');
    if (host.virtualization.state === 'unknown') reasons.push('virtualization state could not be confirmed');
    if (host.freeDiskGiB != null && host.freeDiskGiB < MIN_DOCKER_DISK_GIB) {
      reasons.push(`free disk ${host.freeDiskGiB} GiB is below ${MIN_DOCKER_DISK_GIB} GiB`);
    }
    if (!host.winget) reasons.push('winget is not available');
    return {
      applicable: true,
      installed: false,
      suitable: reasons.length === 0,
      reasons,
      command: 'winget install --id Docker.DockerDesktop -e --accept-package-agreements --accept-source-agreements',
      note: host.wsl.present
        ? 'A WSL daemon already exists; prefer installing Docker Engine inside WSL if you only need Linux containers.'
        : 'Docker Desktop enables Linux containers through WSL2 or Hyper-V.',
    };
  }
  if (host.os === 'macos') {
    return {
      applicable: true,
      installed: false,
      suitable: Boolean(host.brew),
      reasons: host.brew ? [] : ['Homebrew is not available'],
      command: host.brew ? 'brew install --cask docker' : null,
      note: 'Or install Docker Desktop or Colima yourself.',
    };
  }
  return {
    applicable: true,
    installed: false,
    suitable: false,
    reasons: ['Docker Engine installation needs root; this script never elevates.'],
    manual: 'https://docs.docker.com/engine/install/',
  };
}

function redactUrl(value) {
  if (!value) return value;
  try {
    const url = new URL(value);
    if (url.username) url.username = '***';
    if (url.password) url.password = '***';
    return url.toString();
  } catch {
    return value;
  }
}

function buildRecommendations(host, repo, gates, installs) {
  const recommendations = [];
  if (!repo.found) {
    recommendations.push('Run this script from the agent-vivy checkout, or pass --repo <path>.');
  }
  if (repo.found && repo.doctor?.summary?.failures > 0) {
    recommendations.push('Fix the repo doctor [FAIL] lines, then rerun this script.');
  }
  if (host.os === 'windows') {
    if (!installs.wsl.installed) {
      recommendations.push(
        installs.wsl.suitable
          ? 'WSL2 is suitable but absent. Install only after explicit user approval: node advance-dev-env.mjs --install wsl'
          : 'WSL2 is absent or unsuitable. Develop on Windows and let CI run the Linux and race lanes; do not claim them locally.',
      );
    } else {
      recommendations.push('Use WSL2 on the Linux filesystem for the Go/Node/race line; keep Windows for the browser and native .exe acceptance.');
    }
    if (!installs.docker.running) {
      recommendations.push(
        installs.docker.suitable
          ? 'Docker is suitable but not running. Install only after explicit user approval: node advance-dev-env.mjs --install docker'
          : 'Docker packaging is unavailable on this host; keep that gate in CI or in WSL instead of forcing an install.',
      );
    } else if (host.docker.viaWsl) {
      recommendations.push(`Docker runs in WSL ${host.docker.viaWsl.distro}. Run build/smoke commands from the WSL checkout, not the Windows tree.`);
    }
  }
  if (host.os === 'wsl') {
    recommendations.push('Keep the checkout under the Linux filesystem. Use the Windows host for the browser and native .exe acceptance.');
  }
  if (host.os === 'linux') {
    recommendations.push('Windows .exe acceptance needs a separate Windows host; record it as an external gate rather than a local pass.');
  }
  const postgres = gates.find((gate) => gate.id === 'postgres');
  if (postgres && postgres.ready !== true) {
    recommendations.push('Start a real PostgreSQL 16 and export VIVY_POSTGRES_TEST_DSN. A skipped Postgres run is not a pass.');
  }
  if (repo.doctor?.warnings?.some((line) => /proxy: go env GOPROXY/.test(line))) {
    recommendations.push(
      `The global Go proxy differs from this run's ${effectiveProxy()}. Keep the choice process-local (VIVY_GOPROXY) and never run go env -w.`,
    );
  }
  if (host.docker.dockerHost) {
    recommendations.push(`DOCKER_HOST is set (${redactUrl(host.docker.dockerHost)}). Confirm it is intentional and do not expose the socket to every agent.`);
  }
  if (host.docker.socket?.userInDockerGroup) {
    recommendations.push('Your user is in the docker group, which is effectively root. Prefer WSL/Docker Desktop integration over adding agents to that group.');
  }
  if (host.docker.socket?.worldWritable) {
    recommendations.push('The Docker socket is world-writable. Restrict it before running untrusted agents.');
  }
  if (host.memoryGiB < 8) {
    recommendations.push(`The host exposes ${host.memoryGiB} GiB RAM. Cap WSL/Docker and keep one lane active; full CI and containers together will contend.`);
  }
  return recommendations;
}

function describeWsl(wsl) {
  if (!wsl.applicable) return 'not applicable on this host';
  if (wsl.current) return `running inside ${wsl.distros.join(', ')}`;
  if (wsl.present) return `present (${wsl.distros.join(', ')})`;
  return `absent (${wsl.reason || 'not installed'})`;
}

function describeDocker(docker) {
  if (docker.daemon) return `daemon ${docker.version}`;
  if (docker.viaWsl) return `daemon in WSL ${docker.viaWsl.distro} (${docker.viaWsl.version})`;
  if (docker.desktopInstalled) return 'Docker Desktop installed; daemon stopped';
  if (docker.cli) return `CLI present; daemon unavailable (${docker.error || 'unknown'})`;
  return 'not installed on this host';
}

function describePostgres(postgres) {
  if (!postgres.dsnSet) return 'VIVY_POSTGRES_TEST_DSN unset';
  const target = postgres.host ? `${postgres.host}${postgres.port ? `:${postgres.port}` : ''}` : 'unparsed';
  if (postgres.reachable === true) return `${target} reachable`;
  if (postgres.reachable === false) return `${target} unreachable`;
  return `${target} (${postgres.reason || 'not probed'})`;
}

function describeInstall(install) {
  if (!install.applicable) return install.reason || 'not applicable';
  if (install.installed && install.running !== false) return install.reason || 'present';
  if (install.installed && install.running === false) return `${install.reason}; start: ${install.start}`;
  if (install.suitable) return `suitable but absent; explicit opt-in only`;
  return `not suitable: ${(install.reasons || [install.manual || 'no supported installer']).join('; ')}`;
}

function printHuman(host, repo, gates, installs, recommendations) {
  console.log(
    `advance-dev-env: ${host.os}/${host.arch}, ${host.cpus} CPU, ${host.memoryGiB} GiB RAM, ` +
      `${host.freeDiskGiB ?? '?'} GiB free`,
  );
  if (host.os === 'windows') {
    console.log(`  Windows build ${host.windowsBuild ?? 'unknown'}, virtualization ${host.virtualization.state}`);
  }
  console.log(`  WSL: ${describeWsl(host.wsl)}`);
  console.log(`  Docker: ${describeDocker(host.docker)}`);
  console.log(`  C toolchain: ${host.cc.present ? host.cc.version : 'missing'}`);
  console.log(`  Go proxy: ${effectiveProxy()} (process only; no go env -w)`);
  console.log(`  Postgres DSN: ${describePostgres(host.postgres)}`);
  console.log('');
  if (repo.found) {
    console.log(`repo: ${repo.root}`);
    if (repo.doctor) {
      const summary = repo.doctor.summary;
      console.log(
        `repo doctor: ${summary ? `${summary.ok} ok, ${summary.warnings} warnings, ${summary.failures} failures` : `exit ${repo.doctor.exitCode}`}`,
      );
      for (const line of [...repo.doctor.failures, ...repo.doctor.warnings].slice(0, 12)) {
        console.log(`  ${line}`);
      }
    } else {
      console.log('repo doctor: skipped (--no-doctor or missing script)');
    }
    console.log(
      `  laputa ${repo.laputa ? 'present' : 'missing'}; ui/node_modules ${repo.uiModules ? 'present' : 'missing'}; ` +
        `ui/dist ${repo.uiDist ? 'present' : 'missing'}; dev config ${repo.devConfig ? 'present' : 'missing'}`,
    );
  } else {
    console.log(`repo: not found from ${repo.searchedFrom}`);
  }
  console.log('');
  console.log('verification readiness:');
  for (const gate of gates) {
    const status =
      gate.ready === true
        ? 'READY'
        : gate.ready === false
          ? gate.optional
            ? 'OPTIONAL'
            : gate.external
              ? 'EXTERNAL'
              : 'BLOCKED'
          : 'UNKNOWN';
    console.log(`  [${status}] ${gate.label}${gate.requires ? ` - ${gate.requires}` : ''}`);
  }
  console.log('');
  console.log('install assessment (nothing is installed unless you pass --install):');
  console.log(`  wsl: ${describeInstall(installs.wsl)}`);
  console.log(`  docker: ${describeInstall(installs.docker)}`);
  if (recommendations.length) {
    console.log('');
    console.log('next:');
    for (const recommendation of recommendations) console.log(`  - ${recommendation}`);
  }
}

async function startDockerDesktop() {
  const executable = 'C:\\Program Files\\Docker\\Docker\\Docker Desktop.exe';
  if (!existsSync(executable)) {
    console.error('advance-dev-env: Docker Desktop executable not found');
    return 1;
  }
  const child = spawn(executable, [], { detached: true, stdio: 'ignore', windowsHide: true });
  child.unref();
  console.log('advance-dev-env: starting Docker Desktop and waiting for the daemon');
  for (let attempt = 0; attempt < 60; attempt += 1) {
    const version = run('docker', ['version', '--format', '{{.Server.Version}}'], { timeout: 5000 });
    if (version.status === 0 && version.stdout) {
      console.log(`advance-dev-env: Docker daemon ${firstLine(version.stdout)} is up`);
      return 0;
    }
    await new Promise((done) => setTimeout(done, 2000));
  }
  console.error('advance-dev-env: Docker Desktop did not become ready within 120 seconds; start it manually');
  return 1;
}

function pnpmInstall(repo) {
  if (process.platform === 'win32') {
    return runInherit('cmd.exe', ['/d', '/s', '/c', 'pnpm --dir ui install --frozen-lockfile'], { cwd: repo.root });
  }
  return runInherit('pnpm', ['--dir', 'ui', 'install', '--frozen-lockfile'], { cwd: repo.root });
}

async function applySetup(repo) {
  if (!repo.found) fail('no agent-vivy checkout found; pass --repo <path>');
  if (!existsSync(join(repo.root, 'scripts', 'doctor.mjs'))) fail('scripts/doctor.mjs is missing');
  if (args.offline) {
    if (!repo.laputa) fail('offline setup needs the sibling Laputa checkout; run just setup once with network access');
    if (!repo.uiModules) console.warn('advance-dev-env: offline: ui/node_modules is missing; UI gates stay blocked');
    console.log('advance-dev-env: offline mode skips go mod download and pnpm install');
  } else {
    console.log('advance-dev-env: preparing the pinned source closure and Go modules');
    if (
      runInherit(process.execPath, [join(repo.root, 'scripts', 'setup.mjs')], {
        cwd: repo.root,
        env: proxyEnv(),
      }) !== 0
    ) {
      fail('scripts/setup.mjs failed');
    }
    if (!repo.uiModules) {
      if (!commandExists('pnpm')) fail('pnpm is not on PATH; install Node 22+ with corepack/pnpm first');
      console.log('advance-dev-env: installing UI dependencies');
      if (pnpmInstall(repo) !== 0) fail('pnpm install failed');
    }
  }
  console.log('advance-dev-env: running the repo doctor');
  return runInherit(process.execPath, [join(repo.root, 'scripts', 'doctor.mjs')], {
    cwd: repo.root,
    env: proxyEnv(),
  });
}

async function installComponent(host, component) {
  if (component !== 'wsl' && component !== 'docker') usageError('--install accepts wsl or docker');
  const assessment = component === 'wsl' ? assessWsl(host) : assessDocker(host);
  console.log(`advance-dev-env: ${component} assessment: ${describeInstall(assessment)}`);
  if (!assessment.applicable) fail(assessment.reason || `${component} is not applicable on this host`);
  if (assessment.installed && assessment.running !== false) {
    console.log(`advance-dev-env: ${component} is already present; nothing to do`);
    return 0;
  }
  if (assessment.installed && assessment.start) {
    if (args.dryRun) {
      console.log('advance-dev-env: dry run; would start Docker Desktop');
      return 0;
    }
    return startDockerDesktop();
  }
  if (!assessment.suitable) {
    console.error(`advance-dev-env: refusing to install ${component}: ${(assessment.reasons || []).join('; ')}`);
    if (assessment.manual) console.error(`advance-dev-env: manual instructions: ${assessment.manual}`);
    return 1;
  }
  if (args.dryRun) {
    console.log(`advance-dev-env: dry run; would run: ${assessment.command}`);
    return 0;
  }
  if (!assessment.command) {
    console.error(`advance-dev-env: no supported installer for ${component} on this host`);
    if (assessment.manual) console.error(`advance-dev-env: manual instructions: ${assessment.manual}`);
    return 1;
  }
  console.log(`advance-dev-env: running: ${assessment.command}`);
  console.log('advance-dev-env: this script never elevates; approve any UAC prompt yourself');
  let status;
  if (component === 'wsl') {
    status = runInherit('wsl.exe', ['--install', '-d', 'Debian', '--no-launch'], { timeout: 30 * 60 * 1000 });
  } else if (host.os === 'windows') {
    status = runInherit(
      'winget.exe',
      [
        'install',
        '--id',
        'Docker.DockerDesktop',
        '-e',
        '--accept-package-agreements',
        '--accept-source-agreements',
      ],
      { timeout: 30 * 60 * 1000 },
    );
  } else if (host.os === 'macos') {
    status = runInherit('brew', ['install', '--cask', 'docker'], { timeout: 30 * 60 * 1000 });
  } else {
    console.error('advance-dev-env: manual installation is required on this host');
    return 1;
  }
  if (status === 0) {
    console.log(`advance-dev-env: ${component} installer finished; rerun --check after any reboot or first launch`);
    return 0;
  }
  console.error(`advance-dev-env: ${component} installer exited ${status}`);
  return 1;
}

async function main() {
  if (args.mode === 'help') {
    console.log('usage: node advance-dev-env.mjs [--check] [--json] [--repo <path>] [--no-doctor] [--proxy <value>]');
    console.log('       node advance-dev-env.mjs --apply [--offline]');
    console.log('       node advance-dev-env.mjs --install <wsl|docker> [--dry-run]');
    console.log('');
    console.log('Read-only by default. --apply prepares Laputa, Go modules, and UI dependencies.');
    console.log('Go downloads default to direct (VIVY_GOPROXY=direct); --proxy overrides for this process only.');
    console.log('--install is opt-in and never elevates; unsuitable hosts are refused.');
    return 0;
  }

  const host = {
    os: detectOs(),
    arch: process.arch,
    release: release(),
    cpus: cpus().length,
    memoryGiB: round(totalmem() / GiB),
    freeDiskGiB: diskFreeGiB(process.cwd()) ?? diskFreeGiB(homedir()),
  };
  host.windowsBuild = windowsBuild(host.os);
  host.virtualization = detectVirtualization(host.os);
  host.wsl = detectWsl(host.os);
  host.docker = detectDocker(host);
  host.cc = detectCc();
  host.winget = host.os === 'windows' ? commandExists('winget') : null;
  host.brew = host.os === 'macos' ? commandExists('brew') : null;
  host.postgres = await detectPostgres();

  if (args.mode === 'install') {
    return installComponent(host, args.install);
  }

  const repo = detectRepo();
  if (args.mode === 'apply') {
    return applySetup(repo);
  }

  const installs = { wsl: assessWsl(host), docker: assessDocker(host) };
  const gates = buildGates(host, repo);
  const recommendations = buildRecommendations(host, repo, gates, installs);

  if (args.json) {
    console.log(
      JSON.stringify(
        {
          schema: 'vivy.advance-dev-env/v1',
          generatedAt: new Date().toISOString(),
          args: { mode: args.mode, offline: args.offline, proxy: effectiveProxy() },
          host: {
            os: host.os,
            arch: host.arch,
            release: host.release,
            windowsBuild: host.windowsBuild,
            cpus: host.cpus,
            memoryGiB: host.memoryGiB,
            freeDiskGiB: host.freeDiskGiB,
            virtualization: host.virtualization,
            wsl: host.wsl,
            docker: host.docker,
            cc: host.cc,
            winget: host.winget,
            brew: host.brew,
            postgres: host.postgres,
          },
          repo,
          gates,
          installs,
          recommendations,
        },
        null,
        2,
      ),
    );
  } else {
    printHuman(host, repo, gates, installs, recommendations);
  }

  const ok = doctorOk(repo);
  if (!repo.found) return 1;
  if (ok === false) return 1;
  return 0;
}

const invokedDirectly =
  process.argv[1] &&
  resolve(process.argv[1]).toLowerCase() === fileURLToPath(import.meta.url).toLowerCase();

if (invokedDirectly) {
  main()
    .then((code) => process.exit(code ?? 0))
    .catch((error) => {
      console.error(`advance-dev-env: ${error.stack || error.message}`);
      process.exit(1);
    });
}

export { assessDocker, assessWsl, buildGates, parseDsn };
