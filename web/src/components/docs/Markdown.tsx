/**
 * Compact, dependency-free markdown renderer tuned for WiseLabz's generated docs:
 * headings, paragraphs, fenced code, blockquotes, unordered lists, and pipe
 * tables (the infra docs lean on tables). Inline: **bold** and `code`. Content
 * is project-owned, so this is deliberately small rather than a full CommonMark
 * engine. Reading column is capped to a comfortable measure.
 */
import { Fragment, type ReactNode } from 'react';

function inline(text: string, keyBase: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  // Split on `code` and **bold**, keeping delimiters.
  const re = /(`[^`]+`|\*\*[^*]+\*\*)/g;
  let last = 0;
  let m: RegExpExecArray | null;
  let k = 0;
  while ((m = re.exec(text))) {
    if (m.index > last) nodes.push(text.slice(last, m.index));
    const tok = m[0];
    if (tok.startsWith('`')) {
      nodes.push(
        <code
          key={`${keyBase}-${k++}`}
          className="rounded bg-[var(--color-canvas-sunken)] px-1.5 py-0.5 font-mono text-[0.85em] text-[var(--color-signal-bright)]"
        >
          {tok.slice(1, -1)}
        </code>,
      );
    } else {
      nodes.push(
        <strong key={`${keyBase}-${k++}`} className="font-semibold text-[var(--color-ink)]">
          {tok.slice(2, -2)}
        </strong>,
      );
    }
    last = m.index + tok.length;
  }
  if (last < text.length) nodes.push(text.slice(last));
  return nodes;
}

export function Markdown({ source }: { source: string }) {
  const lines = source.replace(/\r\n/g, '\n').split('\n');
  const blocks: ReactNode[] = [];
  let i = 0;
  let key = 0;

  while (i < lines.length) {
    const line = lines[i];

    // blank
    if (!line.trim()) {
      i++;
      continue;
    }

    // fenced code
    if (line.startsWith('```')) {
      const buf: string[] = [];
      i++;
      while (i < lines.length && !lines[i].startsWith('```')) buf.push(lines[i++]);
      i++; // closing fence
      blocks.push(
        <pre
          key={key++}
          className="my-3 overflow-x-auto rounded-lg border border-[var(--color-line-soft)] bg-[var(--color-canvas-sunken)] p-3 font-mono text-xs leading-relaxed text-[var(--color-ink-muted)]"
        >
          <code>{buf.join('\n')}</code>
        </pre>,
      );
      continue;
    }

    // headings
    const h = /^(#{1,4})\s+(.*)$/.exec(line);
    if (h) {
      const level = h[1].length;
      const cls =
        level === 1
          ? 'mt-1 mb-3 font-mono text-2xl font-semibold tracking-tight text-balance'
          : level === 2
            ? 'mb-2 mt-6 font-mono text-lg font-semibold tracking-tight'
            : 'mb-1.5 mt-5 font-mono text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-ink-muted)]';
      const Tag = (`h${level}` as 'h1' | 'h2' | 'h3' | 'h4');
      blocks.push(
        <Tag key={key++} className={`${cls} text-[var(--color-ink)]`}>
          {inline(h[2], `h${key}`)}
        </Tag>,
      );
      i++;
      continue;
    }

    // blockquote
    if (line.startsWith('>')) {
      const buf: string[] = [];
      while (i < lines.length && lines[i].startsWith('>')) {
        buf.push(lines[i].replace(/^>\s?/, ''));
        i++;
      }
      blocks.push(
        <blockquote
          key={key++}
          className="my-3 flex gap-2.5 rounded-sm border border-[var(--color-line-soft)] bg-[var(--color-canvas-sunken)] px-4 py-2.5 text-sm text-[var(--color-ink-muted)]"
        >
          <span className="mt-1.5 h-[6px] w-[6px] shrink-0 bg-[var(--color-signal)]" />
          <span>{inline(buf.join(' '), `q${key}`)}</span>
        </blockquote>,
      );
      continue;
    }

    // table (pipe rows + separator)
    if (line.includes('|') && i + 1 < lines.length && /^\s*\|?[-:\s|]+\|?\s*$/.test(lines[i + 1])) {
      const header = splitRow(line);
      i += 2; // skip separator
      const rows: string[][] = [];
      while (i < lines.length && lines[i].includes('|') && lines[i].trim()) {
        rows.push(splitRow(lines[i]));
        i++;
      }
      blocks.push(
        <div
          key={key++}
          className="my-3 overflow-x-auto rounded-lg border border-[var(--color-line-soft)]"
        >
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr className="bg-[var(--color-canvas-sunken)]">
                {header.map((c, idx) => (
                  <th
                    key={idx}
                    className="border-b border-[var(--color-line-soft)] px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-[var(--color-ink-muted)]"
                  >
                    {inline(c, `th${idx}`)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((r, ri) => (
                <tr key={ri} className="transition-colors hover:bg-[var(--color-surface-raised)]">
                  {r.map((c, ci) => (
                    <td
                      key={ci}
                      className="border-b border-[var(--color-line-soft)] px-3 py-2 text-[var(--color-ink)] last:border-0 [tr:last-child_&]:border-0"
                    >
                      {inline(c, `td${ri}-${ci}`)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>,
      );
      continue;
    }

    // unordered list
    if (/^\s*[-*]\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\s*[-*]\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^\s*[-*]\s+/, ''));
        i++;
      }
      blocks.push(
        <ul key={key++} className="my-3 space-y-1.5 pl-1">
          {items.map((it, idx) => (
            <li key={idx} className="flex gap-2.5 text-sm text-[var(--color-ink-muted)]">
              <span className="mt-2 h-1 w-1 shrink-0 rounded-full bg-[var(--color-signal)]" />
              <span>{inline(it, `li${idx}`)}</span>
            </li>
          ))}
        </ul>,
      );
      continue;
    }

    // paragraph (gather until blank)
    const buf: string[] = [];
    while (i < lines.length && lines[i].trim() && !/^(#{1,4}\s|>|```|\s*[-*]\s)/.test(lines[i])) {
      buf.push(lines[i]);
      i++;
    }
    blocks.push(
      <p key={key++} className="my-2.5 text-sm leading-relaxed text-[var(--color-ink-muted)] text-pretty">
        {inline(buf.join(' '), `p${key}`)}
      </p>,
    );
  }

  return <div className="max-w-[68ch]">{blocks.map((b, idx) => <Fragment key={idx}>{b}</Fragment>)}</div>;
}

function splitRow(row: string): string[] {
  return row
    .trim()
    .replace(/^\|/, '')
    .replace(/\|$/, '')
    .split('|')
    .map((c) => c.trim());
}
