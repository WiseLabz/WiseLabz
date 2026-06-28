/**
 * Floating bottom dock (soft-dark). Centered, elevated nav inspired by the
 * VinylVault dock but solid rather than glass — one amber pill slides under the
 * active item (motion layoutId). Primary sections + Settings live here; the slim
 * Topbar carries brand/search/sync/account.
 */
import { NavLink } from 'react-router-dom';
import { motion } from 'motion/react';
import { useTranslation } from 'react-i18next';
import { cn } from '../../lib/cn';
import { useLive } from '../../store/live';
import { NAV, type NavItem } from './nav';
import { SettingsIcon } from '../icons';

const DOCK_ITEMS: NavItem[] = [...NAV, { to: '/settings', label: 'Settings', Icon: SettingsIcon }];

export function Dock() {
  const pending = useLive((s) => s.pendingAlerts);
  const { t } = useTranslation();

  return (
    <div className="pointer-events-none fixed inset-x-0 bottom-5 z-[var(--z-sticky)] flex justify-center px-4">
      <nav className="pointer-events-auto flex items-center gap-0.5 rounded-[var(--radius-xl)] border border-[var(--color-line)] bg-[color-mix(in_oklch,var(--color-surface-raised)_88%,transparent)] p-1.5 shadow-[var(--shadow-pop)] backdrop-blur-xl">
        {DOCK_ITEMS.map(({ to, Icon, badge }) => (
          <NavLink
            key={to}
            to={to}
            title={t(`nav.${to.replace('/', '')}`)}
            className={({ isActive }) =>
              cn(
                'group relative flex items-center gap-2 rounded-[var(--radius-md)] px-3 py-2 text-sm transition-colors duration-150',
                isActive
                  ? 'text-[var(--color-signal-ink)]'
                  : 'text-[var(--color-ink-muted)] hover:bg-[var(--color-surface)] hover:text-[var(--color-ink)]',
              )
            }
          >
            {({ isActive }) => (
              <>
                {isActive && (
                  <motion.span
                    layoutId="dock-pill"
                    className="absolute inset-0 -z-10 rounded-[var(--radius-md)] bg-[var(--color-signal)] shadow-[var(--shadow-raised)]"
                    transition={{ type: 'spring', stiffness: 500, damping: 38 }}
                  />
                )}
                <span className="relative">
                  <Icon size={18} className="shrink-0" />
                  {badge && pending > 0 && (
                    <span className="nums absolute -right-1.5 -top-1.5 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-[var(--color-err)] px-1 text-[10px] font-bold text-[var(--color-canvas)]">
                      {pending}
                    </span>
                  )}
                </span>
                <span
                  className={cn(
                    'overflow-hidden whitespace-nowrap font-medium transition-all duration-200',
                    isActive ? 'max-w-[120px] opacity-100' : 'max-w-0 opacity-0 group-hover:max-w-[120px] group-hover:opacity-100',
                  )}
                >
                  {t(`nav.${to.replace('/', '')}`)}
                </span>
              </>
            )}
          </NavLink>
        ))}
      </nav>
    </div>
  );
}
