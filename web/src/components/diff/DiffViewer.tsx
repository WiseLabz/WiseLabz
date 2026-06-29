/**
 * Unified DiffViewer. Two formats off the same component:
 *   - infra: structured key-path before/after rows (config/state drift)
 *   - doc:   two full document revisions rendered JetBrains-IDE style — whole
 *            document with context, line + word-level highlighting, foldable
 *            unchanged regions, and a unified ↔ side-by-side toggle.
 * Removed content is tinted err, added ok — but each line is also marked with a
 * −/+ gutter glyph, so the diff reads without relying on color alone.
 */
import { useMemo, useState } from 'react';
import { buildDocDiff, type DiffRowUnit, type DocRow } from '../../lib/docdiffmodel';
import type { WordSegment } from '../../lib/worddiff';
import { useUi } from '../../store/ui';
import { cn } from '../../lib/cn';
import { ColumnsIcon, RowsIcon } from '../icons';
import type { Diff } from '../../api/model';

export function DiffViewer({ diff }: { diff: Diff }) {
  if (diff.format === 'doc') {
    // Prefer the full revisions; fall back to stitching legacy hunks for back-compat.
    const before = diff.baseText ?? (diff.hunks ?? []).map((h) => h.before ?? '').join('\n');
    const after = diff.headText ?? (diff.hunks ?? []).map((h) => h.after ?? '').join('\n');
    return (
      <DocDiff
        before={before}
        after={after}
        baseLabel={diff.baseLabel}
        headLabel={diff.headLabel}
      />
    );
  }
  return <InfraDiff diff={diff} />;
}

function StatBar({ added, removed }: { added: number; removed: number }) {
  return (
    <span className="nums flex items-center gap-2 font-mono text-2xs">
      <span className="text-ok">+{added}</span>
      <span className="text-err">−{removed}</span>
    </span>
  );
}

/* ── Infra format ──────────────────────────────────────────────────────── */

export function InfraDiff({ diff }: { diff: Diff }) {
  const hunks = diff.hunks ?? [];
  const added = hunks.filter((h) => h.before === null).length;
  const removed = hunks.filter((h) => h.after === null).length;

  return (
    <div className="overflow-hidden rounded-lg border border-line-soft">
      <div className="flex items-center justify-between border-b border-line-soft bg-canvas-sunken px-3 py-2">
        <span className="font-mono text-2xs uppercase tracking-wider text-ink-faint">
          infrastructure drift
        </span>
        <StatBar added={added} removed={removed} />
      </div>
      <div className="divide-y divide-line-soft">
        {hunks.map((h, idx) => (
          <div key={idx} className="px-3 py-2.5">
            <p className="mb-1.5 font-mono text-2xs text-ink-faint">{h.path}</p>
            <div className="grid gap-1.5 sm:grid-cols-2">
              <DiffCell value={h.before} kind="before" />
              <DiffCell value={h.after} kind="after" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function DiffCell({ value, kind }: { value: string | null; kind: 'before' | 'after' }) {
  const empty = value === null;
  const isBefore = kind === 'before';
  return (
    <div
      className={cn(
        'flex items-start gap-2 rounded-md px-2.5 py-1.5 font-mono text-xs',
        empty
          ? 'bg-canvas-sunken text-ink-faint'
          : isBefore
            ? 'bg-err-tint text-err'
            : 'bg-ok-tint text-ok'
      )}
    >
      <span className="select-none font-bold opacity-70">{isBefore ? '−' : '+'}</span>
      <span className="break-all">{empty ? (isBefore ? '(absent)' : '(removed)') : value}</span>
    </div>
  );
}

/* ── Doc format (JetBrains-style revision diff) ────────────────────────── */

const WORD_HL: Record<'del' | 'add', string> = {
  del: 'color-mix(in oklch, var(--color-err) 30%, transparent)',
  add: 'color-mix(in oklch, var(--color-ok) 30%, transparent)',
};

/** Render a line's text, highlighting the words that actually changed. */
function Words({ segs, side, text }: { segs?: WordSegment[]; side: 'del' | 'add'; text: string }) {
  if (!segs) return <>{text || ' '}</>;
  return (
    <>
      {segs.map((s, i) =>
        s.changed ? (
          <span key={i} style={{ backgroundColor: WORD_HL[side], borderRadius: 2 }}>
            {s.text}
          </span>
        ) : (
          <span key={i}>{s.text}</span>
        )
      )}
    </>
  );
}

export function DocDiff({
  before,
  after,
  baseLabel,
  headLabel,
  label,
}: {
  before: string;
  after: string;
  baseLabel?: string;
  headLabel?: string;
  label?: string;
}) {
  const layout = useUi((s) => s.diffLayout);
  const setLayout = useUi((s) => s.setDiffLayout);
  const model = useMemo(() => buildDocDiff(before, after), [before, after]);

  return (
    <div className="overflow-hidden rounded-lg border border-line-soft">
      <div className="flex items-center justify-between gap-3 border-b border-line-soft bg-canvas-sunken px-3 py-2">
        <span className="truncate font-mono text-2xs uppercase tracking-wider text-ink-faint">
          {label ?? 'document diff'}
          {baseLabel && headLabel && (
            <span className="ml-2 normal-case tracking-normal text-ink-faint">
              {baseLabel} → {headLabel}
            </span>
          )}
        </span>
        <div className="flex items-center gap-3">
          <StatBar added={model.stats.added} removed={model.stats.removed} />
          <LayoutToggle layout={layout} onChange={setLayout} />
        </div>
      </div>
      {layout === 'split' ? <SplitView model={model.rows} /> : <UnifiedView model={model.rows} />}
    </div>
  );
}

function LayoutToggle({
  layout,
  onChange,
}: {
  layout: 'unified' | 'split';
  onChange: (l: 'unified' | 'split') => void;
}) {
  const opts: { value: 'unified' | 'split'; label: string; Icon: typeof RowsIcon }[] = [
    { value: 'unified', label: 'Unified', Icon: RowsIcon },
    { value: 'split', label: 'Side-by-side', Icon: ColumnsIcon },
  ];
  return (
    <div className="flex items-center gap-0.5 rounded-md border border-line-soft p-0.5">
      {opts.map(({ value, label, Icon }) => (
        <button
          key={value}
          type="button"
          aria-label={label}
          aria-pressed={layout === value}
          onClick={() => onChange(value)}
          className={cn(
            'flex h-6 w-6 items-center justify-center rounded transition-colors',
            layout === value ? 'bg-surface-raised text-ink' : 'text-ink-faint hover:text-ink'
          )}
        >
          <Icon size={13} />
        </button>
      ))}
    </div>
  );
}

/* ── Unified view ──────────────────────────────────────────────────────── */

function UnifiedView({ model }: { model: DocRow[] }) {
  const [open, setOpen] = useState<Set<number>>(new Set());
  const toggle = (i: number) => setOpen((s) => new Set(s).add(i));

  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse font-mono text-xs">
        <tbody>
          {model.map((row, idx) => {
            if (row.kind === 'gap' && !open.has(idx)) {
              return <GapRow key={idx} cols={4} count={row.count} onExpand={() => toggle(idx)} />;
            }
            const units = row.kind === 'gap' ? row.units : [row.unit];
            return units.flatMap((u, k) => unifiedRows(u, `${idx}.${k}`));
          })}
        </tbody>
      </table>
    </div>
  );
}

function unifiedRows(u: DiffRowUnit, key: string) {
  const rows = [];
  if (u.type === 'same') {
    rows.push(
      <UnifiedLine
        key={key}
        tone="same"
        before={u.left?.before}
        after={u.right?.after}
        glyph=" "
        text={u.left?.text ?? ''}
      />
    );
  } else if (u.type === 'mod') {
    rows.push(
      <UnifiedLine
        key={`${key}d`}
        tone="del"
        before={u.left?.before}
        glyph="−"
        segs={u.leftWords}
        side="del"
        text={u.left?.text ?? ''}
      />,
      <UnifiedLine
        key={`${key}a`}
        tone="add"
        after={u.right?.after}
        glyph="+"
        segs={u.rightWords}
        side="add"
        text={u.right?.text ?? ''}
      />
    );
  } else if (u.type === 'del') {
    rows.push(
      <UnifiedLine
        key={key}
        tone="del"
        before={u.left?.before}
        glyph="−"
        text={u.left?.text ?? ''}
      />
    );
  } else {
    rows.push(
      <UnifiedLine
        key={key}
        tone="add"
        after={u.right?.after}
        glyph="+"
        text={u.right?.text ?? ''}
      />
    );
  }
  return rows;
}

function UnifiedLine({
  tone,
  before,
  after,
  glyph,
  text,
  segs,
  side,
}: {
  tone: 'same' | 'add' | 'del';
  before?: number;
  after?: number;
  glyph: string;
  text: string;
  segs?: WordSegment[];
  side?: 'del' | 'add';
}) {
  const bg =
    tone === 'add'
      ? 'bg-[var(--color-ok-tint)]'
      : tone === 'del'
        ? 'bg-[var(--color-err-tint)]'
        : '';
  const fg =
    tone === 'add'
      ? 'text-[var(--color-ok)]'
      : tone === 'del'
        ? 'text-[var(--color-err)]'
        : 'text-[var(--color-ink-muted)]';
  return (
    <tr className={bg}>
      <Gutter>{before ?? ''}</Gutter>
      <Gutter>{after ?? ''}</Gutter>
      <td className={cn('w-5 select-none px-2 py-0.5 text-center font-bold opacity-70', fg)}>
        {glyph}
      </td>
      <td className={cn('whitespace-pre-wrap px-2 py-0.5', fg)}>
        {side ? <Words segs={segs} side={side} text={text} /> : text || ' '}
      </td>
    </tr>
  );
}

/* ── Side-by-side view ─────────────────────────────────────────────────── */

function SplitView({ model }: { model: DocRow[] }) {
  const [open, setOpen] = useState<Set<number>>(new Set());
  const toggle = (i: number) => setOpen((s) => new Set(s).add(i));

  return (
    <div className="overflow-x-auto">
      <table className="w-full table-fixed border-collapse font-mono text-xs">
        <tbody>
          {model.map((row, idx) => {
            if (row.kind === 'gap' && !open.has(idx)) {
              return <GapRow key={idx} cols={6} count={row.count} onExpand={() => toggle(idx)} />;
            }
            const units = row.kind === 'gap' ? row.units : [row.unit];
            return units.map((u, k) => <SplitRow key={`${idx}.${k}`} u={u} />);
          })}
        </tbody>
      </table>
    </div>
  );
}

function SplitRow({ u }: { u: DiffRowUnit }) {
  const leftTone = u.type === 'same' ? 'same' : u.left ? 'del' : 'empty';
  const rightTone = u.type === 'same' ? 'same' : u.right ? 'add' : 'empty';
  return (
    <tr className="align-top">
      <SplitCell
        tone={leftTone}
        num={u.left?.before}
        glyph={u.left ? '−' : ''}
        text={u.left?.text}
        segs={u.leftWords}
        side="del"
      />
      <SplitCell
        tone={rightTone}
        num={u.right?.after}
        glyph={u.right ? '+' : ''}
        text={u.right?.text}
        segs={u.rightWords}
        side="add"
        border
      />
    </tr>
  );
}

function SplitCell({
  tone,
  num,
  glyph,
  text,
  segs,
  side,
  border,
}: {
  tone: 'same' | 'del' | 'add' | 'empty';
  num?: number;
  glyph: string;
  text?: string;
  segs?: WordSegment[];
  side: 'del' | 'add';
  border?: boolean;
}) {
  const bg =
    tone === 'add'
      ? 'bg-[var(--color-ok-tint)]'
      : tone === 'del'
        ? 'bg-[var(--color-err-tint)]'
        : tone === 'empty'
          ? 'bg-[var(--color-canvas-sunken)]'
          : '';
  const fg =
    tone === 'add'
      ? 'text-[var(--color-ok)]'
      : tone === 'del'
        ? 'text-[var(--color-err)]'
        : 'text-[var(--color-ink-muted)]';
  return (
    <>
      <Gutter className={border ? 'border-l border-line' : undefined}>{num ?? ''}</Gutter>
      <td className={cn('w-5 select-none text-center font-bold opacity-70', bg, fg)}>{glyph}</td>
      <td className={cn('whitespace-pre-wrap px-2 py-0.5', bg, fg)} style={{ width: '46%' }}>
        {tone === 'empty' ? (
          ''
        ) : segs ? (
          <Words segs={segs} side={side} text={text ?? ''} />
        ) : (
          text || ' '
        )}
      </td>
    </>
  );
}

/* ── Shared bits ───────────────────────────────────────────────────────── */

function Gutter({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <td
      className={cn(
        'nums w-10 select-none border-r border-line-soft px-2 py-0.5 text-right text-ink-faint',
        className
      )}
    >
      {children}
    </td>
  );
}

function GapRow({ cols, count, onExpand }: { cols: number; count: number; onExpand: () => void }) {
  return (
    <tr className="bg-canvas-sunken">
      <td colSpan={cols} className="px-2 py-0.5">
        <button
          type="button"
          onClick={onExpand}
          className="flex w-full items-center gap-2 py-0.5 font-mono text-2xs text-ink-faint transition-colors hover:text-signal-bright"
        >
          <span className="select-none">⋯</span>
          {count} unchanged {count === 1 ? 'line' : 'lines'} — expand
        </button>
      </td>
    </tr>
  );
}
