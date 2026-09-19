import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vitest/config';

// Test resolution must match the development and production builds
// (`vite.config.ts`) exactly. `@vivy/ui-sdk` is consumed from source there, so
// resolving it here through `node_modules` would run the tests against whatever
// copy `pnpm install` last linked under `ui/node_modules/@vivy/ui-sdk` — a stale
// SDK can then disagree with the app the developer is actually running.
export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@vivy/ui-sdk': fileURLToPath(new URL('../sdk/ui/src', import.meta.url)),
      '@vivy/ui-assembly': fileURLToPath(new URL('./src/generated/assembly.ts', import.meta.url)),
      '@vivy/generated-assembly': fileURLToPath(new URL('./src/generated/assembly.ts', import.meta.url)),
    },
    // The SDK source tree carries its own React devDependency. Without this the
    // aliased source would load a second React copy into the same test tree and
    // every hook the SDK re-exports would fail with a null dispatcher.
    dedupe: ['react', 'react-dom'],
  },
  test: {
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
    exclude: ['node_modules/**', 'dist/**'],
  },
});