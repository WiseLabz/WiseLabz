/**
 * Settings → Appearance (accessibility hub). Keeps the existing theme/OKLCH power
 * surface (ThemeControls) and the Motion preference, then adds the accessibility
 * controls: contrast boost, text size, density, reduce-transparency, focus-ring
 * weight. Everything applies LIVE and persists (theme store, settings store,
 * appearance store). Available to every user.
 */
import { motion } from 'motion/react';
import { useTranslation } from 'react-i18next';
import { useSettings, type MotionPref } from '../../store/settings';
import {
  useAppearance,
  type Contrast,
  type TextSize,
  type Density,
  type FocusRing,
} from '../../store/appearance';
import { ThemeControls } from './ThemeControls';
import { SubHeader, Section, ToggleRow } from './parts';
import { cn } from '../../lib/cn';

/** Generic accessible radio-group of cards. */
function ChoiceGroup<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: { value: T; label: string; desc?: string }[];
  onChange: (v: T) => void;
}) {
  return (
    <div role="radiogroup" aria-label={label} className="grid gap-2 sm:grid-cols-3">
      {options.map((opt) => {
        const active = opt.value === value;
        return (
          <button
            key={opt.value}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(opt.value)}
            className={cn(
              'relative rounded-lg border p-3 text-left transition-colors',
              active
                ? 'border-signal-soft bg-signal-tint'
                : 'border-line-soft hover:border-line-strong'
            )}
          >
            <span className="block text-sm font-medium text-ink">{opt.label}</span>
            {opt.desc && (
              <span className="mt-0.5 block text-2xs leading-relaxed text-ink-muted">
                {opt.desc}
              </span>
            )}
            {active && (
              <motion.span
                layoutId={`choice-${label}`}
                className="absolute right-2.5 top-2.5 h-2 w-2 rounded-full bg-signal-bright"
                transition={{ type: 'spring', stiffness: 500, damping: 32 }}
              />
            )}
          </button>
        );
      })}
    </div>
  );
}

export function AppearancePage() {
  const { t } = useTranslation();
  const motionPref = useSettings((s) => s.motion);
  const setMotion = useSettings((s) => s.setMotion);

  const {
    contrast,
    textSize,
    density,
    reduceTransparency,
    focusRing,
    setContrast,
    setTextSize,
    setDensity,
    setReduceTransparency,
    setFocusRing,
  } = useAppearance();

  return (
    <div>
      <SubHeader
        title={t('settings.appearance.title')}
        description={t('settings.appearance.subtitle')}
      />

      <ThemeControls />

      <div className="mt-4">
        <Section title={t('settings.motion.heading')} description={t('settings.motion.desc')}>
          <ChoiceGroup<MotionPref>
            label={t('settings.motion.groupLabel')}
            value={motionPref}
            onChange={setMotion}
            options={[
              {
                value: 'full',
                label: t('settings.motion.fullLabel'),
                desc: t('settings.motion.fullDesc'),
              },
              {
                value: 'reduced',
                label: t('settings.motion.reducedLabel'),
                desc: t('settings.motion.reducedDesc'),
              },
              {
                value: 'off',
                label: t('settings.motion.offLabel'),
                desc: t('settings.motion.offDesc'),
              },
            ]}
          />
        </Section>
      </div>

      <Section
        title={t('settings.appearance.a11yTitle')}
        description={t('settings.appearance.a11yDesc')}
      >
        <div className="space-y-5">
          <div>
            <p className="mb-2 font-mono text-2xs uppercase tracking-[0.16em] text-ink-faint">
              {t('settings.appearance.textSize')}
            </p>
            <ChoiceGroup<TextSize>
              label={t('settings.appearance.textSize')}
              value={textSize}
              onChange={setTextSize}
              options={[
                {
                  value: 'sm',
                  label: t('settings.appearance.textSm'),
                },
                {
                  value: 'base',
                  label: t('settings.appearance.textBase'),
                },
                { value: 'lg', label: t('settings.appearance.textLg') },
              ]}
            />
          </div>

          <div>
            <p className="mb-2 font-mono text-2xs uppercase tracking-[0.16em] text-ink-faint">
              {t('settings.appearance.density')}
            </p>
            <ChoiceGroup<Density>
              label={t('settings.appearance.density')}
              value={density}
              onChange={setDensity}
              options={[
                {
                  value: 'comfortable',
                  label: t('settings.appearance.comfortable'),
                },
                {
                  value: 'compact',
                  label: t('settings.appearance.compact'),
                },
              ]}
            />
          </div>

          <div>
            <p className="mb-2 font-mono text-2xs uppercase tracking-[0.16em] text-ink-faint">
              {t('settings.appearance.focusRing')}
            </p>
            <ChoiceGroup<FocusRing>
              label={t('settings.appearance.focusRing')}
              value={focusRing}
              onChange={setFocusRing}
              options={[
                {
                  value: 'standard',
                  label: t('settings.appearance.focusStandard'),
                },
                {
                  value: 'bold',
                  label: t('settings.appearance.focusBold'),
                },
              ]}
            />
          </div>

          <ToggleRow
            title={t('settings.appearance.contrastTitle')}
            description={t('settings.appearance.contrastDesc')}
            checked={contrast === 'boost'}
            onChange={(v) => setContrast((v ? 'boost' : 'normal') as Contrast)}
          />

          <ToggleRow
            title={t('settings.appearance.reduceTransparencyTitle')}
            description={t('settings.appearance.reduceTransparencyDesc')}
            checked={reduceTransparency}
            onChange={setReduceTransparency}
          />
        </div>
      </Section>
    </div>
  );
}
