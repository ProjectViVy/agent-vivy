import { describe, expect, it } from 'vitest';
import type { FaceSessionTree, FaceSessionTreeNode } from '@vivy/ui-sdk';
import { flattenTree } from './tree';

const node = (id: string, created: number, parent?: string): FaceSessionTreeNode => ({
  session_id: id,
  title: id,
  created_at: created,
  updated_at: created,
  parent_session_id: parent,
});

describe('flattenTree', () => {
  it('lays out children depth-first under their parents', () => {
    const tree: FaceSessionTree = {
      nodes: [
        node('root', 1),
        node('child-b', 3, 'root'),
        node('grand', 4, 'child-a'),
        node('child-a', 2, 'root'),
        node('orphan', 5, 'ghost'),
      ],
      edges: [
        { from: 'root', to: 'child-a', kind: 'fork' },
        { from: 'root', to: 'child-b', kind: 'fork' },
        { from: 'child-a', to: 'grand', kind: 'fork' },
      ],
    };
    const flat = flattenTree(tree);
    expect(flat.map((row) => row.node.session_id)).toEqual(['root', 'child-a', 'grand', 'child-b', 'orphan']);
    expect(flat.map((row) => row.depth)).toEqual([0, 1, 2, 1, 0]);
  });

  it('sorts roots by created_at and survives parent cycles', () => {
    const tree: FaceSessionTree = {
      nodes: [
        node('late', 10),
        node('early', 1),
        node('cycle-a', 5, 'cycle-b'),
        node('cycle-b', 6, 'cycle-a'),
      ],
      edges: [],
    };
    const flat = flattenTree(tree);
    expect(flat[0]?.node.session_id).toBe('early');
    // Both cycle members resolve (each sees the other's id as a parent in
    // the snapshot) and land in created order, never dropped.
    expect(flat.map((row) => row.node.session_id)).toContain('cycle-a');
    expect(flat.map((row) => row.node.session_id)).toContain('cycle-b');
    expect(flat).toHaveLength(4);
  });
});
