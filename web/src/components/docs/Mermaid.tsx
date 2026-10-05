/**
 * Renders a Mermaid diagram source string as SVG. Themed off the app's live
 * CSS custom properties (set by the palette system in ../../store/theme.ts)
 * rather than a light/dark boolean, since this app has no such flag — just
 * re-reads the current --color-* values, so it re-subscribes to useTheme to
 * re-render whenever the palette changes.
 */
import { useEffect, useId, useRef } from 'react';
import { useTheme } from '../../store/theme';

function cssVar(name: string, fallback: string): string {
  if (typeof window === 'undefined') return fallback;
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return value || fallback;
}

// Color resolution cache to avoid DOM probes on every render.
const colorCache = new Map<string, string>();

// Converts any CSS color string to rgb()/rgba() by painting one pixel.
// Modern browsers keep oklch() as-is in computed styles, and khroma (mermaid's
// color parser) throws "Unsupported color format" on it, so the computed value
// alone is not enough. Returns undefined where canvas is unavailable.
function toRgb(color: string): string | undefined {
  try {
    const ctx = document.createElement('canvas').getContext('2d');
    if (!ctx) return undefined;
    ctx.clearRect(0, 0, 1, 1);
    ctx.fillStyle = color;
    ctx.fillRect(0, 0, 1, 1);
    const [r, g, b, a] = ctx.getImageData(0, 0, 1, 1).data;
    return a === 255 ? `rgb(${r}, ${g}, ${b})` : `rgba(${r}, ${g}, ${b}, ${(a / 255).toFixed(3)})`;
  } catch {
    return undefined;
  }
}

// Resolves a CSS custom property to a color format mermaid's own color
// parser (khroma) understands. The app's palette tokens are oklch(...),
// which khroma can't parse, so the browser resolves the variable and the
// result is converted to rgb() when it is not already in a format khroma
// reads.
function resolveColor(varName: string, fallback: string): string {
  if (typeof document === 'undefined') return fallback;

  const cached = colorCache.get(varName);
  if (cached) return cached;

  const probe = document.createElement('span');
  probe.style.color = `var(${varName})`;
  document.body.appendChild(probe);
  const resolved = getComputedStyle(probe).color;
  document.body.removeChild(probe);

  let result = resolved && !resolved.includes('var(') ? resolved : fallback;
  if (!/^(rgba?\(|#)/i.test(result)) result = toRgb(result) ?? fallback;
  colorCache.set(varName, result);
  return result;
}

// Track current theme to detect changes and clear color cache.
let lastThemeKey: string | null = null;

export function Mermaid({ chart }: { chart: string }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const id = useId().replace(/[^a-zA-Z0-9]/g, '');
  const mode = useTheme((s) => s.mode);
  const preset = useTheme((s) => s.preset);
  const custom = useTheme((s) => s.custom);

  // Initialize mermaid once per theme change.
  useEffect(() => {
    const themeKey = `${mode}-${preset}-${custom}`;
    if (lastThemeKey !== themeKey) {
      lastThemeKey = themeKey;
      colorCache.clear();

      import('mermaid').then(({ default: mermaid }) => {
        mermaid.initialize({
          startOnLoad: false,
          theme: 'base',
          themeVariables: {
            background: resolveColor('--color-canvas', '#0b0c0f'),
            primaryColor: resolveColor('--color-canvas-sunken', '#15171b'),
            primaryTextColor: resolveColor('--color-ink', '#e8e8ea'),
            primaryBorderColor: resolveColor('--color-line-soft', '#2a2d33'),
            lineColor: resolveColor('--color-accent-primary', '#5b8cff'),
            fontFamily: cssVar('--font-mono', 'ui-monospace, monospace'),
          },
        });
      });
    }
  }, [mode, preset, custom]);

  // Render diagram on chart change.
  useEffect(() => {
    let cancelled = false;
    const container = containerRef.current;
    if (!container) return;

    import('mermaid')
      .then(({ default: mermaid }) => mermaid.render(`mermaid-${id}`, chart))
      .then(({ svg }) => {
        if (!cancelled && containerRef.current) containerRef.current.innerHTML = svg;
      })
      .catch((err: unknown) => {
        if (!cancelled && containerRef.current) {
          containerRef.current.textContent = `Diagram error: ${err instanceof Error ? err.message : String(err)}`;
        }
      });

    return () => {
      cancelled = true;
    };
  }, [chart, id]);

  return (
    <div
      ref={containerRef}
      className="my-3 overflow-x-auto rounded-lg border border-line-soft bg-canvas-sunken p-3 text-xs text-ink-muted"
    />
  );
}
