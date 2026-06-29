/**
 * Global top bar: collapse toggle, a ⌘K search affordance, the global-sync
 * button (shows live phase while a fleet sync runs), the alert bell with live
 * count, and the user menu. The sync button is the entry point to the dashboard's
 * signature sweep.
 */
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { AnimatePresence, motion } from 'motion/react';
import { useUi } from '../../store/ui';
import { useLive } from '../../store/live';
import { useAuth } from '../../store/auth';
import { useCanMutate } from '../../hooks/useRole';
import { runSync } from '../../lib/runSync';
import { Button, IconButton } from '../ui/Button';
import { SearchIcon, SyncIcon, BellIcon, UserIcon, ChevronDownIcon } from '../icons';

export function Topbar() {
  const togglePalette = useUi((s) => s.togglePalette);
  const pending = useLive((s) => s.pendingAlerts);
  const globalJob = useLive((s) => s.jobs.global);
  const canMutate = useCanMutate();
  const user = useAuth((s) => s.user);
  const logout = useAuth((s) => s.logout);
  const [menuOpen, setMenuOpen] = useState(false);
  const navigate = useNavigate();
  const { t } = useTranslation();

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
        className="group flex h-9 w-full max-w-sm items-center gap-2.5 rounded-sm border border-line bg-canvas-sunken px-3 font-mono text-xs text-ink-faint transition-colors hover:border-line-strong hover:text-ink-muted"
      >
        <SearchIcon size={15} />
        <span className="flex-1 text-left">{t('topbar.searchPlaceholder')}</span>
        <kbd className="nums rounded border border-line-strong bg-canvas-sunken px-1.5 py-0.5 font-mono text-2xs text-ink-faint">
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
            className="min-w-28"
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
            <span className="nums">
              {syncing
                ? `${t(`sync.phase.${globalJob.phase}`)} ${globalJob.percent}%`
                : t('topbar.syncAll')}
            </span>
          </Button>
        )}

        <div className="relative">
          <IconButton
            label={t('topbar.alerts')}
            onClick={() => navigate('/alerts')}
            className="relative"
          >
            <BellIcon size={18} />
            <AnimatePresence>
              {pending > 0 && (
                <motion.span
                  initial={{ scale: 0 }}
                  animate={{ scale: 1 }}
                  exit={{ scale: 0 }}
                  transition={{ type: 'spring', stiffness: 600, damping: 22 }}
                  className="nums absolute -right-0.5 -top-0.5 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-err px-1 text-[10px] font-bold text-signal-ink"
                >
                  {pending}
                </motion.span>
              )}
            </AnimatePresence>
          </IconButton>
        </div>

        {/* User menu */}
        <div className="relative">
          <button
            onClick={() => setMenuOpen((v) => !v)}
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
                      onClick={() => {
                        setMenuOpen(false);
                        navigate('/settings');
                      }}
                      className="w-full rounded-md px-2.5 py-1.5 text-left text-sm text-ink-muted transition-colors hover:bg-surface-raised hover:text-ink"
                    >
                      {t('account.profile')}
                    </button>
                    <button
                      onClick={() => {
                        setMenuOpen(false);
                        navigate('/settings');
                      }}
                      className="w-full rounded-md px-2.5 py-1.5 text-left text-sm text-ink-muted transition-colors hover:bg-surface-raised hover:text-ink"
                    >
                      {t('account.preferences')}
                    </button>
                    <button
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
