/**
 * ⌘K command palette. Keyboard-first: ⌘K/Ctrl-K toggles, ↑/↓ move, ↵ runs, Esc
 * closes. Rendered with position:fixed at modal z so it never gets clipped by a
 * scroll/overflow ancestor. Commands cover navigation, global sync, and jumping
 * straight to a service or doc.
 *
 * The body is a separate component mounted only while open, so its query/cursor
 * state initializes fresh on every open — no reset effects needed.
 */
import { useEffect, useId, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { AnimatePresence, motion } from 'motion/react';
import { matchSorter } from 'match-sorter';
import { flattenDocTree } from '../../lib/docTree';
import { useUi } from '../../store/ui';
import { useCanMutate } from '../../hooks/useRole';
import { runSync } from '../../lib/runSync';
import { useTheme } from '../../store/theme';
import { PRESETS, type PaletteName } from '../../theme';
import {
  useGetConnectors,
  putConnectorsConnectorIdEnabled,
  getGetConnectorsQueryKey,
} from '../../api/generated/connectors/connectors';
import { useGetSearch } from '../../api/generated/search/search';
import { useGetDocsTree } from '../../api/generated/docs/docs';
import type { Connector, DocNode } from '../../api/model';
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
import { toast } from '../../lib/toast';
import { registeredCommands, type Command, type CommandCtx, type CommandGroup } from './registry';

const GROUP_ORDER: CommandGroup[] = [
  'navigate',
  'actions',
  'services',
  'docs',
  'runbooks',
  'entities',
];

/** Advance the theme palette to the next preset — a quick keyboard-only re-skin. */
function cycleTheme() {
  const names = Object.keys(PRESETS) as PaletteName[];
  const current = useTheme.getState().preset;
  const next = names[(names.indexOf(current) + 1) % names.length];
  useTheme.getState().setPreset(next);
}

function buildCommands(ctx: CommandCtx, connectors: Connector[], docNodes: DocNode[]): Command[] {
  const { t, canMutate } = ctx;

  const nav: Command[] = [
    {
      id: 'n-dash',
      label: t('command.nav.dashboard'),
      group: 'navigate',
      Icon: GaugeIcon,
      run: (c) => c.navigate('/dashboard'),
    },
    {
      id: 'n-svc',
      label: t('command.nav.services'),
      group: 'navigate',
      Icon: LayersIcon,
      run: (c) => c.navigate('/services'),
    },
    {
      id: 'n-docs',
      label: t('command.nav.docs'),
      group: 'navigate',
      Icon: FileTextIcon,
      run: (c) => c.navigate('/docs'),
    },
    {
      id: 'n-journal',
      label: t('command.nav.journal'),
      group: 'navigate',
      Icon: FileTextIcon,
      run: (c) => c.navigate('/journal'),
    },
    {
      id: 'n-chg',
      label: t('command.nav.changes'),
      group: 'navigate',
      Icon: DiffIcon,
      run: (c) => c.navigate('/changes'),
    },
    {
      id: 'n-alerts',
      label: t('command.nav.alerts'),
      group: 'navigate',
      Icon: BellIcon,
      run: (c) => c.navigate('/alerts'),
    },
    {
      id: 'n-set',
      label: t('command.nav.settings'),
      group: 'navigate',
      Icon: SettingsIcon,
      run: (c) => c.navigate('/settings'),
    },
  ];

  const actions: Command[] = [
    {
      id: 'a-theme',
      label: t('command.action.cycleTheme'),
      hint: t('command.action.cycleThemeHint'),
      group: 'actions',
      Icon: SettingsIcon,
      run: cycleTheme,
    },
  ];
  // Mutating actions only surface for operators — the server still enforces it.
  if (canMutate) {
    actions.push({
      id: 'a-new-doc',
      label: t('docs.human.new', { defaultValue: 'New doc' }),
      group: 'actions',
      Icon: FileTextIcon,
      run: (c) => c.navigate('/docs?new=1'),
    });
    actions.unshift({
      id: 'a-sync',
      label: t('command.action.syncAll'),
      hint: t('command.action.syncAllHint'),
      group: 'actions',
      Icon: SyncIcon,
      run: () => runSync(null),
    });
  }

  const services: Command[] = connectors.flatMap((cn) => {
    const items: Command[] = [
      {
        id: `s-${cn.id}`,
        label: cn.name,
        hint: cn.type,
        group: 'services',
        Icon: categoryIcon[cn.category],
        run: (c) => c.navigate(`/services/${cn.id}`),
      },
    ];
    if (canMutate) {
      items.push(
        {
          id: `s-sync-${cn.id}`,
          label: t('command.action.syncOne', { name: cn.name }),
          group: 'actions',
          Icon: SyncIcon,
          run: () => runSync(cn.id),
        },
        {
          id: `s-toggle-${cn.id}`,
          label: t(cn.enabled ? 'command.action.disableOne' : 'command.action.enableOne', {
            name: cn.name,
          }),
          group: 'actions',
          Icon: LayersIcon,
          run: async (c) => {
            await putConnectorsConnectorIdEnabled(cn.id, { enabled: !cn.enabled });
            void c.queryClient.invalidateQueries({ queryKey: getGetConnectorsQueryKey() });
          },
        }
      );
    }
    return items;
  });

  const docs: Command[] = docNodes.map((d) => ({
    id: `d-${d.docId}`,
    label: d.title,
    group: 'docs',
    Icon: FileTextIcon,
    run: (c) => c.navigate(`/docs/${d.docId}`),
  }));

  return [...nav, ...actions, ...services, ...docs, ...registeredCommands(ctx)];
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

  // `?` toggles the shortcuts cheat sheet — same text-field guard as ⌘K.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== '?') return;
      const el = document.activeElement as HTMLElement | null;
      const isTyping =
        el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable);
      if (isTyping) return;
      e.preventDefault();
      useUi.getState().toggleShortcuts();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  return <AnimatePresence>{open && <PaletteBody />}</AnimatePresence>;
}

function PaletteBody() {
  const setOpen = useUi((s) => s.setPalette);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const canMutate = useCanMutate();
  const { t } = useTranslation();
  const connectorsQuery = useGetConnectors();
  const docsTreeQuery = useGetDocsTree();
  const ctx = useMemo<CommandCtx>(
    () => ({ navigate, t, canMutate, queryClient }),
    [navigate, t, canMutate, queryClient]
  );
  const commands = useMemo(
    () =>
      buildCommands(
        ctx,
        connectorsQuery.data ?? [],
        flattenDocTree(docsTreeQuery.data).filter((n) => !n.branch && n.docId !== 'root')
      ),
    [ctx, connectorsQuery.data, docsTreeQuery.data]
  );
  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query.trim()), 300);
    return () => clearTimeout(timer);
  }, [query]);
  const searchQuery = useGetSearch(
    { q: debouncedQuery, limit: 5 },
    {
      query: { enabled: debouncedQuery.length >= 2 },
    }
  );
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const listboxId = useId();
  const optionId = (id: string) => `${listboxId}-${id}`;

  // Focus the input on open and restore focus to the opener on close.
  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null;
    const id = requestAnimationFrame(() => inputRef.current?.focus());
    return () => {
      cancelAnimationFrame(id);
      opener?.focus?.();
    };
  }, []);

  // Escape closes the palette wherever focus is.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [setOpen]);

  const filtered = useMemo(() => {
    const q = query.trim();
    if (!q) return commands;
    let items = matchSorter(commands, q, { keys: ['label', 'hint'] });
    if (q.length >= 2) {
      // Only show hits for the current query while the debounce catches up.
      const results = q === debouncedQuery ? searchQuery.data : undefined;
      const serverDocs: Command[] = (results?.docs ?? []).map((hit) => ({
        id: `d-${hit.id}`,
        label: hit.title,
        hint: hit.snippet,
        group: 'docs',
        Icon: FileTextIcon,
        run: (c) => c.navigate(`/docs/${encodeURIComponent(hit.id)}`),
      }));
      const docIds = new Set(serverDocs.map((c) => c.id));
      items = items.filter((c) => !docIds.has(c.id));
      items.push(...serverDocs);
      items.push(
        ...(results?.runbooks ?? []).map((hit): Command => ({
          id: `r-${hit.id}`,
          label: hit.title,
          hint: hit.snippet,
          group: 'runbooks',
          Icon: FileTextIcon,
          run: (c) =>
            c.navigate(`/search?${new URLSearchParams({ q, type: 'runbook', runbook: hit.id })}`),
        }))
      );
      items.push(
        ...(results?.entities ?? []).map((hit, index): Command => ({
          id: `e-${hit.connectorId}-${index}`,
          label: hit.name,
          hint: [hit.connectorName, hit.kind, hit.ip || hit.hostname || hit.mac || hit.externalId]
            .filter(Boolean)
            .join(' · '),
          group: 'entities',
          Icon: LayersIcon,
          run: (c) =>
            c.navigate(
              hit.entityId
                ? `/entities/${encodeURIComponent(hit.entityId)}`
                : hit.docId
                  ? `/docs/${encodeURIComponent(hit.docId)}`
                  : `/services/${encodeURIComponent(hit.connectorId)}`
            ),
        }))
      );
      items.push({
        id: 'search-all',
        label: t('search.seeAll'),
        group: 'entities',
        Icon: FileTextIcon,
        run: (c) => c.navigate(`/search?${new URLSearchParams({ q })}`),
      });
    }
    // Keep keyboard traversal in the same order as the displayed groups.
    return [
      ...GROUP_ORDER.flatMap((group) =>
        items.filter((c) => c.group === group && c.id !== 'search-all')
      ),
      ...items.filter((c) => c.id === 'search-all'),
    ];
  }, [commands, query, debouncedQuery, searchQuery.data, t]);

  // Clamp the cursor at render time instead of in an effect.
  const cursor = filtered.length ? Math.min(active, filtered.length - 1) : 0;

  const groups = useMemo(() => {
    const map = new Map<CommandGroup, Command[]>();
    for (const c of filtered) {
      if (c.id === 'search-all') continue;
      const arr = map.get(c.group) ?? [];
      arr.push(c);
      map.set(c.group, arr);
    }
    return GROUP_ORDER.filter((g) => map.has(g)).map((g) => [g, map.get(g)!] as const);
  }, [filtered]);

  const run = (c: Command) => {
    setOpen(false);
    Promise.resolve()
      .then(() => c.run(ctx))
      .catch(() => toast.error(t('common.actionFailed')));
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (filtered.length) setActive((cursor + 1) % filtered.length);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (filtered.length) setActive((cursor - 1 + filtered.length) % filtered.length);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const c = filtered[cursor];
      if (c) run(c);
    } else if (e.key === 'Tab') {
      // Focus trap: the input is the only tab stop (options are navigated with arrows).
      e.preventDefault();
      inputRef.current?.focus();
    }
  };

  // keep active item visible (DOM only, no state)
  useEffect(() => {
    listRef.current?.querySelector('[data-active="true"]')?.scrollIntoView({ block: 'nearest' });
  }, [cursor]);

  const renderOption = (c: Command) => {
    // Index into the flat `filtered` list — the same ordering the
    // cursor/Enter handlers use — so the highlight always matches.
    const idx = filtered.indexOf(c);
    const isActive = idx === cursor;
    return (
      <button
        key={c.id}
        id={optionId(c.id)}
        role="option"
        aria-selected={isActive}
        tabIndex={-1}
        data-active={isActive}
        onMouseMove={() => setActive(idx)}
        onClick={() => run(c)}
        className="flex w-full items-center gap-3 rounded-sm px-2.5 py-2 text-left font-mono text-sm transition-colors"
        style={{
          backgroundColor: isActive ? 'var(--color-accent-primary-tint)' : 'transparent',
          color: isActive ? 'var(--color-ink)' : 'var(--color-ink-muted)',
        }}
      >
        <c.Icon size={16} className="shrink-0 opacity-80" />
        <span className="flex-1">{c.label}</span>
        {c.hint && <span className="font-mono text-2xs text-ink-faint">{c.hint}</span>}
      </button>
    );
  };

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
        aria-modal="true"
        aria-label={t('command.title')}
        className="reg-ticks relative w-full max-w-xl overflow-hidden rounded-sm border border-line-strong bg-surface-overlay shadow-(--shadow-pop)"
        initial={{ opacity: 0, y: -12, scale: 0.97 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        exit={{ opacity: 0, y: -8, scale: 0.98 }}
        transition={{ type: 'spring', stiffness: 460, damping: 34 }}
        onMouseDown={(e) => e.stopPropagation()}
        onKeyDown={onKeyDown}
      >
        <div className="flex items-center gap-3 border-b border-line-soft px-4">
          <svg
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
            className="text-ink-faint"
            aria-hidden="true"
          >
            <circle cx="11" cy="11" r="7" />
            <path d="m21 21-4.3-4.3" />
          </svg>
          <input
            ref={inputRef}
            role="combobox"
            aria-expanded="true"
            aria-controls={listboxId}
            aria-autocomplete="list"
            aria-label={t('command.title')}
            aria-activedescendant={filtered[cursor] ? optionId(filtered[cursor].id) : undefined}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
            }}
            placeholder={t('command.placeholder')}
            className="h-12 flex-1 bg-transparent font-mono text-sm text-ink outline-none placeholder:text-ink-faint"
          />
          <kbd className="rounded border border-line-strong px-1.5 py-0.5 font-mono text-2xs text-ink-faint">
            Esc
          </kbd>
        </div>

        <div
          ref={listRef}
          id={listboxId}
          role="listbox"
          aria-label={t('command.results')}
          className="max-h-[52vh] overflow-y-auto p-2"
        >
          {filtered.every((c) => c.id === 'search-all') && (
            <p className="px-3 py-8 text-center text-sm text-ink-faint">
              {t('command.noMatches', { query })}
            </p>
          )}
          {groups.map(([group, items]) => (
            <div key={group} role="group" aria-label={t(`command.group.${group}`)} className="mb-1">
              <p aria-hidden="true" className="px-2.5 py-1.5 text-2xs font-semibold text-ink-faint">
                {t(`command.group.${group}`)}
              </p>
              {items.map(renderOption)}
            </div>
          ))}
          {filtered.filter((c) => c.id === 'search-all').map(renderOption)}
        </div>
      </motion.div>
    </motion.div>
  );
}
