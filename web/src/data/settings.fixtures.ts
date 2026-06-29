/**
 * Deterministic fixtures for the Settings hub (Phase 8). Hand-authored (NOT faker)
 * so the mocked settings surfaces render believable, stable data on every load.
 * Consumed only by `web/src/mocks/settings.mock.ts`.
 *
 * Timestamps are fixed ISO strings relative to a frozen "seed now" so relative-time
 * rendering stays sensible without being random.
 */
import type {
  AiConfig,
  Health,
  NotificationChannel,
  NotificationConfig,
  NotificationRoute,
  Session,
  SystemInfo,
  User,
} from '../api/model';

/** Frozen reference point the relative timestamps below are anchored to. */
const SEED_NOW = Date.parse('2026-06-28T12:00:00.000Z');
const ago = (ms: number) => new Date(SEED_NOW - ms).toISOString();
const MIN = 60_000;
const HOUR = 60 * MIN;
const DAY = 24 * HOUR;

// ── Users ───────────────────────────────────────────────────────────────────
// The signed-in operator (id matches data/fixtures.ts `user`) plus a small,
// believable roster: a second operator, a local viewer, an OIDC viewer, and a
// disabled account.
export const settingsUsers: User[] = [
  {
    id: 'usr-001',
    username: 'gsaraiva',
    displayName: 'G. Saraiva',
    email: 'gsaraiva@wiselabz.local',
    role: 'operator',
    authSource: 'local',
    disabled: false,
    createdAt: ago(180 * DAY),
  },
  {
    id: 'usr-002',
    username: 'mlopes',
    displayName: 'M. Lopes',
    email: 'mlopes@wiselabz.local',
    role: 'operator',
    authSource: 'oidc',
    disabled: false,
    createdAt: ago(96 * DAY),
  },
  {
    id: 'usr-003',
    username: 'readonly',
    displayName: 'Read Only',
    email: 'readonly@wiselabz.local',
    role: 'viewer',
    authSource: 'local',
    disabled: false,
    createdAt: ago(40 * DAY),
  },
  {
    id: 'usr-004',
    username: 'jbarros',
    displayName: 'J. Barros',
    email: 'jbarros@wiselabz.local',
    role: 'viewer',
    authSource: 'oidc',
    disabled: false,
    createdAt: ago(12 * DAY),
  },
  {
    id: 'usr-005',
    username: 'oldadmin',
    displayName: 'Former Admin',
    email: 'oldadmin@wiselabz.local',
    role: 'viewer',
    authSource: 'local',
    disabled: true,
    createdAt: ago(300 * DAY),
  },
];

// ── Active sessions (for /settings/profile) ──────────────────────────────────
export const settingsSessions: Session[] = [
  {
    id: 'ses-current',
    userAgent: 'Firefox 141 · Linux',
    ip: '10.0.1.14',
    createdAt: ago(2 * HOUR),
    lastSeenAt: ago(30_000),
    current: true,
  },
  {
    id: 'ses-laptop',
    userAgent: 'Chrome 140 · macOS',
    ip: '10.0.1.22',
    createdAt: ago(3 * DAY),
    lastSeenAt: ago(5 * HOUR),
    current: false,
  },
  {
    id: 'ses-phone',
    userAgent: 'Safari · iOS 18',
    ip: '100.84.2.9',
    createdAt: ago(9 * DAY),
    lastSeenAt: ago(2 * DAY),
    current: false,
  },
];

// ── AI module config ─────────────────────────────────────────────────────────
// apiKey is write-only — never returned in reads (mirrors the contract).
export const settingsAiConfig: AiConfig = {
  enabled: true,
  provider: 'anthropic',
  model: 'claude-sonnet-4-5',
  baseUrl: '',
  mode: 'suggest_only',
};

// ── Notification routing (event × channel matrix) ────────────────────────────
export const NOTIFICATION_EVENT_TYPES = [
  'alert.created',
  'change.detected',
  'sync.complete',
  'sync.failed',
  'service.offline',
] as const;

const settingsChannels: NotificationChannel[] = [
  { type: 'in_app', enabled: true },
  { type: 'smtp', enabled: true, config: { host: 'smtp.wiselabz.local', port: 587, from: 'wiselabz@home.lab' } },
  { type: 'webhook', enabled: false, config: { url: 'https://hooks.home.lab/wiselabz' } },
];

// Seed routing for every event×channel pair with sensible defaults.
function seedRouting(): NotificationRoute[] {
  const rows: NotificationRoute[] = [];
  for (const eventType of NOTIFICATION_EVENT_TYPES) {
    // Default minimum severity per event.
    const minSeverity =
      eventType === 'sync.complete' ? 'info' : eventType === 'change.detected' ? 'warning' : 'critical';
    for (const ch of settingsChannels) {
      // in_app on for everything; smtp on for the noisy/critical ones; webhook off by default.
      const enabled =
        ch.type === 'in_app'
          ? true
          : ch.type === 'smtp'
            ? eventType !== 'sync.complete'
            : false;
      rows.push({ eventType, channel: ch.type, enabled, minSeverity });
    }
  }
  return rows;
}

export const settingsNotificationConfig: NotificationConfig = {
  channels: settingsChannels,
  routing: seedRouting(),
};

// ── System / instance ────────────────────────────────────────────────────────
export const settingsSystemInfo: SystemInfo = {
  version: '0.1.0',
  syncSchedule: '*/15 * * * *',
  integrations: [
    { name: 'Database (SQLite)', status: 'ok', detail: 'wiselabz.db · 14.2 MB' },
    { name: 'WebSocket hub', status: 'ok', detail: '3 clients connected' },
    { name: 'Scheduler', status: 'ok', detail: 'next run in ~6m' },
    { name: 'AI provider (Anthropic)', status: 'degraded', detail: 'elevated latency (1.8s p95)' },
  ],
};

export const settingsHealth: Health = {
  status: 'degraded',
  components: [
    { name: 'api', status: 'ok', detail: 'p95 38ms' },
    { name: 'database', status: 'ok' },
    { name: 'scheduler', status: 'ok' },
    { name: 'ai', status: 'degraded', detail: 'provider latency elevated' },
  ],
};
