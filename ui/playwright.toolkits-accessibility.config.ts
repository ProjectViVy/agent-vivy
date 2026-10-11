import { defineConfig } from '@playwright/test';
import toolkitConfig from './playwright.toolkits.config';

export default defineConfig({
  ...toolkitConfig,
  testMatch: 'toolkits-accessibility.spec.ts',
});
