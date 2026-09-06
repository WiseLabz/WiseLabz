/**
 * Global top bar: collapse toggle, a ⌘K search affordance, the global-sync
 * button (shows live phase while a fleet sync runs), the alert bell with live
 * count, and the user menu. The sync button is the entry point to the dashboard's
 * signature sweep.
 */
import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { AnimatePresence, motion } from 'motion/react';
import { useUi } from '../../store/ui';
import { useLive } from '../../store/live';
import { useAuth } from '../../store/auth';
import { useCanMutate } from '../../hooks/useRole';
import { runSync } from '../../lib/runSync';
import { Button } from '../ui/Button';
import { SearchIcon, SyncIcon, UserIcon, ChevronDownIcon } from '../icons';
import { NotificationCenter } from './NotificationCenter';

export function Topbar() {
  const togglePalette = useUi((s) => s.togglePalette);
  const globalJob = useLive((s) => s.jobs.global);
  const canMutate = useCanMutate();
  const user = useAuth((s) => s.user);
  const logout = useAuth((s) => s.logout);
  const [menuOpen, setMenuOpen] = useState(false);
  const menuButtonRef = useRef<HTMLButtonElement>(null);
  const firstMenuItemRef = useRef<HTMLButtonElement>(null);
  const navigate = useNavigate();
  const { t } = useTranslation();

  useEffect(() => {
    if (!menuOpen) return;
    firstMenuItemRef.current?.focus();
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setMenuOpen(false);
        menuButtonRef.current?.focus();
      }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [menuOpen]);

  const syncing = globalJob && globalJob.phase !== 'done' && globalJob.phase !== 'error';

  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-b border-line-soft bg-canvas/80 px-4 backdrop-blur-md">
      <div className="flex items-center gap-2.5 pr-1">
        <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-signal shadow-(--shadow-raised)">
          <span className="font-mono text-sm font-bold text-signal-ink">W</span>
        </div>
        <span className="hidden text-sm font-semibold tracking-tight text-ink sm:block">
          WiseLabz
        </span>
      </div>

      {/* Search / command palette trigger */}
      <button
        onClick={togglePalette}
        aria-label={t('topbar.searchPlaceholder')}
        className="group flex h-9 w-9 shrink-0 items-center justify-center rounded-sm border border-line bg-canvas-sunken font-mono text-xs text-ink-faint transition-colors hover:border-line-strong hover:text-ink-muted sm:w-full sm:max-w-sm sm:justify-start sm:gap-2.5 sm:px-3"
      >
        <SearchIcon size={15} />
        <span className="hidden flex-1 text-left sm:block">{t('topbar.searchPlaceholder')}</span>
        <kbd className="nums hidden rounded border border-line-strong bg-canvas-sunken px-1.5 py-0.5 font-mono text-2xs text-ink-faint sm:block">
          ⌘K
        </kbd>
      </button>

      <div className="ml-auto flex items-center gap-2">
        {canMutate && (
          <Button
            variant={syncing ? 'secondary' : 'primary'}
            size="sm"
            onClick={() => runSync(null)}
            disabled={syncing}
            aria-label={syncing ? undefined : t('topbar.syncAll')}
            className="sm:min-w-28"
          >
            <motion.span
              animate={syncing ? { rotate: 360 } : { rotate: 0 }}
              transition={
                syncing ? { repeat: Infinity, duration: 1, ease: 'linear' } : { duration: 0.2 }
              }
              className="inline-flex"
            >
              <SyncIcon size={15} />
            </motion.span>
            <span className="nums hidden sm:inline">
              {syncing
                ? `${t(`sync.phase.${globalJob.phase}`)} ${globalJob.percent}%`
                : t('topbar.syncAll')}
            </span>
          </Button>
        )}

        <NotificationCenter />

        {/* User menu */}
        <div className="relative">
          <button
            ref={menuButtonRef}
            onClick={() => setMenuOpen((v) => !v)}
            aria-haspopup="menu"
            aria-expanded={menuOpen}
            className="flex items-center gap-2 rounded-md py-1 pl-1 pr-2 transition-colors hover:bg-surface-raised"
          >
            <span className="flex h-7 w-7 items-center justify-center rounded-sm bg-signal-tint text-signal-bright">
              <UserIcon size={16} />
            </span>
            <span className="hidden text-sm font-medium text-ink sm:block">
              {user?.displayName ?? user?.username}
            </span>
            <ChevronDownIcon size={14} className="text-ink-faint" />
          </button>

          <AnimatePresence>
            {menuOpen && (
              <>
                <div
                  className="fixed inset-0 z-(--z-dropdown)"
                  onClick={() => setMenuOpen(false)}
                />
                <motion.div
                  role="menu"
                  aria-label={t('account.profile')}
                  initial={{ opacity: 0, y: -6, scale: 0.98 }}
                  animate={{ opacity: 1, y: 0, scale: 1 }}
                  exit={{ opacity: 0, y: -6, scale: 0.98 }}
                  transition={{ duration: 0.16, ease: [0.16, 1, 0.3, 1] }}
                  className="absolute right-0 top-[calc(100%+8px)] z-(--z-dropdown) w-56 overflow-hidden rounded-sm border border-line bg-surface-overlay shadow-(--shadow-pop)"
                >
                  <div className="border-b border-line-soft px-3 py-2.5">
                    <p className="text-sm font-medium text-ink">
                      {user?.displayName ?? user?.username}
                    </p>
                    {user?.email && <p className="text-xs text-ink-faint">{user.email}</p>}
                  </div>
                  <div className="p-1">
                    <button
                      ref={firstMenuItemRef}
                      role="menuitem"
                      onClick={() => {
                        setMenuOpen(false);
                        navigate('/settings/profile');
                      }}
                      className="w-full rounded-md px-2.5 py-1.5 text-left text-sm text-ink-muted transition-colors hover:bg-surface-raised hover:text-ink"
                    >
                      {t('account.profile')}
                    </button>
                    <button
                      role="menuitem"
                      onClick={() => {
                        setMenuOpen(false);
                        navigate('/settings/appearance');
                      }}
                      className="w-full rounded-md px-2.5 py-1.5 text-left text-sm text-ink-muted transition-colors hover:bg-surface-raised hover:text-ink"
                    >
                      {t('account.preferences')}
                    </button>
                    <button
                      role="menuitem"
                      onClick={() => {
                        setMenuOpen(false);
                        void logout().then(() => navigate('/login', { replace: true }));
                      }}
                      className="w-full rounded-md px-2.5 py-1.5 text-left text-sm text-ink-muted transition-colors hover:bg-surface-raised hover:text-ink"
                    >
                      {t('auth.signOut')}
                    </button>
                  </div>
                </motion.div>
              </>
            )}
          </AnimatePresence>
        </div>
      </div>
    </header>
  );
}
