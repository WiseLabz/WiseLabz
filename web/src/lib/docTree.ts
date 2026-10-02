import type { DocNode } from '../api/model';

export function flattenDocTree(node: DocNode | undefined): DocNode[] {
  if (!node) return [];
  return [node, ...(node.children ?? []).flatMap(flattenDocTree)];
}
