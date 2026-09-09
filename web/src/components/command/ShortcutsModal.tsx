/**
 * `?`-triggered keyboard-shortcuts cheat sheet. Static "Palette" section plus a
 * dynamic list of every registered command that declares a `hotkey`, grouped by
 * the same GROUP_ORDER the command palette uses.
 */
import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { Dialog } from '../ui/Dialog';
import { useUi } from '../../store/ui';
import { useCanMutate } from '../../hooks/useRole';
import { registeredCommands, type CommandCtx, type CommandGroup } from './registry';

const GROUP_ORDER: CommandGroup[] = ['navigate', 'actions', 'services', 'docs'];

const PALETTE_SHORTCUTS: Array<{ keys: string; label: string }> = [
  { keys: '⌘K', label: 'Open command palette' },
  { keys: '↑ / ↓', label: 'Navigate results' },
  { keys: '↵', label: 'Run selected command' },
  { keys: 'Esc', label: 'Close' },
];

export function ShortcutsModal() {
  const open = useUi((s) => s.shortcutsOpen);
  const setOpen = useUi((s) => s.setShortcuts);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const canMutate = useCanMutate();
  const { t } = useTranslation();

  const ctx = useMemo<CommandCtx>(
    () => ({ navigate, t, canMutate, queryClient }),
    [navigate, t, canMutate, queryClient]
  );

  const groups = useMemo(() => {
    const commands = registeredCommands(ctx).filter((c) => c.hotkey);
    const map = new Map<CommandGroup, typeof commands>();
    for (const c of commands) {
      const arr = map.get(c.group) ?? [];
      arr.push(c);
      map.set(c.group, arr);
    }
    return GROUP_ORDER.filter((g) => map.has(g)).map((g) => [g, map.get(g)!] as const);
  }, [ctx]);

  return (
    <Dialog open={open} onClose={() => setOpen(false)} title="Keyboard shortcuts">
      <div className="space-y-4">
        <section>
          <p className="mb-1.5 text-2xs font-semibold text-ink-faint">Palette</p>
          <ul className="space-y-1">
            {PALETTE_SHORTCUTS.map((s) => (
              <li key={s.keys} className="flex items-center justify-between gap-3 font-mono text-sm">
                <span className="text-ink-muted">{s.label}</span>
                <kbd className="rounded border border-line-strong px-1.5 py-0.5 text-2xs text-ink-faint">
                  {s.keys}
                </kbd>
              </li>
            ))}
          </ul>
        </section>

        {groups.map(([group, items]) => (
          <section key={group}>
            <p className="mb-1.5 text-2xs font-semibold text-ink-faint">{t(`command.group.${group}`)}</p>
            <ul className="space-y-1">
              {items.map((c) => (
                <li key={c.id} className="flex items-center justify-between gap-3 font-mono text-sm">
                  <span className="text-ink-muted">{c.label}</span>
                  {c.hotkey && (
                    <kbd className="rounded border border-line-strong px-1.5 py-0.5 text-2xs text-ink-faint">
                      {c.hotkey}
                    </kbd>
                  )}
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </Dialog>
  );
}
