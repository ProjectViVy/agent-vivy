import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
    exclude: ['agent-diva-source/**', 'node_modules/**', 'dist/**'],
  },
});
