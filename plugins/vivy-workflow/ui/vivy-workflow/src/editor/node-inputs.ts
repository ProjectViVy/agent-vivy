// Canonical node-input authoring for the properties panel.
//
// INOFY resolves a call node's bindings from `node.inputs`
// (`inofy.workflow/v1` Node.inputs), never from `node.config`: the host
// executor decodes config strictly (`task`/`tool_names` only), so an inputs
// map written under config is ignored or rejected. These helpers keep the
// panel on that single home and reject shapes the engine can never execute —
// the backend remains the full semantic authority.

import type { Binding, Node } from '../studio/schema';

const BINDING_FIELDS = ['literal', 'source', 'pointer'] as const;

export type BindingMapResult =
  | { ok: true; value: Record<string, Binding> }
  | { ok: false; error: string };

/** The canonical authored bindings of a node; absent/garbage reads as empty. */
export function readNodeInputs(node: Node): Record<string, Binding> {
  const raw = node.inputs;
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return {};
  return raw;
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function bindingError(key: string, detail: string): string {
  return `inputs["${key}"] ${detail}`;
}

function validateBinding(key: string, entry: unknown): string | null {
  if (!isPlainObject(entry)) {
    return bindingError(key, 'must be a binding object such as {"source":"a","pointer":"/result"}');
  }
  const keys = Object.keys(entry);
  if (keys.length === 0) {
    return bindingError(key, 'must declare at least one of literal, source, pointer');
  }
  const unknown = keys.find((field) => !BINDING_FIELDS.includes(field as (typeof BINDING_FIELDS)[number]));
  if (unknown) {
    return bindingError(key, `has unsupported field "${unknown}" (allowed: literal, source, pointer)`);
  }
  if ('source' in entry && (typeof entry.source !== 'string' || entry.source.trim() === '')) {
    return bindingError(key, '"source" must be a non-empty string');
  }
  if ('pointer' in entry && typeof entry.pointer !== 'string') {
    return bindingError(key, '"pointer" must be a string (use "" for the whole packet)');
  }
  return null;
}

/** Validate a parsed JSON value as the node `inputs` map. */
export function parseBindingMapValue(parsed: unknown): BindingMapResult {
  if (!isPlainObject(parsed)) {
    return { ok: false, error: 'inputs must be a JSON object' };
  }
  const value: Record<string, Binding> = {};
  for (const key of Object.keys(parsed)) {
    const error = validateBinding(key, parsed[key]);
    if (error) return { ok: false, error };
    value[key] = parsed[key] as Binding;
  }
  return { ok: true, value };
}

/** Parse the panel's JSON draft into a node `inputs` map. */
export function parseBindingMap(text: string): BindingMapResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text) as unknown;
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : String(e) };
  }
  return parseBindingMapValue(parsed);
}

/** Replace `node.inputs`; an empty map drops the field instead of persisting `{}`. */
export function setNodeInputs(node: Node, inputs: Record<string, Binding>): Node {
  if (Object.keys(inputs).length === 0) {
    const { inputs: _dropped, ...rest } = node;
    return rest as Node;
  }
  return { ...node, inputs };
}

/** Format canonical inputs for the JSON draft. */
export function formatNodeInputs(node: Node): string {
  return JSON.stringify(readNodeInputs(node), null, 2);
}
