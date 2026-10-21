import type { FaceExportReadResult, FaceSessionExportResult, FaceSessionTree, FaceSessionTreeNode } from '@vivy/ui-sdk';

/** One rendered row: the node plus its indentation depth in the layout. */
export interface TreeRow {
  readonly node: FaceSessionTreeNode;
  readonly depth: number;
}

const byCreated = (a: FaceSessionTreeNode, b: FaceSessionTreeNode) => a.created_at - b.created_at;

/**
 * Lays the kernel's session/tree read model out depth-first for the page.
 * Children follow their parent_session_id edges sorted by created_at;
 * roots are nodes whose parent is absent from the snapshot; a visited set
 * guards against cycles and any unreachable node is appended flat. The
 * page only renders — the kernel owns the graph.
 */
export function flattenTree(tree: FaceSessionTree): TreeRow[] {
  const rows: TreeRow[] = [];
  const nodes = tree.nodes ?? [];
  const byId = new Map(nodes.map((node) => [node.session_id, node]));
  const children = new Map<string, FaceSessionTreeNode[]>();
  const roots: FaceSessionTreeNode[] = [];
  for (const node of nodes) {
    const parent = node.parent_session_id ? byId.get(node.parent_session_id) : undefined;
    if (parent) {
      const list = children.get(parent.session_id) ?? [];
      list.push(node);
      children.set(parent.session_id, list);
    } else {
      roots.push(node);
    }
  }
  const visit = (node: FaceSessionTreeNode, depth: number, seen: Set<string>) => {
    if (seen.has(node.session_id)) return;
    seen.add(node.session_id);
    rows.push({ node, depth });
    const kids = (children.get(node.session_id) ?? []).slice().sort(byCreated);
    for (const kid of kids) visit(kid, depth + 1, seen);
  };
  const seen = new Set<string>();
  for (const root of roots.slice().sort(byCreated)) visit(root, 0, seen);
  for (const node of nodes.slice().sort(byCreated)) {
    if (!seen.has(node.session_id)) visit(node, 0, seen);
  }
  return rows;
}

/** Checks the immutable publication identity and bounded complete read before saving. */
export function decodeExport(result: FaceSessionExportResult, read: FaceExportReadResult): Uint8Array {
  const maxBytes = 32 * 1024 * 1024;
  if (!result.name || read.name !== result.name || !Number.isSafeInteger(result.size) || result.size < 0 || result.size > maxBytes || read.size !== result.size) {
    throw new Error('Export response does not match the published artifact');
  }
  if (read.data_base64.length > Math.ceil(result.size / 3) * 4) throw new Error('Export exceeds its declared size');
  const binary = atob(read.data_base64);
  if (binary.length !== result.size) throw new Error('Export is incomplete');
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes;
}
