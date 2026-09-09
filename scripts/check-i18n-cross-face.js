#!/usr/bin/env node
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { runInNewContext } from 'node:vm';

const scriptRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const placeholders = (value) => [...value.matchAll(/\{\{\s*(\w+)\s*\}\}/g)].map((match) => match[1]).sort();

function flatten(value, prefix = '', result = Object.create(null)) {
  if (typeof value === 'string') result[prefix] = value;
  else if (value && typeof value === 'object') {
    for (const [key, child] of Object.entries(value)) flatten(child, prefix ? `${prefix}.${key}` : key, result);
  }
  return result;
}

function sameArguments(actual, expected) {
  return JSON.stringify(placeholders(actual)) === JSON.stringify([...expected].sort());
}

export function validateContractData(contract, catalogs) {
  const errors = [];
  if (contract.schemaVersion !== 1) errors.push(`Unsupported cross-face schema version: ${contract.schemaVersion}`);
  if (JSON.stringify(contract.locales) !== JSON.stringify(['en', 'zh'])) errors.push('Cross-face locales must be exactly en and zh');

  const ids = new Set();
  const sharedWeb = new Set();
  const sharedTui = new Set();
  for (const unit of contract.sharedUnits ?? []) {
    if (ids.has(unit.id)) errors.push(`Duplicate shared semantic ID: ${unit.id}`);
    ids.add(unit.id);
    if (sharedWeb.has(unit.web)) errors.push(`Duplicate Web projection: ${unit.web}`);
    if (sharedTui.has(unit.tui)) errors.push(`Duplicate TUI projection: ${unit.tui}`);
    sharedWeb.add(unit.web);
    sharedTui.add(unit.tui);
    for (const locale of contract.locales ?? []) {
      const webValue = catalogs.web[locale]?.[unit.web];
      const tuiValue = catalogs.tui[locale]?.[unit.tui];
      if (webValue === undefined) errors.push(`Missing ${locale} Web projection for ${unit.id}: ${unit.web}`);
      else if (!sameArguments(webValue, unit.arguments ?? [])) errors.push(`Placeholder mismatch in ${locale} Web projection for ${unit.id}`);
      if (tuiValue === undefined) errors.push(`Missing ${locale} TUI projection for ${unit.id}: ${unit.tui}`);
      else if (!sameArguments(tuiValue, unit.arguments ?? [])) errors.push(`Placeholder mismatch in ${locale} TUI projection for ${unit.id}`);
    }
  }

  for (const face of ['web', 'tui']) {
    const en = catalogs[face].en ?? {};
    const zh = catalogs[face].zh ?? {};
    for (const key of Object.keys(en)) {
      if (!(key in zh)) errors.push(`Missing zh ${face.toUpperCase()} key: ${key}`);
      else if (JSON.stringify(placeholders(en[key])) !== JSON.stringify(placeholders(zh[key]))) errors.push(`Placeholder mismatch in ${face.toUpperCase()} key: ${key}`);
    }
    for (const key of Object.keys(zh)) if (!(key in en)) errors.push(`Missing en ${face.toUpperCase()} key: ${key}`);
  }

  const webKeys = new Set(contract.faceSpecific?.webKeys ?? []);
  for (const key of Object.keys(catalogs.web.en ?? {})) {
    if (!sharedWeb.has(key) && !webKeys.has(key)) errors.push(`Unclassified Web key: ${key}`);
  }
  for (const key of webKeys) {
    if (!(key in (catalogs.web.en ?? {}))) errors.push(`Missing Web face-specific key: ${key}`);
  }
  const tuiKeys = new Set(contract.faceSpecific?.tuiKeys ?? []);
  for (const key of Object.keys(catalogs.tui.en ?? {})) {
    if (!sharedTui.has(key) && !tuiKeys.has(key)) errors.push(`Unclassified TUI key: ${key}`);
  }
  for (const key of tuiKeys) {
    if (!(key in (catalogs.tui.en ?? {}))) errors.push(`Missing TUI face-specific key: ${key}`);
  }
  return errors;
}

function loadWebCatalog(root, locale) {
  const require = createRequire(resolve(root, 'ui/package.json'));
  const ts = require('typescript');
  const source = readFileSync(resolve(root, `ui/src/i18n/${locale}.ts`), 'utf8');
  const code = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText;
  const context = { exports: {} };
  runInNewContext(code, context, { timeout: 1000 });
  return flatten(context.exports[locale]);
}

function loadTuiCatalog(root, locale) {
  const source = readFileSync(resolve(root, `sdk/tui/i18n/catalog_${locale}.go`), 'utf8');
  const result = Object.create(null);
  const entry = /^\s*"([^"]+)":\s+(?:`([^`]*)`|"((?:\\.|[^"])*)")/gm;
  for (const match of source.matchAll(entry)) {
    const value = match[2] ?? JSON.parse(`"${match[3]}"`);
    if (Object.hasOwn(result, match[1])) throw new Error(`Duplicate ${locale} TUI key: ${match[1]}`);
    result[match[1]] = value;
  }
  return result;
}

export function validateRepository(root = scriptRoot) {
  const contract = JSON.parse(readFileSync(resolve(root, 'scripts/i18n-cross-face-contract.json'), 'utf8'));
  const catalogs = {
    web: { en: loadWebCatalog(root, 'en'), zh: loadWebCatalog(root, 'zh') },
    tui: { en: loadTuiCatalog(root, 'en'), zh: loadTuiCatalog(root, 'zh') },
  };
  return { contract, catalogs, errors: validateContractData(contract, catalogs) };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    const { contract, errors } = validateRepository(process.argv[2] ? resolve(process.argv[2]) : scriptRoot);
    if (errors.length) {
      console.error(errors.join('\n'));
      process.exitCode = 1;
    } else {
      console.log(`PASS: ${contract.sharedUnits.length} shared semantic units; Web and TUI en/zh projections and arguments conform.`);
    }
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
