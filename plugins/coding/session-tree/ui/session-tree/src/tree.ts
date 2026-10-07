import type { FaceSessionTree, FaceSessionTreeNode } from '@vivy/ui-sdk';

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

/** Decodes one exports/read payload back to bytes. */
export function decodeBase64(value: string): Uint8Array {
  const binary = atob(value);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes;
}

/** SHA-256 hex of the downloaded bytes, compared against the export result. */
export async function sha256Hex(data: Uint8Array): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', data.slice().buffer);
  return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}
