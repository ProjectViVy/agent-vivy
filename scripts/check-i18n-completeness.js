#!/usr/bin/env node
/**
 * From the repository root: node scripts/check-i18n-completeness.js
 * From ui/: node ../scripts/check-i18n-completeness.js
 * Uses the locked UI TypeScript parser, with no new runtime dependency.
 */
import { readFileSync, readdirSync } from 'node:fs';
import { resolve, dirname, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import { runInNewContext } from 'node:vm';

const scriptRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const require = createRequire(resolve(scriptRoot, 'ui/package.json'));
const ts = require('typescript');
const root = process.argv[2] === '--root' ? resolve(process.argv[3]) : scriptRoot;
const errors = [];
const han = /\p{Script=Han}/u;
const placeholders = (value) => [...value.matchAll(/\{\{\s*(\w+)\s*\}\}/g)].map((match) => match[1]).sort();

function flatten(value, prefix = '', result = Object.create(null)) {
  if (typeof value === 'string') {
    if (!value.trim()) errors.push('Empty translation: ' + prefix);
    result[prefix] = value;
  } else if (value && typeof value === 'object') {
    for (const [key, child] of Object.entries(value)) flatten(child, prefix ? prefix + '.' + key : key, result);
  } else {
    errors.push('Non-string translation: ' + prefix);
  }
  return result;
}
function parse(file) {
  const source = ts.createSourceFile(file, readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true);
  for (const diagnostic of source.parseDiagnostics) errors.push('Parse error: ' + file + ': ' + ts.flattenDiagnosticMessageText(diagnostic.messageText, ' '));
  return source;
}
function catalog(locale) {
  const source = parse(resolve(root, 'ui/src/i18n/' + locale + '.ts'));
  function duplicates(node) {
    if (ts.isObjectLiteralExpression(node)) {
      const seen = new Set();
      for (const prop of node.properties) {
        const key = prop.name?.getText(source);
        if (key && seen.has(key)) errors.push('Duplicate key in ' + locale + ': ' + key);
        seen.add(key);
      }
    }
    ts.forEachChild(node, duplicates);
  }
  duplicates(source);
  const code = ts.transpileModule(source.text, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText;
  const context = { exports: {} };
  runInNewContext(code, context, { timeout: 1000 });
  return flatten(context.exports[locale]);
}

// This module exclusively constructs synthetic user/model/tool event payloads.
const dataFiles = new Set(['ui/src/components/trajectory/trajectory-demo-data.ts']);
// Exact value exclusions, never blanket exclusions for host component directories.
const exactHanData = {
  // Protocol recognition tokens, not displayed recovery messages.
  'ui/src/lib/failure.ts': new Set(['控制面', '密钥未配置', '没有可用的 api key', '无法连接！请检查供应商配置！']),
  // Input matching keys, not host labels or generated output.
  'ui/src/lib/demo-api.ts': new Set(['你好', '帮我创建一个项目', '默认']),
  'ui/src/i18n/index.ts': new Set(['简体中文']), // Native language self-name.
  'ui/src/components/settings/channel-platforms.ts': new Set(['飞书', '钉钉']), // Channel brands.
  'ui/src/components/settings/channel-icons.tsx': new Set(['飞书', '钉钉']),
};
// Brand marks, protocol names, license/version/numeric notation and syntax examples.
const exactJsxData = new Set(['Vivy', 'VIVY', 'V', 'Project ViVY', 'HTTP', 'STDIO', 'MIT', 'projectViVY', 'tokens', 'r', 'MCP_DOCS_TOKEN', 'npx', 'CHILD_VAR', 'HOST_VAR']);
function isJsxData(value) {
  return !/[\p{L}]/u.test(value) || exactJsxData.has(value) || /^https?:\/\//.test(value);
}
function scan(file, english) {
  const name = relative(root, file).replaceAll('\\', '/');
  if (/\.test\.[^.]+$/.test(name) || name === 'ui/src/routeTree.gen.ts' ||
      name === 'ui/src/i18n/en.ts' || name === 'ui/src/i18n/zh.ts' || dataFiles.has(name)) return;
  const source = parse(file);
  function visit(node) {
    const location = name + ':' + (source.getLineAndCharacterOfPosition(node.getStart(source)).line + 1);
    // AST excludes comments, including multiline JSX comments, but includes template chunks.
    if (ts.isStringLiteralLike(node) || ts.isTemplateHead(node) || ts.isTemplateMiddle(node) || ts.isTemplateTail(node) || ts.isJsxText(node)) {
      const value = node.text;
      if (han.test(value) && !exactHanData[name]?.has(value)) errors.push('Uncatalogued Han: ' + location + ': ' + value.trim());
      const parent = ts.isJsxExpression(node.parent) ? node.parent.parent : node.parent;
      const visibleAttribute = ts.isJsxAttribute(parent) && ['aria-label', 'title', 'placeholder', 'alt'].includes(parent.name.getText(source));
      const visibleExpression = ts.isJsxExpression(node.parent) && (ts.isJsxElement(parent) || ts.isJsxFragment(parent));
      if ((ts.isJsxText(node) || visibleAttribute || visibleExpression) && !isJsxData(value.trim())) errors.push('Uncatalogued JSX: ' + location + ': ' + value.trim());
    }
    if (ts.isCallExpression(node) && node.expression.getText(source) === 't' && node.arguments[0] && ts.isStringLiteralLike(node.arguments[0])) {
      const key = node.arguments[0].text;
      if (!Object.hasOwn(english, key)) errors.push('Unknown translation: ' + key + ' at ' + location);
    }
    ts.forEachChild(node, visit);
  }
  visit(source);
}
function walk(dir, english) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const file = resolve(dir, entry.name);
    if (entry.isDirectory()) walk(file, english);
    else if (/\.tsx?$/.test(entry.name)) scan(file, english);
  }
}
try {
  const en = catalog('en');
  const zh = catalog('zh');
  for (const key of Object.keys(en)) {
    if (!Object.hasOwn(zh, key)) errors.push('Missing in zh: ' + key);
    else if (JSON.stringify(placeholders(en[key])) !== JSON.stringify(placeholders(zh[key]))) errors.push('Placeholder mismatch: ' + key);
  }
  for (const key of Object.keys(zh)) if (!Object.hasOwn(en, key)) errors.push('Missing in en: ' + key);
  walk(resolve(root, 'ui/src'), en);
  if (errors.length) {
    console.error(errors.join('\n'));
    process.exitCode = 1;
  } else {
    const count = (dict) => Object.values(dict).reduce((sum, value) => sum + placeholders(value).length, 0);
    console.log('PASS: en=' + Object.keys(en).length + ' keys / ' + count(en) + ' placeholders; zh=' + Object.keys(zh).length + ' keys / ' + count(zh) + ' placeholders; runtime copy audit clean.');
  }
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
}
