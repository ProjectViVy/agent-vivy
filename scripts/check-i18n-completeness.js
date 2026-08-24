#!/usr/bin/env node
/**
 * I18N Completeness Checker
 * 
 * This script verifies that zh-CN and en translation keys are synchronized.
 * Run with: node scripts/check-i18n-completeness.js
 */

import { readFileSync } from "fs";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// Read the additional messages file
const additionalMessagesPath = resolve(__dirname, "../ui/src/app/i18n-additional.ts");
const content = readFileSync(additionalMessagesPath, "utf-8");

// Simple extraction of keys (for demonstration; in production use a proper TS parser)
function extractKeys(objStr) {
  const keys = new Set();
  // Match only top-level keys in the object (indented with 2-4 spaces)
  const regex = /^\s{2,4}(\w+):\s*["']/gm;
  let match;
  while ((match = regex.exec(objStr)) !== null) {
    keys.add(match[1]);
  }
  return keys;
}

// Extract zh-CN and en sections
const zhMatch = content.match(/"zh-CN":\s*\{([\s\S]*?)\n\s*\},\n\s*en:/);
const enMatch = content.match(/en:\s*\{([\s\S]*?)\n\s*\},?\n\s*\};/);

if (!zhMatch || !enMatch) {
  console.error("❌ Could not parse translation sections");
  process.exit(1);
}

const zhKeys = extractKeys(zhMatch[1]);
const enKeys = extractKeys(enMatch[1]);

const missingInZh = [...enKeys].filter(key => !zhKeys.has(key));
const missingInEn = [...zhKeys].filter(key => !enKeys.has(key));

let hasErrors = false;

if (missingInZh.length > 0) {
  console.error(`❌ Missing ${missingInZh.length} keys in zh-CN:`);
  missingInZh.forEach(key => console.error(`   - ${key}`));
  hasErrors = true;
}

if (missingInEn.length > 0) {
  console.error(`❌ Missing ${missingInEn.length} keys in en:`);
  missingInEn.forEach(key => console.error(`   - ${key}`));
  hasErrors = true;
}

if (!hasErrors) {
  console.log(`✅ All translations are complete (${zhKeys.size} keys in both languages)`);
} else {
  process.exit(1);
}
