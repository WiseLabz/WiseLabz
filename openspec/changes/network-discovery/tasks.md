# Tasks

## 1. Range and scoped dialer

- [x] 1.1 Add `GuardedDialerForRange(timeout, *net.IPNet)` beside `GuardedDialer` in `internal/connector/connector.go`, reusing `IsDangerousIP`, `BlockedAddressError` and the loopback test allowance; verify unit tests for an in-range address, an out-of-range private address, and loopback and link-local addresses inside a (hypothetical) allowed range.
- [x] 1.2 Create `internal/discovery` with `ParseRange` and host enumeration (design D2); verify a table test covering every scenario under "Accepted range" plus /30, /31 and /32 host counts.

## 2. Discovery hints

- [ ] 2.1 Add `DiscoveryHint`, `DiscoveryProbe`, `DiscoveryResponse` and `DiscoveryHints()` to `internal/connector/registry.go`, with URL template rendering; verify registry tests for hint listing, template rendering and that a type without a hint is absent.
- [ ] 2.2 Declare hints with fixture-tested matchers for the distinct-port types: Proxmox VE, PBS, Home Assistant, Portainer, UniFi, AdGuard Home, Traefik, Docker, Caddy, Nginx Proxy Manager; verify each package has a test with a matching fixture and a non-matching response on the same port.
- [ ] 2.3 Declare hints with fixture-tested matchers for pfSense, OPNsense, TrueNAS and Pi-hole on 80/443; verify each rejects the other three products' fixtures and a generic web server page.
- [ ] 2.4 Add a registry-level test asserting the set of discoverable types and the derived port list equal the spec's "Discoverable products" list, and that hosted, custom, DNS resolver and TLS probe types have no hint; verify it passes (or update the spec if a type was dropped per the design's open question).

## 3. Scanner and manager

- [ ] 3.1 Implement the scanner (design D4): TCP connect then declared probes through the scoped dialer, concurrency bound, timeouts, body cap, no redirects, progress and candidate callbacks; verify tests against local listeners (with `AllowLoopbackForTest`) for a confirmed product, an open port that is not the product (not reported, counted as answered), two products on one host, a redirect to another address that is not followed, and cancellation stopping further dials.
- [ ] 3.2 Implement the manager (design D5): single running scan, six starts per rolling hour with an injectable clock, 60 second deadline with the partial flag, cancel, result expiry after 15 minutes and on next start; verify unit tests for each scenario under "Scan limits", "Cancelling a scan", "Results are not stored" and the partial result.
- [ ] 3.3 Mark candidates that match an existing connector of the same type by URL host and port (design D8); verify tests for a match, a different port, a different type on the same address, and a connector configured by host name (not matched).

## 4. API, events and audit

- [ ] 4.1 Add the WebSocket event constants and payloads (design D7) and document them in `docs/WS_CONTRACT.md`; verify a manager test with a fake broadcaster receives progress, candidate and complete events addressed to the starter only.
- [ ] 4.2 Implement `internal/api/discovery` handlers and `routes_discovery.go` in the instance-admin group (design D6), with `RequireElevation(..., "discovery.scan")` on start; verify handler tests with `newTestApp` for non-admin 403, `elevation_required` with step-up on, start without a token with step-up off, 400 field errors, 409 with the running scan, 429 with `Retry-After`, read while running, 404 when none, and cancel.
- [ ] 4.3 Implement range suggestions (design D9) with the interface lookup injectable; verify handler tests for both "Range suggestions" scenarios and de-duplication when client and server share a /24.
- [ ] 4.4 Write the audit entries (design D10) and add them to `docs/AUDIT.md`; verify handler and manager tests assert start, complete, cancel and reject entries with counts, and that no entry's detail contains a scanned host address.
- [ ] 4.5 Describe the four endpoints and their schemas in `docs/openapi.yaml` and regenerate the web client; verify `openapi_contract_test.go` passes and the generated discovery module compiles.

## 5. Web

- [ ] 5.1 Add `initialType` and `initialValues` to `ConnectorForm`; verify `ConnectorForm.test.tsx` covers opening with a type and URL prefilled and that existing tests pass unchanged.
- [ ] 5.2 Add the discovery store and WebSocket handling, plus mocks in `mocks/curated.ts` and `mocks/ws/MockWebSocket.ts`; verify store tests for applying events, ignoring events from another scan id, and hydrating from the read endpoint.
- [ ] 5.3 Build `DiscoveryPanel`: suggestion chips with prefill, start through `useStepUpMutation`, progress, cancel, result list with selection and the already-connected badge, the nothing-found state with the answered count, and conflict and rate-limit messages; verify component tests for each of those states.
- [ ] 5.4 Build `ConnectQueue` with save, skip, stop and stay-on-failure; verify component tests for the three "Connect queue" scenarios.
- [ ] 5.5 Add the panel to onboarding's connect step for instance admins and make `SyncStep` handle a list of connectors; verify tests for admin and non-admin connect steps, two connectors syncing, and the everything-skipped case, and that the manual single-connector path still reaches the sync step.
- [ ] 5.6 Add the "Scan network" action to `AddConnectorPage`, hidden for non-admins; verify a component test for visibility and for returning to `/services` after the queue.
- [ ] 5.7 Add en and pt-BR strings for the panel, queue and onboarding changes; verify the i18n key parity test and web lint.

## 6. Documentation

- [ ] 6.1 Add a user guide page under `docs/` covering what a scan sends, the accepted ranges and limits, elevation, what is audited, which products are discoverable, and the Docker bridge note on suggestions, and add an `## Unreleased` entry to `CHANGELOG.md`; verify the docs link check and that every limit stated matches the spec.

## 7. Integration

- [ ] 7.1 Run backend tests, web tests, lint and typecheck with the low-memory settings, `openspec validate network-discovery --strict`, then `graphify update .`; verify all pass in CI on the PR.
- [ ] 7.2 Manually scan a real /24 from onboarding and from the add-connector page, connect two candidates through the queue, cancel a scan mid-run, and trigger the conflict and hourly limit; verify the behaviour and the audit entries match the spec scenarios.
