# Proposal

## Why

Network discovery (GitHub #515, follow-up #737, archived change `2026-10-16-network-discovery`) scans one private /24 the admin types. That misses what a homelab usually looks like: several VLANs, a Tailscale or NetBird mesh in 100.64.0.0/10, guests the instance already knows about from Proxmox or a DHCP server, and devices that announce themselves over mDNS or SSDP. It also throws away two things the scan already sees: every port that completed a TLS handshake (certificates WiseLabz could watch) and the pfSense or OPNsense box whose host overrides the DNS Resolver connector reads.

## What Changes

- A scan takes **one or more ranges**, each IPv4, inside RFC 1918 or 100.64.0.0/10, and no wider than a /22, with at most **1,024 addresses** across the whole scan.
- **Known hosts:** the scan can include every address WiseLabz already holds in its entity index (VMs, LXCs, mesh devices, network devices, interfaces, DNS clients, DHCP leases), except Docker and Portainer container addresses. Known hosts may lie outside the typed ranges and may be single IPv6 unique local addresses (fd00::/8). A typed range is optional when known hosts are included.
- **Ask the network:** while a scan runs, the server sends one mDNS and one SSDP query and listens for a few seconds; every host that answers from an allowed address is probed like any other. Only the answer's source address is used.
- Budget order: typed ranges must fit; known hosts fill what is left; network answers fill the rest during the scan. What does not fit is counted and shown, not probed.
- Every probe dials through a **target-set dialer** that refuses any address outside the scan's exact target set, which grows only through the budget-checked admission of network answers.
- The scan deadline grows with the target set: 60 seconds per 256 addresses, at most 4 minutes. One scan at a time and six per hour are unchanged.
- **Monitor certificates:** every scanned port that completed a TLS handshake is offered as one grouped row that adds those `host:port` targets to an existing TLS Probe (through its edit form, duplicates skipped, overflow beyond its cap listed) or prefills a new one.
- **DNS Resolver add-on:** a pfSense or OPNsense candidate also offers a DNS Resolver connector with the appliance URL prefilled.
- Range suggestions add up to three `/24`s where known hosts cluster, and report how many known hosts a scan would include.
- Each candidate says where its address came from: a range, known hosts or the network.
- Web: range chips, "Include known hosts (N)" and "Ask the network" toggles (both on), a budget meter, the source on each candidate, the certificate row and the add-on in the connect queue.
- Audit entries record the ranges, the toggles and counts per source; they still hold no host address.

Not in this change: new products or ports, IPv6 ranges, public ranges, a background multicast listener, trusting addresses named inside mDNS records or SSDP `LOCATION` headers, storing scan results, and detecting the container network mode.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `network-discovery`: accepted ranges and the address budget, the target-set guarantee, known hosts and network answers as scan targets, deadline scaling, candidate source, certificate monitoring and the DNS Resolver add-on, range suggestions, audit detail, and the scan form.

## Impact

- Backend: `internal/discovery` (ranges, target set, inventory and passive sources, scanner, manager), `internal/api/discovery` (request, suggestions, audit), `internal/connector/connector.go` (target-set dialer replaces the single-range one), `internal/connector/registry.go` (optional add-on declared by a hint), `pfsense` and `opnsense` hints, `internal/store` (an unscoped listing of indexed entity addresses with their connector type), `internal/connector/tlsprobe` (exported target parsing for the already-monitored check); `docs/openapi.yaml`, `docs/WS_CONTRACT.md`, `docs/AUDIT.md`, `docs/NETWORK_DISCOVERY.md`.
- Web: `features/discovery` (panel, flow, queue), `store/discovery.ts`, a `ConnectorEditForm` extracted from `ConnectorEditPage` so the queue can edit a TLS Probe inline, mocks, i18n (en, pt-BR), regenerated API client.
- Network: up to 1,024 addresses per scan instead of 254; when "Ask the network" is on, one mDNS query to 224.0.0.251 and ff02::fb and one SSDP M-SEARCH to 239.255.255.250 and ff02::c per scan, from an ephemeral port.
- No database migration, no new dependency. The request field `cidr` stays accepted as a one-range shorthand.
