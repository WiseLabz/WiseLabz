# Proposal

## Why

An expired certificate is one of the most common homelab outages and nothing in WiseLabz tracks expiry today (GitHub #507). Nginx Proxy Manager certificates are already synced with an expiry date nobody evaluates, and hosts served by anything else are invisible.

## What Changes

- Add a **TLS probe** connector type: its configuration is a list of `host:port` targets; each sync performs a TLS handshake with every target and records one `certificate` entity per target with its expiry, issuer, names and reachability.
- The TLS probe can optionally take additional targets from a Traefik connector: the literal `Host(...)` names of its TLS routers, probed on port 443.
- Certificates from Nginx Proxy Manager gain a normalized `not_after` attribute so every source reports expiry under one name.
- Compliance rules gain two operators that compare the days left until a timestamp attribute.
- Add a **Certificate expiry** rule pack with non-overlapping bands per source: 8 to 30 days left is info, 2 to 7 is warning, 1 or fewer (including expired) is critical. Packs stay opt-in; the web app offers a one-click install when a TLS probe connector is created or the widget is enabled and the pack is missing.
- Add an **Expiring certificates** dashboard widget (default disabled) and the endpoint behind it, listing certificates soonest-first.

Not in this change: certificate data from Caddy, Proxmox or TrueNAS; findings for untrusted chains or hostname mismatch; STARTTLS; probing Traefik's own address with SNI; non-literal Traefik rules.

## Capabilities

### New Capabilities

- `cert-expiry-watch`: sources of certificate expiry data (TLS probe connector, Traefik host import, Nginx Proxy Manager), days-left compliance operators and the certificate expiry rule pack, and the expiring-certificates listing and widget.

### Modified Capabilities

None. Compliance rules have no main spec yet; the two operators are specified inside the new capability.

## Impact

- Backend: new `internal/connector/tlsprobe`; `internal/connector` (optional interface letting a connector ask the sync layer for another connector's and its own previous snapshot); `internal/sync/run.go` (supplies them before `Fetch`); `internal/connector/npm/tables.go` (`not_after`); `internal/compliance/engine.go` and `packs/` (operators, new pack); `internal/api/compliance` (operator validation, pack installed state); a certificates listing handler and route; connector create/update validation for the Traefik reference; `docs/openapi.yaml`, connector docs.
- Web: connector form for the new type, pack-install prompt, compliance rule editor operators, dashboard widget and store, i18n (en, pt-BR), regenerated API client.
- Network: new outbound TLS handshakes from the WiseLabz host to user-listed targets, through the existing guarded dialer. At most 100 targets per connector, 5 second timeout each.
- No breaking changes. Existing NPM `expires_on` stays.
