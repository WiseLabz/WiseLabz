/**
 * Sync-ownership markers (#478). The backend wraps every generated section of
 * a doc as
 *
 *   <!-- wl:gen key="snap.containers" h="3fa9c0d1e2b4" -->
 *   …body…
 *   <!-- /wl:gen -->
 *
 * Text outside these blocks is human-owned and never touched by sync; edits
 * inside a block are flagged for review on the next sync. These rules mirror
 * backend/internal/doc/blocks.go: anything malformed counts as plain text.
 */
const OPEN_RE = /^<!-- wl:gen key="([A-Za-z0-9._-]+)" h="[0-9a-f]{12}" -->$/;
const CLOSE = '<!-- /wl:gen -->';

/** True for an open or close marker as it appears in rendered markdown. */
export function isGenMarker(text: string): boolean {
  const t = text.trim();
  return t === CLOSE || OPEN_RE.test(t);
}

export interface GenBlockRange {
  key: string;
  /** 1-based line numbers of the open and close marker lines. */
  openLine: number;
  closeLine: number;
}

/** Line ranges of the well-formed generated blocks in content. */
export function findGenBlocks(content: string): GenBlockRange[] {
  const lines = content.split('\n');
  const blocks: GenBlockRange[] = [];
  for (let i = 0; i < lines.length; i++) {
    const m = OPEN_RE.exec(lines[i]);
    if (!m) continue;
    for (let j = i + 1; j < lines.length; j++) {
      if (lines[j] === CLOSE && j > i + 1) {
        blocks.push({ key: m[1], openLine: i + 1, closeLine: j + 1 });
        i = j;
        break;
      }
      if (OPEN_RE.test(lines[j])) break;
    }
  }
  return blocks;
}

interface MdNode {
  type: string;
  value?: string;
  children?: MdNode[];
}

function strip(node: MdNode) {
  if (!node.children) return;
  node.children = node.children.filter((c) => !(c.type === 'html' && isGenMarker(c.value ?? '')));
  node.children.forEach(strip);
}

/** remark plugin dropping the wl:gen marker comments from the rendered doc. */
export function remarkStripGenMarkers() {
  return (tree: MdNode) => strip(tree);
}
