/** Status + severity mapping. Signal-orange is brand-only; status uses its own
 *  restrained data palette (ok/warn/err/idle), always paired with a word. */
import type { ServiceStatus, Severity } from '../../api/model';

export type Tone = 'ok' | 'warn' | 'err' | 'signal' | 'idle';

export const statusMeta: Record<ServiceStatus, { label: string; tone: Tone }> = {
  online: { label: 'online', tone: 'ok' },
  degraded: { label: 'degraded', tone: 'warn' },
  offline: { label: 'offline', tone: 'err' },
  unknown: { label: 'unknown', tone: 'idle' },
};

export const severityMeta: Record<Severity, { label: string; tone: Tone }> = {
  info: { label: 'info', tone: 'idle' },
  warning: { label: 'warning', tone: 'warn' },
  critical: { label: 'critical', tone: 'err' },
};

export const toneColor: Record<Tone, { fg: string; tint: string }> = {
  ok: { fg: 'var(--color-ok)', tint: 'var(--color-ok-tint)' },
  warn: { fg: 'var(--color-warn)', tint: 'var(--color-warn-tint)' },
  err: { fg: 'var(--color-err)', tint: 'var(--color-err-tint)' },
  signal: { fg: 'var(--color-signal-bright)', tint: 'var(--color-signal-tint)' },
  idle: { fg: 'var(--color-idle)', tint: 'var(--color-idle-tint)' },
};
