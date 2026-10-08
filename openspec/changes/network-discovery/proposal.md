# Proposal

## Why

Onboarding adds connectors one at a time, each typed by hand, so a new instance shows nothing until the admin has looked up every address and port (GitHub #515). Most homelab products WiseLabz supports listen on a well-known port and identify themselves without credentials, so the instance can find them and offer "Found Proxmox at 10.0.0.5 - connect?".

## What Changes

- Add a **network scan** an instance admin can start over one private IPv4 range they type (RFC 1918, /24 or smaller). The scan connects to a fixed set of known connector ports on each address and confirms the product with one unauthenticated HTTP(S) request.
- Discoverable in this change: Proxmox VE, Proxmox Backup Server, Home Assistant, Portainer, UniFi, AdGuard Home, Traefik, Docker, Caddy, pfSense, OPNsense, TrueNAS, Pi-hole and Nginx Proxy Manager. Each connector type declares its own discovery hint.
- The scan runs in the background: starting it returns immediately, progress and each confirmed candidate are pushed to the admin who started it, and the result can be fetched or the scan cancelled while it is held in memory.
- Scans are limited to one at a time per instance and six starts per hour, require elevation when the instance has step-up enabled, and are audit-logged with the range and counts but no host addresses.
- Probes dial through a new range-scoped guarded dialer that refuses any address outside the submitted range, on top of the existing loopback and link-local block.
- The range field suggests the /24 around the admin's own address and the server's private interface subnets.
- Web: a scan panel in onboarding's connect step and on the add-connector page, with a result list where hosts that already have a connector are marked, multi-select, and a queue that opens the normal connector form prefilled for each selected candidate. Onboarding's sync step shows every connector created in the queue.

Not in this change: passive discovery (mDNS, SSDP, ARP), IPv6, public or CGNAT ranges, ranges wider than /24, listing open ports that are not confirmed as a known product, reading a product's version, storing scan results, and discovery of SaaS, custom, DNS resolver or TLS probe connectors.

## Capabilities

### New Capabilities

- `network-discovery`: who may scan and what range is accepted, what is probed and how a product is confirmed, scan lifecycle (start, progress, result, cancel, expiry), limits, audit, range suggestions, and turning candidates into connectors from onboarding and the add-connector page.

### Modified Capabilities

None. Onboarding, connectors' creation form, the outbound address guard and audit have no main spec; the behaviour this change adds to them is specified inside the new capability.

## Impact

- Backend: new `internal/discovery` (range validation, scanner, scan manager) and `internal/api/discovery` (handlers, routes in the instance-admin group); `internal/connector/connector.go` (range-scoped guarded dialer); `internal/connector/registry.go` (optional discovery hint on `TypeSchema`) and the fourteen connector packages that declare one; `internal/ws/ws.go` (three event types); `docs/openapi.yaml`, `docs/WS_CONTRACT.md`, `docs/AUDIT.md`.
- Web: new `features/discovery` (scan panel, connect queue); `ConnectorForm` accepts an initial type and values; `OnboardingPage` connect and sync steps; `AddConnectorPage`; i18n (en, pt-BR); mocks; regenerated API client.
- Network: new outbound traffic from the WiseLabz host, only when an admin starts a scan: up to 254 addresses times 12 ports of TCP connects, and one HTTP(S) request per open port, inside the submitted range.
- No database changes, no new dependencies, no breaking changes.
