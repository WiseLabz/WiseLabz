# Tasks

Groups 1 to 6 and 10 are backend and docs work; groups 7 to 9 are web work and start from the contract commit of group 1.

## 1. Contract (first backend commit)

- [x] 1.1 Update `docs/openapi.yaml` for design D12 and D11 (request `ranges` / `includeInventory` / `askNetwork` / deprecated `cidr`; `Scan` and `Candidate` additions; `tlsTargets`; `addOns`; suggestion source `inventory` and `knownHosts`), update `docs/WS_CONTRACT.md` for the new `discovery.candidate` fields, regenerate the web client (`bun run gen:api`), and keep the web compiling with minimal type fixes in `mocks/curated.ts`; verify `TestOpenAPIMatchesRouter`, web typecheck and the existing discovery web tests pass.

## 2. Address classes, ranges and the target set

- [x] 2.1 Add `discovery.Allowed` (design D2) and `discovery.ParseRanges` (D3), keeping `ParseRange` only if still used; verify table tests for every scenario under "Accepted range", /22 to /32 host counts, overlaps counted once, 16 and 17 entries, the 1,024 limit at exactly 1,024 and 1,025 addresses, `no_targets`, field names `ranges[i]` and the v1 `cidr` form.
- [x] 2.2 Add `discovery.TargetSet` with `Contains` and budget-checked `Admit` (D1, D5) and replace `connector.GuardedDialerForRange` with `GuardedDialerForSet`; port `guarded_dialer_range_test.go` and verify: in set, out of set, a prefix member, a single-address member, admission after creation, loopback / link-local / multicast inside the set still blocked, nil set, and that `AllowLoopbackForTest` never relaxes the set check.

## 3. Sources

- [x] 3.1 Add `store.ListIndexedAddresses` (D4) for SQLite and Postgres; verify store tests on both engines with rows from two connector types, an empty `ip`, and a deleted connector.
- [x] 3.2 Add `discovery.InventorySource` (D4); verify tests for the Docker/Portainer container exclusion with a Proxmox `container` kept, public and link-local addresses dropped, fd00::/8 kept and other IPv6 dropped, unparseable rows skipped, de-duplication, removal of addresses inside typed ranges, and deterministic order.
- [x] 3.3 Add `discovery.PassiveSource` (D6) with injectable interface listing, packet sender and clock; verify tests for the query bytes (mDNS legacy-unicast PTR question, SSDP M-SEARCH), source-address-only admission, a payload naming another address ignored, non-allowed and link-local sources ignored, the 3 s window, the read size cap, and distinct `passiveAnswered` counting; plus one loopback-free integration test that sends and receives on a local UDP socket pair without multicast.

## 4. Scanner and manager

- [x] 4.1 Change `Runner.Run` to take `Targets` (D7): static addresses, the target set, and a late-address channel; verify scanner tests for late addresses probed after static ones, a late address rejected by the set never dialled, the closed channel ending the scan, cancellation, and every v1 scanner test still passing.
- [x] 4.2 Add the `OnTLSTarget` callback (D9) reported once per address and port for any HTTPS probe with a TLS leaf; verify tests for a matched product, an unmatched HTTPS server, an HTTP-only port (none) and de-duplication across probes.
- [x] 4.3 Extend the manager: `Start` takes the start request (ranges, inventory, passive), admits sources in D5 order, computes the deadline (D8) with `TimeoutPer256` / `MaxTimeout`, tracks `sources`, `skipped`, `passiveAnswered`, `tlsTargets` and candidate `source`; verify manager tests for every "Address budget" scenario, the deadline scenarios (256, 257, 600, 1,024 addresses; with and without passive), total growing on admission, and every v1 manager test still passing.
- [x] 4.4 Mark TLS targets monitored by existing TLS Probes on every read (D9), exporting `tlsprobe.ParseTargets`; verify tests for an IP literal match, a different port, a host name target (not matched), an imported target, and a stale marker cleared.

## 5. Add-ons

- [x] 5.1 Add `DiscoveryAddOn` to `DiscoveryHint` and declare the DNS Resolver add-on in `pfsense` and `opnsense` (D10); candidates carry `addOns`; mark an add-on's `connectorId` from existing `dnsresolver` connectors; verify registry tests (only those two types declare add-ons), the "OPNsense found" scenario, and the already-connected add-on.

## 6. API and audit

- [x] 6.1 Update the start handler for the D12 request (including the v1 `cidr` form and field names), the known-hosts read, and the 500 path; verify handler tests with `newTestApp` for each new "Accepted range" scenario, "Nothing to scan", "Ranges over budget", "Earlier request form", and that v1 handler tests pass.
- [x] 6.2 Update suggestions (D11) with inventory /24s and `knownHosts`, and 100.64.0.0/10 client/server addresses; verify the three "Range suggestions" scenarios and de-duplication against client and server suggestions.
- [x] 6.3 Update audit details (D14) and `docs/AUDIT.md`; verify handler and manager tests assert the new start, complete, cancel and reject details, and a test that no entry contains any range, known-host, passive or candidate host address (ranges excepted).

## 7. Web: scan form and results

- [ ] 7.1 Rework `DiscoveryPanel` for range chips, the two toggles with the known-host count, the budget meter, and the start body (D13); verify component tests for adding/removing chips, inventory suggestion chips, per-index field errors, Start disabled with no range and known hosts off, the "Budget shown" scenario, and step-up still applied.
- [ ] 7.2 Show candidate source, skipped counts and the network-answer count with the host-networking hint; update `store/discovery.ts` for the new fields; verify store tests and component tests for each.
- [ ] 7.3 Add the certificate row (D13): unmonitored targets selectable, monitored ones listed; verify tests for the "Certificates offered" and "Certificate already monitored" scenarios.

## 8. Web: connect queue

- [ ] 8.1 Extract `ConnectorEditForm` from `ConnectorEditPage` with `overrides`, `onSaved`, `onCancel` (D13); verify `ConnectorEditPage.test.tsx` passes unchanged and new tests cover `overrides` applied once and save/cancel callbacks.
- [ ] 8.2 Add queue item kinds `addOn` and `tls` to `ConnectQueue` / `DiscoveryFlow`: add-on after its candidate (saved or skipped), TLS edit of an existing probe with the 100-target cap and the "not added" list, a TLS Probe select when several exist, a create form when none exists; mark them connected/monitored after save; verify tests for "pfSense with DNS Resolver", "Certificates appended to an existing probe", the create path, the select, skip and stop, and that the v1 queue tests pass.
- [ ] 8.3 Onboarding sync step includes an edited TLS Probe; verify `OnboardingPage.test.tsx` covers it and the v1 cases pass.

## 9. Web: strings and mocks

- [ ] 9.1 Add en and pt-BR strings for every new element and update `mocks/curated.ts` (scan with several sources, TLS targets, an OPNsense candidate with an add-on, suggestions with `knownHosts`); verify the i18n key parity test, web lint and typecheck.

## 10. Documentation

- [x] 10.1 Update `docs/NETWORK_DISCOVERY.md`: ranges and the budget, known hosts and the container exclusion, asking the network (what is sent, source-address-only, needs host networking, the multicast exception to the guarded dialer), mesh ranges needing the server on the mesh, the deadline formula, certificate monitoring, the DNS Resolver add-on, and the audit fields; add an `## Unreleased` entry to `CHANGELOG.md`; verify the docs link check and that every limit stated matches the spec.

## 11. Integration

- [ ] 11.1 Run backend tests, web tests, lint and typecheck with the low-memory settings, `openspec validate network-discovery-v2 --strict`, then `graphify update .`; verify all pass in CI on the PR.
- [ ] 11.2 Manually scan two ranges plus known hosts with ask-the-network on, from a host-networked instance, connect a pfSense or OPNsense candidate with its DNS Resolver add-on, append certificates to an existing TLS Probe, and trigger the budget overflow; verify the behaviour and the audit entries match the spec scenarios.
