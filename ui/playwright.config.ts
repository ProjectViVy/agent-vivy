// Playwright config for the D4(b) UI smoke. The webServer is the real
// Go binary (ui/../vivy.exe) started in an isolated e2e workdir that
// holds a generated mock-provider config.yaml. The workdir must exist
// before Playwright starts the webServer, so it is prepared here at
// config-load time (globalSetup would run too late). VIVY_ADDR pins the
// listen address so the run never collides with the 8787 real-provider
// dev server.

import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "@playwright/test";
import { E2E_ADDR, e2eWorkdir, prepareE2eWorkdir } from "./e2e/global-setup";

// Worker processes reload this config while vivy already holds the
// workdir: prepare exactly once per invocation, in the first loader.
if (!process.env.VIVY_E2E_PREPARED) {
  prepareE2eWorkdir();
  process.env.VIVY_E2E_PREPARED = "1";
}

const here = path.dirname(fileURLToPath(import.meta.url)); // ui/
const repoRoot = path.resolve(here, "..");
const vivyExe = path.join(repoRoot, "vivy.exe");
const addr = E2E_ADDR;

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  workers: 1, // one vivy process, one session timeline
  retries: 0,
  reporter: "list",
  use: {
    baseURL: `http://${addr}`,
  },
  webServer: {
    command: JSON.stringify(vivyExe),
    cwd: e2eWorkdir,
    url: `http://${addr}/healthz`,
    env: { VIVY_ADDR: addr },
    reuseExistingServer: false,
    timeout: 15_000,
  },
});
