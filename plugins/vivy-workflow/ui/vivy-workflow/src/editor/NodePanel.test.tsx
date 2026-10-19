// @vitest-environment happy-dom
//
// The properties panel is the only authoring surface for a call node's
// bindings. INOFY resolves them from `node.inputs`; the host executor rejects
// unknown `config` fields, so writing them under config loses the edit or
// fails the node. These tests pin the canonical field and the shape gate.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { NodePanel } from './NodePanel';
import type { Artifact, Node } from '../studio/schema';

const t = (key: string) => key;

function artifactWith(node: Node): Artifact {
  return {
    definition: {
      schema_version: 'inofy.workflow/v1',
      graph: {
        nodes: [node],
        edges: [],
        exits: ['a'],
        outputs: { answer: { source: 'a', pointer: '/result' } },
      },
    },
  };
}

const callNode: Node = {
  id: 'a',
  kind: 'call',
  type: 'vivy.child-task@1',
  config: { task: 'first half', tool_names: ['echo_info'] },
};

function setTextarea(el: HTMLTextAreaElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value')!.set!;
  setter.call(el, value);
  el.dispatchEvent(new window.Event('input', { bubbles: true }));
}

async function blur(el: HTMLElement) {
  await act(async () => {
    el.dispatchEvent(new window.FocusEvent('focusout', { bubbles: true }));
  });
}

describe('NodePanel canonical node inputs', () => {
  let container: HTMLElement;
  let root: Root;
  let changes: Artifact[];

  const render = async (node: Node) => {
    await act(async () =>
      root.render(
        <NodePanel
          artifact={artifactWith(node)}
          node={node}
          descriptor={undefined}
          t={t}
          onChange={(next) => changes.push(next)}
          onDelete={vi.fn()}
        />,
      ),
    );
  };

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
    changes = [];
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    vi.unstubAllGlobals();
  });

  it('authors bindings on node.inputs and leaves node.config untouched', async () => {
    await render(callNode);
    const textarea = container.querySelector<HTMLTextAreaElement>('#wf-node-inputs')!;
    expect(textarea.value).toBe('{}');

    await act(async () => {
      setTextarea(
        textarea,
        JSON.stringify(
          {
            literal_text: { literal: 'PG literal' },
            root_text: { source: 'input', pointer: '/topic' },
            upstream: { source: 'a', pointer: '/result' },
          },
          null,
          2,
        ),
      );
    });
    await blur(textarea);

    expect(changes).toHaveLength(1);
    const node = changes[0]!.definition.graph.nodes[0]!;
    expect(node.inputs).toEqual({
      literal_text: { literal: 'PG literal' },
      root_text: { source: 'input', pointer: '/topic' },
      upstream: { source: 'a', pointer: '/result' },
    });
    expect(node.config).toEqual({ task: 'first half', tool_names: ['echo_info'] });
    expect(container.textContent).not.toContain('inputs must be a JSON object');
  });

  it('accepts a whole-packet pointer and a null literal', async () => {
    await render(callNode);
    const textarea = container.querySelector<HTMLTextAreaElement>('#wf-node-inputs')!;
    await act(async () => {
      setTextarea(textarea, JSON.stringify({ whole: { source: 'a', pointer: '' }, none: { literal: null } }));
    });
    await blur(textarea);

    expect(changes).toHaveLength(1);
    expect(changes[0]!.definition.graph.nodes[0]!.inputs).toEqual({
      whole: { source: 'a', pointer: '' },
      none: { literal: null },
    });
  });

  it('rejects a non-object input map without mutating the artifact', async () => {
    await render(callNode);
    const textarea = container.querySelector<HTMLTextAreaElement>('#wf-node-inputs')!;
    await act(async () => setTextarea(textarea, '[1,2]'));
    await blur(textarea);

    expect(changes).toHaveLength(0);
    expect(container.textContent).toContain('inputs must be a JSON object');

    await act(async () => setTextarea(textarea, 'not json'));
    await blur(textarea);
    expect(changes).toHaveLength(0);
  });

  it('rejects a binding with an unsupported field or an empty binding', async () => {
    await render(callNode);
    const textarea = container.querySelector<HTMLTextAreaElement>('#wf-node-inputs')!;

    await act(async () => setTextarea(textarea, JSON.stringify({ x: { source: 'a', pointer: '/r', extra: true } })));
    await blur(textarea);
    expect(changes).toHaveLength(0);
    expect(container.textContent).toContain('unsupported field "extra"');

    await act(async () => setTextarea(textarea, JSON.stringify({ x: {} })));
    await blur(textarea);
    expect(changes).toHaveLength(0);
    expect(container.textContent).toContain('must declare at least one of literal, source, pointer');

    await act(async () => setTextarea(textarea, JSON.stringify({ x: { source: 'a' }, y: { source: '' } })));
    await blur(textarea);
    expect(changes).toHaveLength(0);
    expect(container.textContent).toContain('"source" must be a non-empty string');
  });

  it('drops the field when the map is emptied instead of persisting an empty object', async () => {
    const bound: Node = { ...callNode, inputs: { upstream: { source: 'a', pointer: '/result' } } };
    await render(bound);
    const textarea = container.querySelector<HTMLTextAreaElement>('#wf-node-inputs')!;
    expect(textarea.value).toContain('/result');

    await act(async () => setTextarea(textarea, '{}'));
    await blur(textarea);

    expect(changes).toHaveLength(1);
    const node = changes[0]!.definition.graph.nodes[0]!;
    expect(node).not.toHaveProperty('inputs');
    expect(node.config).toEqual({ task: 'first half', tool_names: ['echo_info'] });
  });

  it('refreshes the draft when the selected node or its canonical inputs change', async () => {
    await render(callNode);
    const textarea = container.querySelector<HTMLTextAreaElement>('#wf-node-inputs')!;

    const next: Node = { ...callNode, inputs: { one: { literal: 1 } } };
    await render(next);

    expect(container.querySelector<HTMLTextAreaElement>('#wf-node-inputs')!.value).toBe(
      JSON.stringify({ one: { literal: 1 } }, null, 2),
    );
    expect(textarea).toBeTruthy();
  });
});
