# Design

## Context

See proposal.md for motivation. Current state that shapes the approach:

- A connector's `Fetch(ctx, config)` receives only its own config; no connector reads the store or another connector. The sync layer calls it in `internal/sync/run.go` and already passes hints through config (`"fields"`, read by `connector.RequestedFields`).
- Outbound connections go through `GuardedDialer` in `internal/connector/connector.go`, which blocks loopback, link-local, unspecified and multicast addresses and allows private ranges.
- NPM emits `certificate` entities with `expires_on` as NPM reports it (`npm/tables.go`, `buildCertificateTable`). Traefik emits `router` entities with `rule`, `tls` and `entryPoints` but has no certificate expiry.
- Compliance rules (`internal/compliance/engine.go`) target one connector type and entity kind, carry one severity, and AND their conditions through `matches`. Rule packs are embedded YAML (`compliance/packs/`), installed on request by `InstallPack`, which creates rules not already present by name. The quality checker evaluates rules after each sync and on its cron, resolves findings for entities that stop matching, and re-notifies on severity changes.
- Quality findings are keyed by connector, which is why the probe is a connector and not a standalone job.
- Dashboard widgets live in `web/src/components/dashboard/widgets.tsx`; `FleetUptimeWidget` is the precedent for a default-disabled widget backed by one endpoint (`GET /api/connectors/uptime`).

## Goals / Non-Goals

**Goals:**
- Reuse sync scheduling, grants, entity pages, findings and notifications; no new tables.
- Keep connectors store-free while letting one use another's data.
- Expiry findings stay correct between syncs because rules compute days left at evaluation time.

**Non-Goals:**
- Trust validation, OCSP, chain analysis.
- A general cross-connector query facility beyond what the import needs.
- Changing existing rule packs or auto-installing rules.

## Decisions

### D1. TLS probe is a connector type
New package `internal/connector/tlsprobe`, category `monitoring`. Config: `targets` (text list of `host:port`), `import_connector_id` (optional), `import_port` (default 443). `Fetch` dials each target with `tls.Client` over a connection from `GuardedDialer`, `InsecureSkipVerify: true`, `ServerName` = host, 5 second deadline, bounded concurrency (8). It reads the leaf certificate from the connection state.
Alternative: a scheduler job with its own table and API. Rejected by the user; it would need new permission, findings and UI plumbing.

`InsecureSkipVerify` is deliberate and confined to this package: the probe reads public certificate fields and sends nothing over the connection. A code comment and the connector docs say so, so the linter exclusion is explained.

### D2. Sync layer supplies related snapshots through an optional interface
Add to `internal/connector`:

```go
// SnapshotDependent connectors need stored snapshots to fetch.
type SnapshotDependent interface {
    SnapshotInputs(config map[string]any) (relatedConnectorIDs []string, wantPrevious bool)
}
```

Before `conn.Fetch` in `sync/run.go`, when the connector implements it, the engine loads the latest stored snapshot of each related connector and, if asked, the connector's own previous snapshot, and places them in the config map under reserved keys read through helpers (`connector.RelatedSnapshots(config)`, `connector.PreviousSnapshot(config)`), mirroring `RequestedFields`. A missing connector or snapshot is simply absent. The probe uses the related snapshot for the Traefik import and the previous snapshot to carry forward attributes of unreachable targets.
Alternatives: give the probe the Traefik URL and credentials (duplicates the client and copies secrets); hardcode a type switch in the sync engine (works, but puts probe logic in `sync`). The interface keeps both the engine and the connector ignorant of each other's internals.

### D3. Host extraction from Traefik rules
A small function in `tlsprobe` extracts every literal inside ``Host(`...`)`` and `Host("...")` matchers, including multiple arguments and matchers joined with `||` or `&&`, from routers with `tls = true`. `HostRegexp`, `HostSNI` and anything else contribute nothing. It is a regular expression over the rule string, not a rule parser; a negated matcher (`!Host(...)`) is excluded by checking the preceding character. Hosts are lower-cased, de-duplicated and sorted.

### D4. Dial imported hosts by name on one port
Imported hosts are resolved through DNS and dialed on `import_port`. This tests what clients see and needs no entry-point parsing. A name that does not resolve from the WiseLabz host becomes an unreachable entity, which is visible and actionable.
Alternative: dial the Traefik host with SNI on the router's entry-point port. Rejected by the user for v1; listed as a non-goal.

### D5. Entity shape and stability
`Kind: "certificate"`, `ExternalID` and `Name` = `host:port`. Attributes: `host`, `port`, `not_after`, `not_before` (RFC 3339 UTC, whole seconds), `issuer`, `subject`, `dns_names` (sorted), `self_signed`, `reachable`, `error` (only when unreachable), `source` (`manual` or `imported`). No "checked at" timestamp and no days-left attribute: either would change every sync and create snapshot diff and drift noise. Attributes are registered with `RegisterAttributeCatalog`.

For an unreachable target the connector copies the certificate attributes from the previous snapshot's entity with the same `ExternalID`, then sets `reachable = false` and `error`. Error text is reduced to a stable class (`dns`, `refused`, `timeout`, `handshake`, `blocked`) plus a short message so it does not churn.

### D6. Health
The health check reuses the last fetch outcome rule from the spec: offline only when there is at least one target and all are unreachable. `Validate` checks target syntax and the 100 limit without dialing.

### D7. `not_after` on NPM
`buildCertificateTable` parses `expires_on` with the layouts NPM emits and writes `not_after` in the same format as the probe. `expires_on` stays so existing rules and snapshots are unaffected. The attribute is added to NPM's catalog.

### D8. Two days-left operators
Add `days_left_lt` and `days_left_gt` to `matches`: parse the attribute as RFC 3339, compute `floor((t - now) / 24h)`, compare with the integer value. `now` comes from a clock on the evaluator so tests are deterministic. Absent or unparseable attribute means no match, including for the `gt` form.
Two operators let a rule express a closed band with the existing AND of conditions, which gives one finding per certificate that changes severity as it ages.
Alternatives: one `lt` operator with overlapping rules (three findings for an expired certificate); a derived numeric attribute emitted by connectors (goes stale between syncs and churns snapshots); a built-in check like config drift (rejected by the user in favour of editable rules).

### D9. Pack contents
New `compliance/packs/certificate-expiry.yaml`, six rules (two connector types times three bands):

| Band | Conditions on `not_after` | Severity |
|---|---|---|
| 8 to 30 days | `days_left_lt 31`, `days_left_gt 7` | info |
| 2 to 7 days | `days_left_lt 8`, `days_left_gt 1` | warning |
| 1 day or fewer | `days_left_lt 2` | critical |

A test asserts that for every integer from -5 to 40 exactly zero or one band matches. Escalation between bands relies on existing behaviour: the lower rule's finding resolves when it stops matching and the higher rule opens a new one, which notifies.

### D10. Pack installed state
The pack listing gains an `installed` boolean, true when every rule name of the pack exists, using the same name match `InstallPack` uses. The web prompt reads it after creating a TLS probe connector and when the widget is enabled, and shows only to users allowed to call `InstallPack`. A dismissed prompt is remembered per browser.

### D11. Certificates listing
`GET /api/certificates?limit=` (default 10, max 100). The handler takes the connectors the caller can view (existing grant and API-key filtering helpers), keeps those whose type declares a `certificate` kind with `not_after` in its attribute catalog, reads each one's latest snapshot, collects certificate entities with a parseable `not_after`, sorts and truncates. Days left uses the same function as D8. No new table; the number of such connectors is small and snapshots are already loaded this way by the quality checker.
Alternative: query findings. Rejected: the widget must work without the pack.

### D12. Reference validation on save
Connector create and update for type `tlsprobe` verify that `import_connector_id`, when set, names a Traefik connector the caller can view. This lives beside existing connector config validation in `internal/api/connectors`, not in the connector package, because it needs grants.

## Risks / Trade-offs

- [The probe is an outbound scanner under user control] → Explicit target list, 100 target cap, 5 second timeout, guarded dialer, connector creation already restricted, one handshake per target per sync.
- [Imported host names become visible to viewers of the probe connector] → Saving the reference requires view access to the Traefik connector; documented in the connector docs.
- [Regex extraction misreads an unusual rule] → Only literal `Host` matchers are taken; tests cover multi-argument, `||`, `&&`, negation and `HostRegexp`. A miss means a host is not probed, never a wrong write.
- [Last-known `not_after` on an unreachable target may be outdated after a renewal] → The entity is marked unreachable in the listing and widget; the finding errs toward alerting.
- [Rules are evaluated after sync and on the quality cron, so a band change can lag by one interval] → Acceptable at day granularity.
- [Six pack rules for two sources] → Rules are per connector type today; a wildcard type would touch the engine, editor and related clauses and is out of scope.

## Migration Plan

No schema migration. New connector type, operators, pack and endpoint are additive. Rollback is a binary rollback; rules using the new operators would then be rejected by an older evaluator as an unknown operator and match nothing, and TLS probe connectors would fail to load as an unknown type, both of which the release note states.
