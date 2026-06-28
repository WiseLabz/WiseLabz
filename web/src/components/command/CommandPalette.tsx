/**
 * ⌘K command palette. Keyboard-first: ⌘K/Ctrl-K toggles, ↑/↓ move, ↵ runs, Esc
 * closes. Rendered with position:fixed at modal z so it never gets clipped by a
 * scroll/overflow ancestor. Commands cover navigation, global sync, and jumping
 * straight to a service or doc.
 *
 * The body is a separate component mounted only while open, so its query/cursor
 * state initializes fresh on every open — no reset effects needed.
 */
import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { AnimatePresence, motion } from 'motion/react';
import { useUi } from '../../store/ui';
import { triggerMockSync } from '../../ws/triggerSync';
import { connectors, docTree } from '../../data/fixtures';
import { categoryIcon } from '../categoryIcon';
import {
  GaugeIcon,
  LayersIcon,
  FileTextIcon,
  DiffIcon,
  BellIcon,
  SettingsIcon,
  SyncIcon,
} from '../icons';

interface Command {
  id: string;
  label: string;
  hint?: string;
  group: 'Navigate' | 'Actions' | 'Services' | 'Docs';
  Icon: React.ComponentType<{ size?: number; className?: string }>;
  run: (nav: ReturnType<typeof useNavigate>) => void;
}

function buildCommands(): Command[] {
  const nav: Command[] = [
    { id: 'n-dash', label: 'Dashboard', group: 'Navigate', Icon: GaugeIcon, run: (n) => n('/dashboard') },
    { id: 'n-svc', label: 'Services', group: 'Navigate', Icon: LayersIcon, run: (n) => n('/services') },
    { id: 'n-docs', label: 'Docs', group: 'Navigate', Icon: FileTextIcon, run: (n) => n('/docs') },
    { id: 'n-chg', label: 'Changes', group: 'Navigate', Icon: DiffIcon, run: (n) => n('/changes') },
    { id: 'n-alerts', label: 'Alerts', group: 'Navigate', Icon: BellIcon, run: (n) => n('/alerts') },
    { id: 'n-set', label: 'Settings', group: 'Navigate', Icon: SettingsIcon, run: (n) => n('/settings') },
  ];
  const actions: Command[] = [
    {
      id: 'a-sync',
      label: 'Sync all services',
      hint: 'Run a full fleet sync',
      group: 'Actions',
      Icon: SyncIcon,
      run: () => triggerMockSync(null),
    },
  ];
  const services: Command[] = connectors.map((c) => ({
    id: `s-${c.id}`,
    label: c.name,
    hint: c.type,
    group: 'Services',
    Icon: categoryIcon[c.category],
    run: (n) => n('/services'),
  }));
  const docs: Command[] = (docTree.children ?? []).map((d) => ({
    id: `d-${d.docId}`,
    label: d.title,
    group: 'Docs',
    Icon: FileTextIcon,
    run: (n) => n(`/docs/${d.docId}`),
  }));
  return [...nav, ...actions, ...services, ...docs];
}

export function CommandPalette() {
  const open = useUi((s) => s.paletteOpen);

  // Global ⌘K / Ctrl-K toggle lives on the always-mounted shell.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        useUi.getState().togglePalette();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  return <AnimatePresence>{open && <PaletteBody />}</AnimatePresence>;
}

function PaletteBody() {
  const setOpen = useUi((s) => s.setPalette);
  const navigate = useNavigate();
  const commands = useMemo(() => buildCommands(), []);
  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const id = requestAnimationFrame(() => inputRef.current?.focus());
    return () => cancelAnimationFrame(id);
  }, []);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return commands;
    return commands.filter(
      (c) => c.label.toLowerCase().includes(q) || c.hint?.toLowerCase().includes(q),
    );
  }, [commands, query]);

  // Clamp the cursor at render time instead of in an effect.
  const cursor = filtered.length ? Math.min(active, filtered.length - 1) : 0;

  const groups = useMemo(() => {
    const map = new Map<string, Command[]>();
    for (const c of filtered) {
      const arr = map.get(c.group) ?? [];
      arr.push(c);
      map.set(c.group, arr);
    }
    return [...map.entries()];
  }, [filtered]);

  const run = (c: Command) => {
    setOpen(false);
    c.run(navigate);
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActive((cursor + 1) % filtered.length);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive((cursor - 1 + filtered.length) % filtered.length);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const c = filtered[cursor];
      if (c) run(c);
    } else if (e.key === 'Escape') {
      setOpen(false);
    }
  };

  // keep active item visible (DOM only, no state)
  useEffect(() => {
    listRef.current?.querySelector('[data-active="true"]')?.scrollIntoView({ block: 'nearest' });
  }, [cursor]);

  let flatIndex = -1;

  return (
    <motion.div
      className="fixed inset-0 flex items-start justify-center px-4 pt-[12vh]"
      style={{ zIndex: 'var(--z-modal)' }}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.15 }}
      onMouseDown={() => setOpen(false)}
    >
      <div className="absolute inset-0 bg-black/55 backdrop-blur-sm" />
      <motion.div
        role="dialog"
        aria-label="Command palette"
        className="reg-ticks relative w-full max-w-xl overflow-hidden rounded-sm border border-line-strong bg-surface-overlay shadow-(--shadow-pop)"
        initial={{ opacity: 0, y: -12, scale: 0.97 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        exit={{ opacity: 0, y: -8, scale: 0.98 }}
        transition={{ type: 'spring', stiffness: 460, damping: 34 }}
        onMouseDown={(e) => e.stopPropagation()}
        onKeyDown={onKeyDown}
      >
        <div className="flex items-center gap-3 border-b border-line-soft px-4">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="text-ink-faint" aria-hidden="true">
            <circle cx="11" cy="11" r="7" />
            <path d="m21 21-4.3-4.3" />
          </svg>
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
            }}
            placeholder="type a command or search…"
            className="h-12 flex-1 bg-transparent font-mono text-sm text-ink outline-none placeholder:text-ink-faint"
          />
          <kbd className="rounded border border-line-strong px-1.5 py-0.5 font-mono text-2xs text-ink-faint">
            Esc
          </kbd>
        </div>

        <div ref={listRef} className="max-h-[52vh] overflow-y-auto p-2">
          {filtered.length === 0 && (
            <p className="px-3 py-8 text-center text-sm text-ink-faint">
              No matches for “{query}”
            </p>
          )}
          {groups.map(([group, items]) => (
            <div key={group} className="mb-1">
              <p className="px-2.5 py-1.5 text-2xs font-semibold uppercase tracking-wider text-ink-faint">
                {group}
              </p>
              {items.map((c) => {
                flatIndex += 1;
                const isActive = flatIndex === cursor;
                const idx = flatIndex;
                return (
                  <button
                    key={c.id}
                    data-active={isActive}
                    onMouseMove={() => setActive(idx)}
                    onClick={() => run(c)}
                    className="flex w-full items-center gap-3 rounded-sm px-2.5 py-2 text-left font-mono text-sm transition-colors"
                    style={{
                      backgroundColor: isActive ? 'var(--color-signal-tint)' : 'transparent',
                      color: isActive ? 'var(--color-ink)' : 'var(--color-ink-muted)',
                    }}
                  >
                    <c.Icon size={16} className="shrink-0 opacity-80" />
                    <span className="flex-1">{c.label}</span>
                    {c.hint && (
                      <span className="font-mono text-2xs text-ink-faint">{c.hint}</span>
                    )}
                  </button>
                );
              })}
            </div>
          ))}
        </div>
      </motion.div>
    </motion.div>
  );
}
