/**
 * Hierarchical doc navigation — lab root + per-service children. Current doc gets
 * the iris selection treatment; the lab root is always expanded (the tree is
 * shallow by design). Service nodes show their kind as a quiet mono tag.
 */
import { NavLink } from 'react-router-dom';
import { cn } from '../../lib/cn';
import { FileTextIcon, LayersIcon } from '../icons';
import type { DocNode } from '../../api/model';

export function DocTree({ tree }: { tree: DocNode }) {
  return (
    <nav className="flex flex-col gap-0.5">
      <NavLink
        to={`/docs/${tree.docId}`}
        end
        className={({ isActive }) =>
          cn(
            'flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm font-medium transition-colors',
            isActive
              ? 'bg-[var(--color-signal-tint)] text-[var(--color-ink)]'
              : 'text-[var(--color-ink-muted)] hover:bg-[var(--color-surface-raised)] hover:text-[var(--color-ink)]',
          )
        }
      >
        <LayersIcon size={16} className="text-[var(--color-signal-bright)]" />
        {tree.title}
      </NavLink>

      <div className="ml-3 mt-0.5 flex flex-col gap-0.5 border-l border-[var(--color-line-soft)] pl-2">
        {(tree.children ?? []).map((node) => (
          <NavLink
            key={node.docId}
            to={`/docs/${node.docId}`}
            className={({ isActive }) =>
              cn(
                'group flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm transition-colors',
                isActive
                  ? 'bg-[var(--color-signal-tint)] text-[var(--color-ink)]'
                  : 'text-[var(--color-ink-muted)] hover:bg-[var(--color-surface-raised)] hover:text-[var(--color-ink)]',
              )
            }
          >
            <FileTextIcon size={15} className="shrink-0 opacity-70" />
            <span className="truncate">{node.title.split(' — ')[0]}</span>
            {node.title.includes(' — ') && (
              <span className="ml-auto truncate font-mono text-2xs text-[var(--color-ink-faint)]">
                {node.title.split(' — ')[1]}
              </span>
            )}
          </NavLink>
        ))}
      </div>
    </nav>
  );
}
