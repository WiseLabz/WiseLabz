/**
 * Live theme switcher (Settings → Theme). Font picker + color picker with two
 * modes: Basic (preset swatches) and Advanced (every makePalette knob on a
 * slider). Changes apply instantly and persist via the theme store.
 */
import { motion } from 'motion/react';
import { useTranslation } from 'react-i18next';
import { useTheme } from '../../store/theme';
import {
  FONT_SETS,
  OPT_META,
  PRESETS,
  makePalette,
  type FontSetName,
  type PaletteName,
  type PaletteOpts,
  type PaletteTokens,
} from '../../theme';
import { Panel, PanelHeader } from '../../components/ui/Panel';
import { Button } from '../../components/ui/Button';
import { cn } from '../../lib/cn';
import { SettingsIcon } from '../../components/icons';

const FONT_KEYS = Object.keys(FONT_SETS) as FontSetName[];
const PRESET_KEYS = Object.keys(PRESETS) as PaletteName[];
const OPT_KEYS = Object.keys(OPT_META) as (keyof PaletteOpts)[];

/** Small color preview built from a token set. */
function Swatch({ tokens, font }: { tokens: PaletteTokens; font?: FontSetName }) {
  return (
    <div
      className="flex items-center gap-2 rounded-sm border px-2.5 py-2"
      style={{
        backgroundColor: tokens['--color-canvas'],
        borderColor: tokens['--color-line'],
        fontFamily: font ? FONT_SETS[font].mono.replace(/'/g, '') : undefined,
      }}
    >
      <span className="h-4 w-4 rounded-sm" style={{ backgroundColor: tokens['--color-signal'] }} />
      <span className="flex gap-1">
        <span className="h-4 w-1.5" style={{ backgroundColor: tokens['--color-ok'] }} />
        <span className="h-4 w-1.5" style={{ backgroundColor: tokens['--color-warn'] }} />
        <span className="h-4 w-1.5" style={{ backgroundColor: tokens['--color-err'] }} />
      </span>
      <span className="ml-auto text-2xs" style={{ color: tokens['--color-ink-muted'] }}>
        Aa 01
      </span>
    </div>
  );
}

function Segmented<T extends string>({
  options,
  value,
  onChange,
  layoutId,
}: {
  options: { value: T; label: string }[];
  value: T;
  onChange: (v: T) => void;
  layoutId: string;
}) {
  return (
    <div className="inline-flex rounded-sm border border-line-soft bg-canvas-sunken p-0.5">
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button
            key={o.value}
            onClick={() => onChange(o.value)}
            className={cn(
              'relative rounded-sm px-3 py-1.5 font-mono text-xs transition-colors',
              active ? 'text-ink' : 'text-ink-muted hover:text-ink'
            )}
          >
            {active && (
              <motion.span
                layoutId={layoutId}
                className="absolute inset-0 -z-10 rounded-sm bg-surface-raised"
                transition={{ type: 'spring', stiffness: 500, damping: 36 }}
              />
            )}
            {o.label}
          </button>
        );
      })}
    </div>
  );
}

export function ThemeControls() {
  const { t } = useTranslation();
  const { font, mode, preset, custom } = useTheme();
  const setFont = useTheme((s) => s.setFont);
  const setMode = useTheme((s) => s.setMode);
  const setPreset = useTheme((s) => s.setPreset);
  const setCustomOpt = useTheme((s) => s.setCustomOpt);
  const forkPresetToCustom = useTheme((s) => s.forkPresetToCustom);
  const reset = useTheme((s) => s.reset);

  return (
    <Panel>
      <PanelHeader
        title={t('appearance.theme')}
        icon={<SettingsIcon size={14} />}
        action={
          <Button size="sm" variant="ghost" onClick={reset}>
            {t('appearance.reset')}
          </Button>
        }
      />

      <div className="flex flex-col gap-6 p-5">
        {/* Font */}
        <section>
          <Label>{t('appearance.font')}</Label>
          <div className="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-4">
            {FONT_KEYS.map((k) => {
              const active = font === k;
              return (
                <button
                  key={k}
                  onClick={() => setFont(k)}
                  className={cn(
                    'flex flex-col items-start gap-1 rounded-sm border px-3 py-2.5 text-left transition-colors',
                    active
                      ? 'border-signal-soft bg-signal-tint'
                      : 'border-line-soft hover:border-line-strong'
                  )}
                >
                  <span
                    className="text-base font-semibold text-ink"
                    style={{ fontFamily: FONT_SETS[k].mono.replace(/'/g, '') }}
                  >
                    Ag
                  </span>
                  <span className="font-mono text-2xs text-ink-muted">{FONT_SETS[k].label}</span>
                </button>
              );
            })}
          </div>
        </section>

        {/* Color */}
        <section>
          <div className="flex items-center justify-between">
            <Label>{t('appearance.color')}</Label>
            <Segmented
              layoutId="theme-mode"
              value={mode}
              onChange={setMode}
              options={[
                { value: 'preset', label: t('appearance.basic') },
                { value: 'custom', label: t('appearance.advanced') },
              ]}
            />
          </div>

          {mode === 'preset' ? (
            <div className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2">
              {PRESET_KEYS.map((k) => {
                const tokens = makePalette(PRESETS[k].opts);
                const active = preset === k;
                return (
                  <button
                    key={k}
                    onClick={() => setPreset(k)}
                    className={cn(
                      'flex flex-col gap-2 rounded-sm border p-2.5 text-left transition-colors',
                      active ? 'border-signal-soft' : 'border-line-soft hover:border-line-strong'
                    )}
                  >
                    <div className="flex items-baseline justify-between">
                      <span className="font-mono text-xs font-medium text-ink">
                        {PRESETS[k].label}
                      </span>
                      <span className="font-mono text-2xs text-ink-faint">{PRESETS[k].desc}</span>
                    </div>
                    <Swatch tokens={tokens} font={font} />
                  </button>
                );
              })}
            </div>
          ) : (
            <AdvancedControls
              custom={custom}
              setCustomOpt={setCustomOpt}
              onSeed={forkPresetToCustom}
              preset={preset}
              font={font}
            />
          )}
        </section>
      </div>
    </Panel>
  );
}

function AdvancedControls({
  custom,
  setCustomOpt,
  onSeed,
  preset,
  font,
}: {
  custom: PaletteOpts;
  setCustomOpt: <K extends keyof PaletteOpts>(k: K, v: PaletteOpts[K]) => void;
  onSeed: () => void;
  preset: PaletteName;
  font: FontSetName;
}) {
  const { t } = useTranslation();
  const tokens = makePalette(custom);
  return (
    <div className="mt-3 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="max-w-[42ch] font-mono text-2xs text-ink-faint">
          {t('appearance.advancedHint')}
        </p>
        <Button size="sm" variant="secondary" onClick={onSeed}>
          {t('appearance.startFrom', { name: PRESETS[preset].label })}
        </Button>
      </div>

      <Swatch tokens={tokens} font={font} />

      <div className="grid gap-x-6 gap-y-4 sm:grid-cols-2">
        {OPT_KEYS.map((key) => {
          const meta = OPT_META[key];
          const value = custom[key];
          const display = meta.step >= 1 ? `${value}°` : value.toFixed(meta.step < 0.01 ? 3 : 2);
          return (
            <label key={key} className="block">
              <span className="flex items-baseline justify-between">
                <span className="font-mono text-xs text-ink">{meta.label}</span>
                <span className="nums font-mono text-2xs text-signal-bright">{display}</span>
              </span>
              <input
                type="range"
                min={meta.min}
                max={meta.max}
                step={meta.step}
                value={value}
                onChange={(e) => setCustomOpt(key, Number(e.target.value))}
                className="theme-range mt-1.5 w-full"
                aria-label={meta.label}
              />
              <span className="font-mono text-2xs text-ink-faint">{meta.hint}</span>
            </label>
          );
        })}
      </div>
    </div>
  );
}

function Label({ children }: { children: React.ReactNode }) {
  return (
    <span className="font-mono text-2xs uppercase tracking-[0.16em] text-ink-faint">
      {children}
    </span>
  );
}
