// Starter definition for a new draft: one governed child-task node as the
// sole exit, with `result` bound to its whole output packet (RFC 6901 root
// pointer) so the graph is immediately valid under host admission
// (len(graph.outputs) >= 1). Node id avoids the reserved "start"/"end".

import type { Artifact } from './studio/schema';

export const CHILD_TASK_TYPE = 'vivy.child-task@1';
export const ENTRY_NODE_ID = 'begin';

export function blankArtifact(title: string): Artifact {
  return {
    definition: {
      schema_version: 'inofy.workflow/v1',
      graph: {
        nodes: [
          {
            id: ENTRY_NODE_ID,
            kind: 'call',
            type: CHILD_TASK_TYPE,
            config: { task: title },
          },
        ],
        edges: [],
        exits: [ENTRY_NODE_ID],
        outputs: { result: { source: ENTRY_NODE_ID, pointer: '' } },
      },
    },
    presentation: {
      title,
      layout: {
        positions: { [ENTRY_NODE_ID]: { x: 80, y: 80 } },
        viewport: { x: 0, y: 0, zoom: 1 },
      },
    },
  };
}

// The workflow id is the address and primary key: reject only inputs that
// fail unconditionally (blank, path separators, cursor separators).
export function validateWorkflowID(id: string): string | null {
  if (id.trim() === '') return 'empty';
  if (id !== id.trim()) return 'whitespace';
  if (/[/\\?#:]/.test(id)) return 'separator';
  if (id.length > 64) return 'length';
  return null;
}
