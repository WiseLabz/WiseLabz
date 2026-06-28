/**
 * Hand-authored MSW handlers for the surfaces we showcase. Registered BEFORE the
 * generated faker handlers in handlers.ts, so MSW's first-match wins and these
 * deterministic fixtures render instead of random faker data. Anything not
 * covered here still falls through to the generated mocks.
 */
import { http, HttpResponse, delay } from 'msw';
import {
  alertPage,
  changeDetail,
  changePage,
  connectorSchemas,
  connectors,
  dashboardOverview,
  docTree,
  docVersionContent,
  docVersionMeta,
  docs,
  removalImpact,
  user,
} from '../data/fixtures';

// Small artificial latency so loading skeletons are actually exercised on first paint.
const LATENCY = 280;

// Session-mutable auth config so the Settings step-up toggle persists across reads.
const authConfig = {
  localEnabled: true,
  accessTokenTtl: 900,
  refreshTokenTtl: 2592000,
  stepUpForDestructive: true,
  oidcProviders: [] as unknown[],
};

// Mock session state. Login/oidc set it true; refresh succeeds only when set (i.e.
// a refresh "cookie" exists); logout clears it. Resets on full page reload, which
// matches the in-memory-token model (no persistent client session).
let hasSession = false;
const newSession = () => ({
  accessToken: `acc-${Math.random().toString(36).slice(2)}`,
  expiresIn: authConfig.accessTokenTtl,
  user,
});

export const curatedHandlers = [
  http.get('*/auth/providers', async () => {
    await delay(LATENCY);
    return HttpResponse.json({
      localEnabled: authConfig.localEnabled,
      oidc: [
        {
          id: 'authentik',
          displayName: 'Authentik',
          authUrl: '/auth/callback?providerId=authentik&code=mock-oidc-code&state=mock-state',
        },
      ],
    });
  }),

  http.post('*/auth/login', async ({ request }) => {
    await delay(LATENCY);
    const body = (await request.json().catch(() => ({}))) as { username?: string; password?: string };
    if (!body.username || !body.password) {
      return HttpResponse.json({ code: 'invalid-credentials', message: 'Invalid credentials' }, { status: 401 });
    }
    hasSession = true;
    return HttpResponse.json(newSession());
  }),

  http.post('*/auth/oidc/callback', async () => {
    await delay(LATENCY);
    hasSession = true;
    return HttpResponse.json(newSession());
  }),

  http.post('*/auth/refresh', async () => {
    await delay(LATENCY);
    if (!hasSession) {
      return HttpResponse.json({ code: 'no-session', message: 'No active session' }, { status: 401 });
    }
    return HttpResponse.json(newSession());
  }),

  http.post('*/auth/logout', async () => {
    await delay(LATENCY);
    hasSession = false;
    return new HttpResponse(null, { status: 204 });
  }),

  http.get('*/auth/config', async () => {
    await delay(LATENCY);
    return HttpResponse.json(authConfig);
  }),

  http.put('*/auth/config', async ({ request }) => {
    await delay(LATENCY);
    const body = (await request.json().catch(() => ({}))) as Partial<typeof authConfig>;
    Object.assign(authConfig, body);
    return HttpResponse.json(authConfig);
  }),

  // Manager mutations — operate on the fixture connectors in place so toggles and
  // removals visibly persist for the session.
  http.put('*/connectors/:connectorId/enabled', async ({ params, request }) => {
    await delay(LATENCY);
    const body = (await request.json().catch(() => ({}))) as { enabled?: boolean };
    const c = connectors.find((x) => x.id === params.connectorId);
    if (!c) return new HttpResponse(null, { status: 404 });
    c.enabled = body.enabled ?? c.enabled;
    return HttpResponse.json(c);
  }),

  http.post('*/connectors/:connectorId/test', async () => {
    await delay(LATENCY);
    return HttpResponse.json({ ok: true, message: 'Connection succeeded', latencyMs: 42 });
  }),

  http.post('*/connectors/:connectorId/sync', async ({ params }) => {
    await delay(LATENCY);
    return HttpResponse.json({ jobId: `job-${Date.now().toString(36)}`, serviceId: params.connectorId });
  }),

  http.post('*/sync', async () => {
    await delay(LATENCY);
    return HttpResponse.json({ jobId: `job-${Date.now().toString(36)}`, serviceId: null });
  }),

  http.post('*/connectors', async ({ request }) => {
    await delay(LATENCY);
    const body = (await request.json().catch(() => ({}))) as Record<string, unknown>;
    const created = {
      id: `svc-${Date.now().toString(36)}`,
      name: String(body.name ?? 'new-connector'),
      category: body.category,
      type: String(body.type ?? 'custom'),
      enabled: true,
      status: 'online',
      url: String(body.url ?? ''),
      verifyTls: Boolean(body.verifyTls),
      lastSyncAt: null,
    };
    connectors.push(created as (typeof connectors)[number]);
    return HttpResponse.json(created, { status: 201 });
  }),

  // Destructive: honour step-up — reject when the toggle is on and the elevation
  // token header is absent, otherwise drop the connector.
  http.delete('*/connectors/:connectorId', async ({ params, request }) => {
    await delay(LATENCY);
    if (authConfig.stepUpForDestructive && !request.headers.get('X-Elevation-Token')) {
      return HttpResponse.json(
        { code: 'elevation-required', message: 'Step-up required' },
        { status: 403 },
      );
    }
    const idx = connectors.findIndex((x) => x.id === params.connectorId);
    if (idx >= 0) connectors.splice(idx, 1);
    return new HttpResponse(null, { status: 204 });
  }),

  http.get('*/me', async () => {
    await delay(LATENCY);
    return HttpResponse.json(user);
  }),

  http.get('*/dashboard/overview', async () => {
    await delay(LATENCY);
    return HttpResponse.json(dashboardOverview());
  }),

  http.get('*/connectors/schema', async () => {
    await delay(LATENCY);
    return HttpResponse.json(connectorSchemas);
  }),

  http.get('*/connectors/:connectorId/removal-impact', async ({ params }) => {
    await delay(LATENCY);
    return HttpResponse.json(removalImpact(params.connectorId as string));
  }),

  http.get('*/connectors', async () => {
    await delay(LATENCY);
    return HttpResponse.json(connectors);
  }),

  // Step-up: mint a short-lived elevation token for a single destructive action.
  http.post('*/auth/elevate', async () => {
    await delay(LATENCY);
    return HttpResponse.json({
      token: `elev-${Math.random().toString(36).slice(2)}`,
      expiresAt: new Date(Date.now() + 2 * 60_000).toISOString(),
    });
  }),

  http.get('*/changes', async () => {
    await delay(LATENCY);
    return HttpResponse.json(changePage());
  }),

  http.get('*/changes/:changeId', async ({ params }) => {
    await delay(LATENCY);
    const detail = changeDetail(params.changeId as string);
    if (!detail) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(detail);
  }),

  http.get('*/alerts', async () => {
    await delay(LATENCY);
    return HttpResponse.json(alertPage());
  }),

  http.get('*/docs/tree', async () => {
    await delay(LATENCY);
    return HttpResponse.json(docTree);
  }),

  http.get('*/docs/:docId/versions/:rev', async ({ params }) => {
    await delay(LATENCY);
    const byRev = docVersionContent[params.docId as string];
    const v = byRev?.[Number(params.rev)];
    if (!v) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(v);
  }),

  http.get('*/docs/:docId/versions', async ({ params }) => {
    await delay(LATENCY);
    return HttpResponse.json(docVersionMeta[params.docId as string] ?? []);
  }),

  http.get('*/docs/:docId', async ({ params }) => {
    await delay(LATENCY);
    const doc = docs[params.docId as string];
    if (!doc) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(doc);
  }),
];
