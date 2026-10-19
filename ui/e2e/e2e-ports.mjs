/**
 * One port resolver for every browser gate in this repository.
 *
 * Each gate keeps its historical defaults, but an operator may run it beside
 * another lane by setting the two documented variables. Validation happens
 * before any process starts, so an invalid or colliding pair fails with the
 * variable name instead of a Playwright "server already running" symptom.
 */
const LOOPBACK = '127.0.0.1';

function readPort(env, variable, fallback) {
  const raw = env?.[variable];
  if (raw === undefined || raw === '') return fallback;
  if (!/^[0-9]+$/.test(raw)) {
    throw new Error(`${variable} must be an integer between 1 and 65535, got ${JSON.stringify(raw)}.`);
  }
  const port = Number(raw);
  if (port < 1 || port > 65535) {
    throw new Error(`${variable} must be between 1 and 65535, got ${port}.`);
  }
  return port;
}

/**
 * Resolve loopback endpoints for a browser gate.
 *
 * @param {object} input
 * @param {string} input.backendVariable environment variable for the backend
 * @param {number} input.backendDefault historical backend port
 * @param {string} input.uiVariable environment variable for Vite
 * @param {number} input.uiDefault historical UI port
 * @param {NodeJS.ProcessEnv} [input.env]
 */
export function resolveGateEndpoints({
  backendVariable,
  backendDefault,
  uiVariable,
  uiDefault,
  env = process.env,
}) {
  const backendPort = readPort(env, backendVariable, backendDefault);
  const uiPort = readPort(env, uiVariable, uiDefault);
  if (backendPort === uiPort) {
    throw new Error(
      `${backendVariable} and ${uiVariable} resolve to the same port ${backendPort}; ` +
        'choose distinct loopback ports.',
    );
  }
  return Object.freeze({
    backendPort,
    uiPort,
    backend: `${LOOPBACK}:${backendPort}`,
    ui: `${LOOPBACK}:${uiPort}`,
    baseURL: `http://${LOOPBACK}:${uiPort}`,
  });
}
