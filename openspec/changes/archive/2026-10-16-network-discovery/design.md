# Design

## Context

See proposal.md for motivation. Current state that shapes the approach:

- Outbound connections go through `GuardedDialer` in `internal/connector/connector.go`. Its `Control` hook rejects loopback, link-local, unspecified and multicast addresses (`IsDangerousIP`) and allows private ranges, so a scan of RFC 1918 space is not blocked today and nothing needs to be relaxed. There is no per-range mechanism.
- `RequireElevation(jwtSvc, recorder, action)` in `internal/auth/middleware.go` enforces a single-use `X-Elevation-Token` only when the instance's step-up setting is on (except `mfa.manage`). The web side handles it with `useStepUpMutation`.
- Connector types register a `TypeSchema` plus factory in their package `init` (`internal/connector/registry.go`). The schema already carries type-level metadata (`NoURL`, `EndpointConfigKeys`, `ConfigCheck`).
- Long-running work reports over WebSocket: sync returns 202 and emits `sync.progress` / `sync.complete`; `Hub.BroadcastToUser` targets one user. There is no job table and no polling endpoint.
- Rate limiting (`api/middleware/ratelimit.go`) is a per-key token bucket applied before the handler, so it counts rejected requests too.
- The default compose file runs the backend on a bridge network, and nothing in the backend reads its interfaces.
- `ConnectorForm` keeps type and values in local state and accepts only `onCreated` / `onCancel`. Onboarding's `SyncStep` handles one connector.

## Goals / Non-Goals

**Goals:**
- No new tables, dependencies or background infrastructure.
- A probe can never leave the submitted range, even if range validation had a bug.
- Adding discovery for a connector type is a declaration in that connector's package, not an edit to the scanner.

**Non-Goals:**
- A general port scanner or host inventory.
- Changing what the global guard blocks or how existing connectors dial.
- Multi-instance coordination; limits and results are per process, like the existing rate limiter.

## Decisions

### D1. Range-scoped guarded dialer instead of relaxing the guard
Add `GuardedDialerForRange(timeout, *net.IPNet)` beside `GuardedDialer`. Its `Control` applies the same `IsDangerousIP` check (honouring `AllowLoopbackForTest`) and then returns `BlockedAddressError` for any address outside the range. The scanner's TCP connects and its HTTP transport both dial through it; redirects are refused with `httpx.NoRedirect`; the scanner dials IP literals only.
The issue asked to "relax the SSRF guard only for the submitted range". Exploration showed the guard already allows private ranges, so there is nothing to relax. The user chose to still build a range mechanism, as an allowlist: it binds scan traffic to the range now and is the hook to use if the global guard is ever tightened.
Alternatives: no new dialer, rely on range validation only (rejected by the user); make the global guard block RFC 1918 and exempt ranges (changes every existing connector, out of scope); let a range bypass the loopback/link-local block (rejected, contradicts the accepted-range rule).

### D2. Range validation
`discovery.ParseRange` uses `netip.ParsePrefix`, masks host bits, and requires IPv4, `Bits() >= 24`, and containment in one of the three RFC 1918 prefixes. Hosts are enumerated from the prefix, dropping network and broadcast for prefixes of 30 or less. Validation and D1 are independent layers.

### D3. Discovery hints live on the connector registry
Add an optional `Discovery *DiscoveryHint` to `TypeSchema` (`json:"-"`):

```go
type DiscoveryHint struct {
    Probes      []DiscoveryProbe
    URLTemplate string // "{scheme}://{host}:{port}/api2/json"
}
type DiscoveryProbe struct {
    Port   int
    Scheme string // "http" or "https"
    Path   string
    Match  func(DiscoveryResponse) bool // status, headers, body (capped), TLS leaf
}
```

Each of the fourteen types sets it in its existing `Register` call. `connector.DiscoveryHints()` returns them; the scanner derives the port list from that, so there is no second list to keep in step. Matchers are pure functions over a captured response and are unit-tested per connector with recorded fixtures.
Alternative: one table in `internal/discovery`. Rejected: it separates a product's identity check from the package that knows the product.

Starting points for the matchers, to be confirmed against each product while writing its fixture: Docker `GET /version` (JSON with `ApiVersion`), Traefik `GET /api/version`, Home Assistant `GET /manifest.json`, Nginx Proxy Manager `GET /api/` on 81, Caddy `GET /config/`, UniFi `GET /status`, Proxmox VE and PBS by `Server` header on `/`, and login-page markers for pfSense, OPNsense, TrueNAS, Pi-hole (`/admin/`), AdGuard Home and Portainer. A matcher that cannot be made specific is left out and the type is noted in the docs as not discoverable.

### D4. Scanner
`internal/discovery/scanner.go`. One pass per host: TCP connect to each listed port through the scoped dialer (500 ms timeout); for each open port run the probes declared for it (2 s per request, body read capped at 64 KB, TLS verification off, no credentials, no redirects). A semaphore bounds concurrent connections at 128. The scan context carries a 60 second deadline and the cancel function. Results flow through a callback (`onProgress`, `onCandidate`). Worst case is 254 hosts × 12 ports, roughly 15 to 20 seconds.
`InsecureSkipVerify` is confined to the scanner's own transport, with a comment explaining that only public response fields are read.

### D5. Scan manager: single scan, in-memory result
`discovery.Manager` holds, under one mutex: the current scan (id, range, starter, state, progress, answered count, candidates, started/ended times, partial flag, cancel func) and a slice of start times for the rolling-hour limit. `Start` checks, in order, a running scan (conflict) and the six-per-hour window (rate limited, with retry time), then records the start and launches the goroutine. A result is dropped on read once 15 minutes past its end, and replaced when the next scan starts.
The hourly limit lives here and not in `middleware.RateLimit` because the middleware runs before validation and would count rejected requests.
Alternatives: a scans table (rejected by the user, results are ephemeral); synchronous request (no progress, depends on proxy timeouts); polling (pattern the app does not use).

### D6. API
Mounted in the instance-admin group of `mountAPIRoutes`, in a new `routes_discovery.go`, handlers in `internal/api/discovery`:

- `GET /api/discovery/suggestions` → `{suggestions: [{cidr, source: "client" | "server"}]}`.
- `POST /api/discovery/scan` with `{cidr}`, behind `RequireElevation(cfg.JWT, cfg.Store, "discovery.scan")` → 202 `{scan}`; 400 field error; 409 `scan_in_progress` with the running scan; 429 with `Retry-After`.
- `GET /api/discovery/scan` → the current or most recent unexpired scan, 404 when none.
- `DELETE /api/discovery/scan` → cancels the running scan; 404 when none is running.

Because only one scan exists at a time, the scan is a singleton resource and its id appears in payloads only, to let the client discard WebSocket events from an earlier scan. Any instance admin may read or cancel it; live events go to the admin who started it.

### D7. WebSocket events
`discovery.progress` `{scan_id, done, total, answered}` (throttled to a few per second), `discovery.candidate` `{scan_id, candidate}`, `discovery.complete` `{scan_id, state, partial}`, sent with `Hub.BroadcastToUser`. The web panel reads the scan once on mount (covers reload and events missed before the socket was ready) and then applies events.

### D8. Already-connected matching
After a candidate is confirmed, the manager compares it with stored connectors of the same type by parsing each connector's `url` to host and effective port. Host names in connector URLs are not resolved; a connector configured by name is not matched and the candidate stays connectable (the form's own duplicate handling applies). Accepted trade-off to avoid DNS lookups driven by a scan.

### D9. Suggestions
Client range from `httputil.ClientIP` (already honours the trusted-proxy configuration); server ranges from `net.InterfaceAddrs`. Both filtered to RFC 1918 and narrowed to the containing /24, de-duplicated, client first. Only the client range is prefilled: under bridge networking the server's range is the container network, which is still a valid thing to scan but rarely what the admin wants.

### D10. Audit
`store.RecordAuditFromContext` for `discovery.scan.start` and `.reject` in the handler; `RecordAuditAs` with the starter for `.complete` / `.cancel` from the scan goroutine. Target type `discovery_scan`, target id the scan id. Detail holds the range and counts only.

### D11. Web
- `ConnectorForm` gains `initialType` and `initialValues`, read once into its existing state.
- `features/discovery/DiscoveryPanel` (range input with suggestion chips, start through `useStepUpMutation`, progress, result list with checkboxes and the already-connected badge, cancel, empty state with the answered count) and `ConnectQueue` (renders `ConnectorForm` per selected candidate with save / skip / stop and returns the created connectors). Discovery state lives in a small store fed by the WebSocket provider, like `store/live.ts`.
- `OnboardingPage`: the connect branch shows the panel beside the manual form for instance admins; the created-connector state becomes a list and `SyncStep` starts and shows one sync per entry. The manual path passes a one-element list.
- `AddConnectorPage`: a "Scan network" action that shows the same panel; on queue end it navigates to `/services`.

## Risks / Trade-offs

- [An admin scans a network they do not own, e.g. a shared VLAN] → admin-only, optional elevation, one /24 at a time, six per hour, audited with actor and range; the docs state that the admin is responsible for the range.
- [Probing triggers an IDS or a login-attempt counter] → one unauthenticated GET per open port, no credentials, no retries; documented.
- [False positives or negatives in matchers] → unconfirmed ports are dropped; matchers are fixture-tested; the manual form is always available.
- [Limits and results are per process] → same constraint as the existing in-memory rate limiter; documented, not solved here.
- [The admin's address is rewritten by a proxy or Docker's userland proxy] → the client suggestion is then missing or a bridge address; the field stays editable and nothing is prefilled when no private range is found.
- [WebSocket not connected when the scan starts] → the panel reads the scan on mount and when the socket reconnects.

## Migration Plan

Additive. No data migration. Rollback is reverting the change; nothing persists.

## Open Questions

- Exact matcher per product (D3) is settled while writing each fixture; a type without a reliable matcher is dropped from discovery and listed in the docs, which narrows the "Discoverable products" requirement and must then be reflected in the spec before archive.
