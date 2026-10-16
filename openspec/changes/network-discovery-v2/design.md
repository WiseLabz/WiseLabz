# Design

## Context

See proposal.md for motivation. v1 (archived as `2026-10-16-network-discovery`, main spec `openspec/specs/network-discovery/spec.md`) shapes the approach:

- `discovery.ParseRange` (`internal/discovery/range.go`) accepts one IPv4 RFC 1918 prefix of /24 or narrower and returns a `Range`; `Range.Hosts()` enumerates it.
- `Scanner.Run(ctx, Range, Callbacks)` (`internal/discovery/scanner.go`) dials through `connector.GuardedDialerForRange(timeout, *net.IPNet)`, which takes exactly one prefix. Probes capture the TLS leaf (`DiscoveryResponse.TLSLeaf`) for every HTTPS probe that completed a handshake, whether or not the matcher accepted it.
- `Manager.Start(Range, Actor, started func(Scan))` (`internal/discovery/manager.go`) owns one scan, the six-per-hour window, a fixed `Timeout` of 60 s, and the in-memory result. `Scan` has `cidr`, progress counts, `partial` and `candidates`; `Candidate` has `type, name, address, port, url, urlField, connectorId`.
- `internal/api/discovery/handlers.go`: `startRequest{cidr}`, `Suggestions` (client and server /24s), audit through `RecordAuditFromContext` / `RecordAuditAs`; detail never holds a host address.
- Every connector writes its entities' addresses to `entity_index.ip` (with `kind`, keyed by `connector_id`) on each sync. Proxmox LXCs use kind `container`, the same kind as Docker and Portainer containers, so containers must be excluded by connector type, not by kind alone. The existing `SearchEntities` is grant-scoped; there is no unscoped listing.
- `connector.IsDangerousIP` blocks loopback, link-local, unspecified and multicast; 100.64.0.0/10 and fd00::/8 pass it.
- TLS Probe (`internal/connector/tlsprobe`) is one connector holding a newline-separated `targets` field (at most 100 targets together with imported hosts, `maxTargets` in `targets.go`); `parseTargets` is unexported.
- DNS Resolver (`internal/connector/dnsresolver`) reads Unbound host overrides from pfSense or OPNsense: fields `url` (appliance URL), `api_key`, `verify_tls`.
- Web: `DiscoveryPanel` sends `{cidr}` through `useStepUpMutation`, polls every 3 s while running, re-reads on `discovery.complete`; `ConnectQueue` renders `ConnectorForm` (create only) per candidate; `ConnectorEditPage` is a full page with its own load/save logic and no reusable form component.

## Goals / Non-Goals

**Goals:**
- No new tables, migrations, dependencies or background components.
- A probe can never reach an address outside the scan's exact target set, even if validation had a bug.
- Every v1 behaviour not named here is unchanged; a v1 request body (`{cidr}`) keeps working.

**Non-Goals:**
- New products or ports; IPv6 ranges; public ranges.
- A background multicast listener, or any address taken from inside an mDNS record or an SSDP `LOCATION` header.
- Detecting whether the server runs on a container bridge network.
- Multi-instance coordination (limits and results stay per process).

## Decisions

### D1. Target set and target-set dialer
`discovery.TargetSet` holds the scan's admitted prefixes (typed ranges) and single addresses (known hosts, network answers), safe for concurrent use. It implements `Contains(netip.Addr) bool`. `connector.GuardedDialerForSet(timeout, allowed interface{ Contains(netip.Addr) bool })` replaces `GuardedDialerForRange`: `Control` applies the shared `guardedAddress` check (still honouring `AllowLoopbackForTest` for loopback only) and returns `BlockedAddressError` for any address the set does not contain (a nil set contains nothing). The scanner builds one dialer per scan over the scan's `TargetSet`. The set grows only through `TargetSet.Admit(addr, source)`, which checks the address class (D2) and the remaining budget (D5) under its lock and reports whether the address was admitted. The scanner keeps dialling IP literals only, refuses redirects, and builds its HTTP transport on the same dialer.
Alternatives: one dialer per source (more paths to keep in step); keep `GuardedDialerForRange` and add a second function (two near-identical allowlists; the only caller is the scanner, so it is replaced and its tests are ported).

### D2. Address classes
One function, `discovery.Allowed(addr netip.Addr) bool`, decides which single addresses may ever be targets: IPv4 inside 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 or 100.64.0.0/10, or IPv6 inside fd00::/8; IPv4-mapped IPv6 is unmapped first; anything `IsDangerousIP` rejects is not allowed. Typed ranges use the same IPv4 blocks (no IPv6 ranges).

### D3. Range validation
`discovery.ParseRanges([]string) (Ranges, error)`:
- 1 to 16 entries when known hosts are off, 0 to 16 when on; more than 16 is a request error.
- Each entry: `netip.ParsePrefix`, masked; IPv4; `Bits() >= 22`; contained in one of the four IPv4 blocks of D2. Errors keep v1's reasons (`invalid_cidr`, `not_ipv4`, `not_private`, `too_wide`) and messages, with the private message naming 100.64.0.0/10 too, and the too-wide message saying /22.
- Overlapping or repeated ranges are allowed; addresses are de-duplicated. Hosts per range drop the network and broadcast address for prefixes of 30 or less (v1 rule).
- The de-duplicated host count across ranges must be at most 1,024, else reason `too_many_addresses` on the field `ranges`.
- Field errors name the entry: `ranges[2]`. A request carrying only the v1 field `cidr` is treated as `ranges: [cidr]` and its field errors keep the name `cidr`. A request carrying both uses `ranges`.
- When `ranges` is empty and `includeInventory` is false the request fails with reason `no_targets` on `ranges`.

### D4. Known hosts (inventory)
New store method `ListIndexedAddresses(ctx) ([]IndexedAddress, error)` returns `{connectorType, kind, ip}` for every `entity_index` row with a non-empty `ip`, joined with `connectors` for the type, across all connectors (no grant scoping: only instance admins reach discovery). `discovery.InventorySource` turns it into targets: parse `ip` with `netip.ParseAddr` (rows that do not parse are skipped), drop rows whose connector type is `docker` or `portainer` and whose kind is `container`, keep only `Allowed` addresses, de-duplicate, drop addresses already inside a typed range, and sort (IPv4 before IPv6, then by address) so truncation is deterministic. The same function gives the "known hosts" count for suggestions (D11), counted after the same filtering but without subtracting ranges. A failing store read fails the start with 500 and writes no audit entry beyond the reject (reason `internal`).

### D5. Budget and admission order
Budget is 1,024 addresses per scan (`MaxAddresses`). At start: typed range hosts are admitted first (D3 already guarantees they fit); known hosts are admitted in D4 order until the budget is full, the rest counted as `skipped.inventory`; network answers are admitted during the scan while budget remains, the rest counted as `skipped.passive`. Each admitted address carries its first source (`range`, `inventory` or `passive`); an address seen again from a later source keeps the first. `Scan.Total` is the admitted count and grows as network answers are admitted (the web treats `total` from events as authoritative, as in v1).

### D6. Ask the network (passive)
`discovery.PassiveSource` runs at scan start when `askNetwork` is true, concurrently with probing the static targets:
- For each up, multicast-capable interface with an `Allowed` IPv4 address: one mDNS legacy-unicast query (PTR `_services._dns-sd._udp.local.`, sent from an ephemeral UDP port to 224.0.0.251:5353) and one SSDP `M-SEARCH` (`ST: ssdp:all`, `MX: 2`, to 239.255.255.250:1900). For each such interface that also has an `Allowed` IPv6 address: the same to `[ff02::fb%iface]:5353` and `[ff02::c%iface]:1900`. Sockets are bound to ephemeral ports (never 5353 or 1900), so a host running avahi or minissdpd is not disturbed; the outbound interface is set with `golang.org/x/net/ipv4` / `ipv6` (already a module dependency).
- Listen for 3 seconds (`PassiveWindow`). For every datagram, take only the UDP source address; payloads are discarded unread except for a size cap of 9,000 bytes per read. A source that is not `Allowed` is ignored (this drops link-local IPv6 answers). Each new source address is passed to `TargetSet.Admit(addr, passive)` and, when admitted, fed to the scanner.
- The multicast send does not go through the guarded dialer (which blocks multicast by design); this is the one documented exception and is confined to `passive.go` with a comment. No unicast traffic is sent by the passive source.
- `Scan.passiveAnswered` counts distinct allowed source addresses, admitted or not. On the default bridge network this is normally 0; the panel and docs say so.
Alternatives: a background listener (rejected by the user); using addresses from A/AAAA records or `LOCATION` (rejected: a spoofed answer could steer probes).

### D7. Scanner input
`Runner.Run(ctx, Targets, Callbacks) Progress` where `Targets` carries the `TargetSet` (for the dialer), the static addresses in admission order, and a receive-only channel of late addresses (closed when the passive window ends; nil when passive is off). The scanner works through static addresses, then late ones as they arrive, with the same per-host routine as v1 (TCP connect on listed ports, declared probes). The semaphore of 128 connections, connect and request timeouts, body cap and no-redirect rule are unchanged.

### D8. Deadline
`Timeout = 60 s × ceil(max(static, 1) / 256)`, capped at 240 s, where `static` is the admitted count at start (ranges plus known hosts). With `askNetwork` on and fewer than 1,024 static addresses, add 60 s for the passive share, still capped at 240 s. The partial flag keeps v1 meaning. `Config.Timeout` becomes `Config.TimeoutPer256` and `Config.MaxTimeout` (defaults 60 s and 240 s) so tests can shrink them.

### D9. Certificate targets (Monitor certificates)
Whenever a probe's response has a TLS leaf, the scanner reports a `TLSTarget{address, port, source}` through a new `OnTLSTarget` callback, once per address and port, whether or not a product matched. The manager stores them on the scan (`tlsTargets`, sorted by address then port). After the scan ends (and on every read, like `connectorId`), each target is marked `monitoredBy: <connector id>` when an existing `tlsprobe` connector's `targets` list holds the same IP literal and port; host names are not resolved (v1 D8 trade-off). To read targets the same way the connector does, `tlsprobe` exports `ParseTargets(raw string) ([]Target, []error)` (wrapping today's `parseTargets`). The panel shows one grouped row after the candidates when at least one target is not monitored (D13). No new WebSocket event: targets reach the panel through the read that follows `discovery.complete` and the 3 s poll.

### D10. Add-on connectors (DNS Resolver)
`DiscoveryHint` gains `AddOns []DiscoveryAddOn{Type, URLTemplate}`, declared by `pfsense` and `opnsense` as `{Type: "dnsresolver", URLTemplate: "{scheme}://{host}:{port}"}`. A candidate built from such a hint carries `addOns: [{type, url, connectorId?}]`; `connectorId` is set when an existing `dnsresolver` connector's URL has the same address and effective port (same matching as v1's already-connected marker). The queue offers each unconnected add-on right after its candidate (D13).

### D11. Suggestions
`GET /api/discovery/suggestions` returns `{suggestions: [{cidr, source}], knownHosts: N}` where `source` is `client`, `server` or `inventory`. Client and server suggestions are unchanged except that 100.64.0.0/10 addresses also qualify. Inventory suggestions: the /24 containing each known IPv4 host (D4 filtering), ranked by host count (ties by address), top three, skipping any /24 already suggested. `knownHosts` is the D4 count. Only the client suggestion is prefilled (v1 rule).

### D12. API contract
Request `POST /api/discovery/scan`:
```json
{ "ranges": ["192.168.1.0/24", "100.100.0.0/22"], "includeInventory": true, "askNetwork": true }
```
`ranges` (array of string, 0 to 16), `includeInventory` (boolean, default false when absent so v1 clients keep v1 behaviour), `askNetwork` (boolean, default false when absent), `cidr` (string, deprecated, one-range shorthand). Responses keep v1 status codes (202, 400 field errors, 409 `scan_in_progress` with the scan, 429 with `Retry-After`).
`Scan` adds: `ranges` (array of string), `includeInventory`, `askNetwork`, `sources` `{range, inventory, passive}` (admitted addresses per source), `skipped` `{inventory, passive}`, `passiveAnswered` (integer), `tlsTargets` (array of `{address, port, source, monitoredBy?}`). `cidr` stays and holds the first range or the empty string. `Candidate` adds `source` (`range` | `inventory` | `passive`) and `addOns` (array of `{type, url, connectorId?}`, empty when none). WebSocket payloads: `discovery.candidate` carries the new candidate fields; `discovery.progress` keeps `{scan_id, done, total, answered}`; `discovery.complete` is unchanged. `docs/openapi.yaml`, the generated client and `docs/WS_CONTRACT.md` are updated in the first backend commit (task group 1) so web work builds against the final contract.

### D13. Web
- `DiscoveryPanel`: a chip input for ranges (Enter or comma adds, each chip removable, invalid entries shown inline with the server's field error by index); suggestion chips add a range (inventory suggestions labelled "known network"); two checkboxes, "Include known hosts (N)" and "Ask the network", both checked by default, the first disabled with N = 0; a budget meter `used / 1024` computed client-side from range host counts plus `min(knownHosts, remaining)`, with "passive answers fill what remains"; Start enabled when at least one range is present or known hosts is on. Each candidate row shows its source. After a scan, the panel shows `skipped` counts when non-zero and "N hosts answered the network query" when ask-network was on (with a hint that this needs host networking when N = 0).
- Certificate row: one row "Monitor N certificates" listing the unmonitored `host:port` targets with checkboxes (all checked); selecting it adds a queue item of kind `tls` after the product candidates. Already-monitored targets are listed as such and not selectable.
- `ConnectorEditForm`: the load/edit/save body of `ConnectorEditPage` moves into a component `{connectorId, overrides?, onSaved, onCancel}`; `ConnectorEditPage` renders it with `onSaved` navigating as today. `overrides` is applied once to the loaded values (for the queue: `targets` = existing targets plus the new ones, duplicates skipped, kept to the probe's 100-target cap; targets that do not fit are listed above the form as "not added").
- Queue item kinds: `candidate` (v1), `addOn` (a `ConnectorForm` with `initialType` `dnsresolver` and the add-on URL, offered right after its pfSense/OPNsense candidate whether that candidate was saved or skipped; skipping it is allowed), and `tls`. A `tls` item renders `ConnectorEditForm` for the chosen TLS Probe, or `ConnectorForm` with `initialType` `tlsprobe` and the targets when none exists. When several TLS Probes exist, a select above the form chooses one, defaulting to the most recently created. Save, skip and stop behave as in v1.
- `store/discovery.ts` keeps v1's event rules; `markConnected` also marks add-ons and TLS targets created or edited through the queue.
- Onboarding's sync step still syncs every connector created through the queue; an edited TLS Probe is synced too.

### D14. Audit
- `discovery.scan.start`: `{ranges, includeInventory, askNetwork, planned: {range, inventory}, skipped: {inventory}}`.
- `discovery.scan.complete` / `.cancel`: v1 fields (addresses probed, answered, candidates per type, duration, partial; `cancelledBy`, `cancelReason`) plus `sources {range, inventory, passive}`, `skipped {inventory, passive}`, `passiveAnswered`, `tlsTargets` (count).
- `discovery.scan.reject`: `{ranges, reason}` with each range cut to 64 characters and at most 16 echoed; `cidr` for a v1-shaped request.
- No entry holds a known host, network answer or found address; typed ranges are what the admin submitted, as in v1.

## Risks / Trade-offs

- [Known hosts reach outside the ranges the admin typed] → only `Allowed` classes, only addresses WiseLabz already synced from connectors the admin configured, budget-capped, the dialer bound to the exact set, audited with counts.
- [A LAN host answers the multicast query to get itself probed] → it can only add its own source address, inside the allowed classes and the budget; probes are the same unauthenticated GETs as v1.
- [1,024 addresses take longer and generate more traffic] → deadline scaling, unchanged concurrency, six scans per hour; documented.
- [CGNAT addresses reached through the server's Tailscale or NetBird interface] → reachable only when the server itself is on the mesh; documented.
- [Passive finds nothing on the default bridge compose] → the count is shown and the docs explain host networking.
- [Inventory addresses are stale (a guest moved)] → the probe simply finds nothing there; the address counts as probed.
- [Extracting `ConnectorEditForm` regresses the edit page] → the existing `ConnectorEditPage.test.tsx` must pass unchanged.

## Migration Plan

Additive. No data migration. v1 request bodies keep working. Rollback is reverting the change.

## Open Questions

None. Details decided while writing this design (not asked in the planning session): at most 16 ranges per request; the extra 60 s of deadline for the passive share; the 3 s passive window; the TLS Probe select defaulting to the most recently created; the add-on offered even when its parent candidate was skipped.
