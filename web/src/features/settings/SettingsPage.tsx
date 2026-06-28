/**
 * Settings → Appearance. Home of the motion control: WiseLabz ships motion ON by
 * default (it's a core part of the product's feel), and this is where a user dials
 * it back. The OS reduced-motion preference only seeds the initial value — the
 * choice here wins and persists.
 */
import { motion } from 'motion/react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useSettings, type MotionPref } from '../../store/settings';
import {
  useGetAuthConfig,
  putAuthConfig,
  getGetAuthConfigQueryKey,
} from '../../api/generated/settings/settings';
import { useCanMutate } from '../../hooks/useRole';
import { Panel } from '../../components/ui/Panel';
import { ThemeControls } from './ThemeControls';
import { cn } from '../../lib/cn';
import { SparklesIcon, GaugeIcon, XIcon } from '../../components/icons';

const MOTION_OPTIONS: {
  value: MotionPref;
  Icon: React.ComponentType<{ size?: number }>;
}[] = [
  { value: 'full', Icon: SparklesIcon },
  { value: 'reduced', Icon: GaugeIcon },
  { value: 'off', Icon: XIcon },
];

export function SettingsPage() {
  const { t } = useTranslation();
  const motionPref = useSettings((s) => s.motion);
  const setMotion = useSettings((s) => s.setMotion);

  return (
    <div className="mx-auto max-w-[760px] px-6 py-6">
      <header className="mb-6">
        <h1 className="text-xl font-semibold tracking-tight text-[var(--color-ink)]">{t('settings.title')}</h1>
        <p className="text-sm text-[var(--color-ink-muted)]">{t('settings.subtitle')}</p>
      </header>

      <div className="mb-4">
        <ThemeControls />
      </div>

      <Panel className="p-6">
        <div className="mb-4">
          <h2 className="text-sm font-semibold text-[var(--color-ink)]">{t('settings.motion.heading')}</h2>
          <p className="mt-0.5 text-xs text-[var(--color-ink-muted)]">
            {t('settings.motion.desc')}
          </p>
        </div>

        <div role="radiogroup" aria-label={t('settings.motion.groupLabel')} className="grid gap-3 sm:grid-cols-3">
          {MOTION_OPTIONS.map((opt) => {
            const active = motionPref === opt.value;
            return (
              <button
                key={opt.value}
                role="radio"
                aria-checked={active}
                onClick={() => setMotion(opt.value)}
                className={cn(
                  'relative flex flex-col gap-2 rounded-lg border p-3.5 text-left transition-colors',
                  active
                    ? 'border-[var(--color-signal-soft)] bg-[var(--color-signal-tint)]'
                    : 'border-[var(--color-line-soft)] hover:border-[var(--color-line-strong)]',
                )}
              >
                <span
                  className="flex h-8 w-8 items-center justify-center rounded-md"
                  style={{
                    color: active ? 'var(--color-signal-bright)' : 'var(--color-ink-faint)',
                    backgroundColor: 'var(--color-canvas-sunken)',
                  }}
                >
                  <opt.Icon size={16} />
                </span>
                <span className="text-sm font-medium text-[var(--color-ink)]">{t(`settings.motion.${opt.value}Label`)}</span>
                <span className="text-2xs leading-relaxed text-[var(--color-ink-muted)]">
                  {t(`settings.motion.${opt.value}Desc`)}
                </span>
                {active && (
                  <motion.span
                    layoutId="motion-pick"
                    className="absolute right-2.5 top-2.5 h-2 w-2 rounded-full bg-[var(--color-signal-bright)]"
                    transition={{ type: 'spring', stiffness: 500, damping: 32 }}
                  />
                )}
              </button>
            );
          })}
        </div>

        {/* Live preview so the choice is tangible */}
        <div className="mt-5 rounded-lg border border-[var(--color-line-soft)] bg-[var(--color-canvas-sunken)] p-4">
          <p className="mb-2 text-2xs uppercase tracking-wider text-[var(--color-ink-faint)]">
            {t('settings.motion.preview')}
          </p>
          <div className="flex gap-2">
            {[0, 1, 2].map((i) => (
              <motion.span
                key={`${motionPref}-${i}`}
                className="h-8 flex-1 rounded-md bg-[var(--color-signal-soft)]"
                initial={{ opacity: 0, y: 12 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: i * 0.08, type: 'spring', stiffness: 320, damping: 24 }}
              />
            ))}
          </div>
        </div>
      </Panel>

      <SecuritySettings />
    </div>
  );
}

/** Operator-only: the step-up escape hatch for the single-admin homelab. */
function SecuritySettings() {
  const { t } = useTranslation();
  const canMutate = useCanMutate();
  const queryClient = useQueryClient();
  const { data } = useGetAuthConfig({ query: { enabled: canMutate } });
  const update = useMutation({
    mutationFn: (stepUpForDestructive: boolean) => putAuthConfig({ stepUpForDestructive }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: getGetAuthConfigQueryKey() }),
  });

  if (!canMutate) return null;
  const enabled = data?.stepUpForDestructive ?? true;

  return (
    <Panel className="mt-4 p-6">
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-[var(--color-ink)]">{t('settings.security.heading')}</h2>
        <p className="mt-0.5 text-xs text-[var(--color-ink-muted)]">
          {t('settings.security.desc')}
        </p>
      </div>

      <div className="flex items-start justify-between gap-4 rounded-lg border border-[var(--color-line-soft)] p-3.5">
        <div className="flex-1">
          <p className="text-sm font-medium text-[var(--color-ink)]">
            {t('settings.security.stepUpTitle')}
          </p>
          <p className="mt-0.5 text-2xs leading-relaxed text-[var(--color-ink-muted)]">
            {t('settings.security.stepUpDesc')}
          </p>
        </div>
        <button
          role="switch"
          aria-checked={enabled}
          aria-label={t('settings.security.stepUpTitle')}
          disabled={update.isPending}
          onClick={() => update.mutate(!enabled)}
          className="relative mt-0.5 h-5 w-9 shrink-0 rounded-full transition-colors disabled:opacity-50"
          style={{ backgroundColor: enabled ? 'var(--color-signal)' : 'var(--color-line-strong)' }}
        >
          <motion.span
            layout
            transition={{ type: 'spring', stiffness: 600, damping: 34 }}
            className="absolute top-0.5 h-4 w-4 rounded-full bg-white"
            style={{ left: enabled ? '18px' : '2px' }}
          />
        </button>
      </div>
    </Panel>
  );
}
