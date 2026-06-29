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
        title={t('settings.appearance.title', { defaultValue: 'Appearance' })}
        description={t('settings.appearance.subtitle', {
          defaultValue: 'Theme, motion, and accessibility preferences.',
        })}
      />

      <ThemeControls />

      <div className="mt-4">
        <Section
          title={t('settings.motion.heading', { defaultValue: 'Motion' })}
          description={t('settings.motion.desc', {
            defaultValue:
              'WiseLabz uses motion to make live state legible. Turn it down here if you prefer a calmer interface — your choice is remembered.',
          })}
        >
          <ChoiceGroup<MotionPref>
            label={t('settings.motion.groupLabel', { defaultValue: 'Motion preference' })}
            value={motionPref}
            onChange={setMotion}
            options={[
              {
                value: 'full',
                label: t('settings.motion.fullLabel', { defaultValue: 'Full' }),
                desc: t('settings.motion.fullDesc', {
                  defaultValue: 'Every transition and signature moment. The default.',
                }),
              },
              {
                value: 'reduced',
                label: t('settings.motion.reducedLabel', { defaultValue: 'Reduced' }),
                desc: t('settings.motion.reducedDesc', {
                  defaultValue: 'Essential feedback only — springs collapse to fades.',
                }),
              },
              {
                value: 'off',
                label: t('settings.motion.offLabel', { defaultValue: 'Off' }),
                desc: t('settings.motion.offDesc', {
                  defaultValue: 'No animation. Instant state changes everywhere.',
                }),
              },
            ]}
          />
        </Section>
      </div>

      <Section
        title={t('settings.appearance.a11yTitle', { defaultValue: 'Accessibility' })}
        description={t('settings.appearance.a11yDesc', {
          defaultValue: 'Tune legibility and comfort. Changes apply instantly.',
        })}
      >
        <div className="space-y-5">
          <div>
            <p className="mb-2 font-mono text-2xs uppercase tracking-[0.16em] text-ink-faint">
              {t('settings.appearance.textSize', { defaultValue: 'Text size' })}
            </p>
            <ChoiceGroup<TextSize>
              label={t('settings.appearance.textSize', { defaultValue: 'Text size' })}
              value={textSize}
              onChange={setTextSize}
              options={[
                {
                  value: 'sm',
                  label: t('settings.appearance.textSm', { defaultValue: 'Compact' }),
                },
                {
                  value: 'base',
                  label: t('settings.appearance.textBase', { defaultValue: 'Default' }),
                },
                { value: 'lg', label: t('settings.appearance.textLg', { defaultValue: 'Large' }) },
              ]}
            />
          </div>

          <div>
            <p className="mb-2 font-mono text-2xs uppercase tracking-[0.16em] text-ink-faint">
              {t('settings.appearance.density', { defaultValue: 'Density' })}
            </p>
            <ChoiceGroup<Density>
              label={t('settings.appearance.density', { defaultValue: 'Density' })}
              value={density}
              onChange={setDensity}
              options={[
                {
                  value: 'comfortable',
                  label: t('settings.appearance.comfortable', { defaultValue: 'Comfortable' }),
                },
                {
                  value: 'compact',
                  label: t('settings.appearance.compact', { defaultValue: 'Compact' }),
                },
              ]}
            />
          </div>

          <div>
            <p className="mb-2 font-mono text-2xs uppercase tracking-[0.16em] text-ink-faint">
              {t('settings.appearance.focusRing', { defaultValue: 'Focus ring' })}
            </p>
            <ChoiceGroup<FocusRing>
              label={t('settings.appearance.focusRing', { defaultValue: 'Focus ring' })}
              value={focusRing}
              onChange={setFocusRing}
              options={[
                {
                  value: 'standard',
                  label: t('settings.appearance.focusStandard', { defaultValue: 'Standard' }),
                },
                {
                  value: 'bold',
                  label: t('settings.appearance.focusBold', { defaultValue: 'Bold' }),
                },
              ]}
            />
          </div>

          <ToggleRow
            title={t('settings.appearance.contrastTitle', { defaultValue: 'Increase contrast' })}
            description={t('settings.appearance.contrastDesc', {
              defaultValue: 'Brighten secondary text for easier reading.',
            })}
            checked={contrast === 'boost'}
            onChange={(v) => setContrast((v ? 'boost' : 'normal') as Contrast)}
          />

          <ToggleRow
            title={t('settings.appearance.reduceTransparencyTitle', {
              defaultValue: 'Reduce transparency',
            })}
            description={t('settings.appearance.reduceTransparencyDesc', {
              defaultValue: 'Make modal and overlay backdrops fully opaque.',
            })}
            checked={reduceTransparency}
            onChange={setReduceTransparency}
          />
        </div>
      </Section>
    </div>
  );
}
