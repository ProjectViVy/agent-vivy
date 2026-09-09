import assert from 'node:assert/strict';
import test from 'node:test';

import { validateContractData } from './check-i18n-cross-face.js';

const contract = {
  schemaVersion: 1,
  locales: ['en', 'zh'],
  sharedUnits: [
    {
      id: 'session.list.title',
      arguments: [],
      web: 'layout.sessions',
      tui: 'vivy.tui.sessions.title',
    },
  ],
  faceSpecific: {
    webKeys: ['webOnly.label'],
    tuiKeys: ['vivy.tui.only.label'],
  },
};

const validCatalogs = {
  web: {
    en: { 'layout.sessions': 'Sessions', 'webOnly.label': 'Web only' },
    zh: { 'layout.sessions': '会话', 'webOnly.label': '仅 Web' },
  },
  tui: {
    en: { 'vivy.tui.sessions.title': 'Sessions', 'vivy.tui.only.label': 'TUI only' },
    zh: { 'vivy.tui.sessions.title': '会话', 'vivy.tui.only.label': '仅 TUI' },
  },
};

test('accepts complete shared projections and explicit face-specific units', () => {
  assert.deepEqual(validateContractData(contract, validCatalogs), []);
});

test('rejects a missing shared projection', () => {
  const catalogs = structuredClone(validCatalogs);
  delete catalogs.tui.zh['vivy.tui.sessions.title'];
  assert.match(validateContractData(contract, catalogs).join('\n'), /Missing zh TUI projection/);
});

test('rejects placeholder drift across faces', () => {
  const catalogs = structuredClone(validCatalogs);
  contract.sharedUnits[0].arguments = ['count'];
  catalogs.web.en['layout.sessions'] = '{{count}} sessions';
  catalogs.web.zh['layout.sessions'] = '{{count}} 个会话';
  assert.match(validateContractData(contract, catalogs).join('\n'), /placeholder mismatch.*TUI/i);
  contract.sharedUnits[0].arguments = [];
});

test('rejects unclassified face-specific keys', () => {
  const catalogs = structuredClone(validCatalogs);
  catalogs.web.en['surprise.label'] = 'Surprise';
  catalogs.web.zh['surprise.label'] = '意外';
  assert.match(validateContractData(contract, catalogs).join('\n'), /Unclassified Web key: surprise\.label/);
});

test('rejects an unmapped Web key under an existing face-specific family', () => {
  const catalogs = structuredClone(validCatalogs);
  catalogs.web.en['webOnly.surprise'] = 'Surprise';
  catalogs.web.zh['webOnly.surprise'] = '意外';
  assert.match(validateContractData(contract, catalogs).join('\n'), /Unclassified Web key: webOnly\.surprise/);
});

test('rejects an unmapped TUI key under an existing face-specific family', () => {
  const catalogs = structuredClone(validCatalogs);
  catalogs.tui.en['vivy.tui.only.surprise'] = 'Surprise';
  catalogs.tui.zh['vivy.tui.only.surprise'] = '意外';
  assert.match(validateContractData(contract, catalogs).join('\n'), /Unclassified TUI key: vivy\.tui\.only\.surprise/);
});

test('rejects a declared Web face-specific key missing from the catalogs', () => {
  const catalogs = structuredClone(validCatalogs);
  delete catalogs.web.en['webOnly.label'];
  delete catalogs.web.zh['webOnly.label'];
  assert.match(validateContractData(contract, catalogs).join('\n'), /Missing Web face-specific key: webOnly\.label/);
});

test('rejects a declared TUI face-specific key missing from the catalogs', () => {
  const catalogs = structuredClone(validCatalogs);
  delete catalogs.tui.en['vivy.tui.only.label'];
  delete catalogs.tui.zh['vivy.tui.only.label'];
  assert.match(validateContractData(contract, catalogs).join('\n'), /Missing TUI face-specific key: vivy\.tui\.only\.label/);
});
