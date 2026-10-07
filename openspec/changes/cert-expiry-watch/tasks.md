# Tasks

## 1. Days-left operators and NPM normalization

- [x] 1.1 Add `days_left_lt` and `days_left_gt` to the compliance matcher with an injectable clock; verify unit tests for inside the window, expired (negative), the two-condition band at 1 day 12 hours, missing attribute and non-timestamp attribute.
- [x] 1.2 Validate the operators on rule save (whole-number value) and include them wherever operators are listed (rule API validation, `GET /api/compliance/schema` if it lists operators, OpenAPI enum); verify handler tests for a valid rule and the non-integer rejection, and the OpenAPI contract test.
- [x] 1.3 Add `not_after` to NPM certificate entities and the NPM attribute catalog, keeping `expires_on`; verify `tables_test.go` for the normalized value and for an unparseable expiry, and that the NPM snapshot stability test still passes.

## 2. Snapshot inputs for connectors

- [ ] 2.1 Add the optional `SnapshotDependent` interface and the `RelatedSnapshots` / `PreviousSnapshot` config helpers in `internal/connector`; verify unit tests for the helpers with present and absent inputs.
- [ ] 2.2 Supply related and previous snapshots before `Fetch` in `internal/sync/run.go` for connectors that implement the interface, tolerating a missing connector or snapshot; verify a sync test with a fake dependent connector that receives both, and one where the related connector was deleted.

## 3. TLS probe connector

- [ ] 3.1 Create `internal/connector/tlsprobe` with config schema (`targets`, `import_connector_id`, `import_port`), `Validate` (target syntax, 100 limit) and registration under category `monitoring`; verify validation tests for a malformed target and 101 targets, and the registry test.
- [ ] 3.2 Implement `Fetch`: guarded TLS dial per target with SNI, 5 second timeout and bounded concurrency, producing `certificate` entities with the attributes in design D5 and a registered attribute catalog; verify tests against local TLS servers for a valid, a self-signed and a private-CA certificate, and a blocked loopback address without the test allowance.
- [ ] 3.3 Implement unreachable handling: stable error classes, carry-forward of certificate attributes from the previous snapshot, entity without `not_after` when never reached, and the health rule (offline only when all of at least one target fail, online with zero targets); verify a test for each spec scenario under "Unreachable targets" and that two consecutive fetches of an unchanged target produce identical snapshots.
- [ ] 3.4 Implement Traefik host extraction and import: literal `Host` matchers from `tls = true` routers, de-duplication against listed targets, alphabetical truncation at the limit with the left-out count in the sync result; verify table tests for multi-argument, `||`, `&&`, negated and `HostRegexp` rules, and tests for the duplicate, limit, removed-router and never-synced scenarios.
- [ ] 3.5 Validate `import_connector_id` on connector create and update (exists, Traefik type, caller can view); verify handler tests for 403 without access and the wrong-type field error.
- [ ] 3.6 Add the connector's documentation page (what is probed, that certificates are read without trust validation, imported host visibility, limits) where other connectors are documented, and update OpenAPI if connector types are enumerated; verify the docs build or link check and the OpenAPI contract test.

## 4. Certificate expiry pack

- [ ] 4.1 Add `compliance/packs/certificate-expiry.yaml` with the six banded rules; verify the pack loads and a test asserts that for every integer days-left from -5 to 40 at most one band matches, with the expected severity.
- [ ] 4.2 Add `installed` to the pack listing response and OpenAPI, then regenerate the web client; verify handler tests before install, after install and after one pack rule is deleted.
- [ ] 4.3 Add a quality checker test over NPM and TLS probe snapshots with a fixed clock: 21 days gives one info finding, moving to 7 days resolves it and opens a warning with a notification, expired gives one critical, renewal resolves, an unreachable target with a last-known date still matches.

## 5. Certificates listing

- [ ] 5.1 Implement `GET /api/certificates` (limit default 10, max 100) over viewable connectors' latest snapshots with days left and the unreachable flag, register the route, add OpenAPI and regenerate the web client; verify handler tests for ordering with expired first, a hidden connector, a restricted API key, and no rules installed.

## 6. Web

- [ ] 6.1 Support the TLS probe type in the connector form: targets list input, optional Traefik connector picker limited to viewable Traefik connectors, import port; verify component tests for save and for server field errors.
- [ ] 6.2 Offer the pack install after creating a TLS probe connector and when enabling the widget, only when `installed` is false and the user may install packs, remembering a dismissal; verify component tests for shown, already installed, and no permission.
- [ ] 6.3 Add the two operators to the compliance rule editor with a whole-number input; verify `RulesPage.test.tsx` covers creating a rule with `days_left_lt`.
- [ ] 6.4 Add the `ExpiringCertificatesWidget` (default disabled) with expired / 7 days / 30 days / later states, an unreachable marker, entity links and the empty state, and register it in the dashboard store; verify a widget test modelled on `FleetUptimeWidget.test.tsx` for populated, default-hidden and empty.
- [ ] 6.5 Add en and pt-BR strings for the connector, operators, pack prompt and widget; verify the i18n key parity test and web lint.

## 7. Integration

- [ ] 7.1 Run backend tests, web tests, lint and typecheck with the low-memory settings, then `graphify update .`; verify all pass in CI on the PR.
- [ ] 7.2 Manually create a TLS probe connector against real lab hosts with a Traefik import, install the pack and enable the widget; verify entities, an expiry finding for a short-lived test certificate, and the widget match the spec scenarios.
