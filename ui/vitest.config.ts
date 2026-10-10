import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vitest/config';

// Test resolution must match the development and production builds
// (`vite.config.ts`) exactly. `@vivy/ui-sdk` is consumed from source there, so
// resolving it here through `node_modules` would run the tests against whatever
// copy `pnpm install` last linked under `ui/node_modules/@vivy/ui-sdk` — a stale
// SDK can then disagree with the app the developer is actually running.
export default defineConfig({
  esbuild: { jsx: 'automatic' },
  server: { fs: { allow: [fileURLToPath(new URL('..', import.meta.url))] } },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@vivy/ui-sdk': fileURLToPath(new URL('../sdk/ui/src', import.meta.url)),
      '@vivy/ui-assembly': fileURLToPath(new URL('./src/generated/assembly.ts', import.meta.url)),
      '@vivy/generated-assembly': fileURLToPath(new URL('./src/generated/assembly.ts', import.meta.url)),
      // Module sources under ../plugins resolve bare imports from their own
      // package.json (no installed copies). Deps that exist only for the
      // staged UI build resolve from the ui install here, matching prod.
      'react-markdown': fileURLToPath(new URL('./node_modules/react-markdown/index.js', import.meta.url)),
      'remark-gfm': fileURLToPath(new URL('./node_modules/remark-gfm/index.js', import.meta.url)),
    },
    // The SDK source tree carries its own React devDependency. Without this the
    // aliased source would load a second React copy into the same test tree and
    // every hook the SDK re-exports would fail with a null dispatcher.
    dedupe: ['react', 'react-dom', 'lucide-react'],
  },
  test: {
    // Run Module regressions from authoritative source and omit duplicate
    // staged mask tests below.
    include: [
      'src/**/*.test.ts', 'src/**/*.test.tsx',
      '../plugins/vivy-masks-ui/ui/vivy-masks/src/**/*.test.ts',
      '../plugins/vivy-masks-ui/ui/vivy-masks/src/**/*.test.tsx',
      '../plugins/vivy-persona/ui/vivy-persona/src/**/*.test.ts',
      '../plugins/vivy-persona/ui/vivy-persona/src/**/*.test.tsx',
      '../plugins/coding/session-tree/ui/session-tree/src/**/*.test.ts',
      '../plugins/coding/session-tree/ui/session-tree/src/**/*.test.tsx',
      '../plugins/vivy-notebook/ui/vivy-notebook/src/**/*.test.ts',
      '../plugins/vivy-notebook/ui/vivy-notebook/src/**/*.test.tsx',
    ],
    exclude: [
      'node_modules/**',
      'dist/**',
      'src/generated/ui/vivy-masks/src/**/*.test.*',
      'src/generated/ui/vivy-persona/src/**/*.test.*',
      'src/generated/ui/session-tree/src/**/*.test.*',
      'src/generated/ui/vivy-notebook/src/**/*.test.*',
    ],
  },
});
