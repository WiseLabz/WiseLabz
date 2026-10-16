# Network discovery

Network discovery lets an instance admin scan several private or mesh IPv4 ranges,
include known hosts, and ask nearby devices to identify themselves. It finds services
WiseLabz has connectors for and offers certificate monitoring. Each product it confirms becomes a candidate such as
"Found Proxmox at 10.0.0.5 - connect?", and you choose which ones to connect. Results
are held in memory only; nothing found is stored in the database.

## Using it

Scanning is available to instance admins only. Other users see the manual connector
form and nothing else.

You can start a scan in two places:

- **Onboarding**, on the connect step, beside adding a connector by hand.
- **Add connector** → **Scan network**.

Then:

1. The range chips are prefilled with your own /24 when one is found. Enter a CIDR
   and press Enter or comma to add it; remove a chip to leave out that range.
   Suggested ranges can also be added as chips.
2. **Include known hosts (N)** and **Ask the network** are selected by default.
   Known hosts is disabled when N is zero. The budget meter shows how the ranges and
   known hosts fill the 1,024-address limit; network answers fill the remaining space.
3. Click **Scan**. Progress shows addresses done out of the admitted total, which
   can grow when devices answer the network queries.
4. Results show each candidate's source: range, known hosts or network answer.
   Candidates with an existing connector at the same address and port are marked
   **Already connected**. The result also reports skipped hosts and distinct network
   responders, and offers unmonitored certificate targets as a group.
5. Select candidates and certificate targets, then connect them through the normal
   forms with their type and address filled in. **Save** moves to the next item;
   **Skip** leaves it unconnected; **Stop** returns to the results. A validation
   error stays on its form until corrected or skipped.

A pfSense or OPNsense candidate offers a DNS Resolver form immediately after its
own form, whether you saved or skipped the appliance connector. The URL is filled
in, but the API key is empty: no credential is copied or reused.

Certificate targets open an existing TLS Probe's edit form with the selected
addresses appended, or a create form when no probe exists. With several probes,
choose one; the most recently created is selected initially. Manual and imported
targets share the probe's 100-target cap. Targets that do not fit are listed as
**not added** so you can put them in another probe. Already-monitored targets are
listed without a selection checkbox.

In onboarding, the sync step syncs every connector created through the queue and
any TLS Probe edited there. If you skip everything, you can add a connector by hand
or scan again. If you stop after saving some items, **Continue with N connected**
moves to the sync step.

## What a scan sends

For every admitted address from the ranges, known hosts or network answers, a scan:

1. Makes a TCP connection to each port on the list below, with a 500 ms timeout.
2. For each port that accepts the connection, sends one unauthenticated GET for each
   product listed for that port, with a 2 s timeout.

Ports: 80, 81, 443, 2019, 2375, 3000, 8006, 8007, 8080, 8123, 8443, 9443.

The requests follow these rules:

- No credentials are sent.
- Redirects are not followed.
- TLS certificates are not verified. Self-signed certificates are normal on these
  products, and the scan reads only public response fields.
- At most 64 KB of each response body is read.
- Host names are never resolved. The scan connects only to IP addresses.
- Every TCP connection goes through a guarded dialer bound to the exact target set,
  including addresses admitted after the scan starts. It also blocks loopback,
  link-local, unspecified and multicast addresses.
- At most 128 TCP connections or probe requests run concurrently. The fixed
  multicast queries described below are the sole exception to the guarded dialer.

An open port that is not confirmed as a listed product is never shown. It is only
counted toward **answered**, the number of addresses that accepted a connection on any
listed port.

A scan may show up in an intrusion detection system or a firewall log. Scan only
networks you own or administer. You are responsible for the range you scan.

## Discoverable products

| Product (connector type) | Port | Request | What identifies it |
|---|---|---|---|
| Proxmox VE (`proxmox`) | 8006 https | GET / | `Server: pve-api-daemon` header, or page title ending ` - Proxmox Virtual Environment` |
| Proxmox Backup Server (`pbs`) | 8007 https | GET / | page title ending ` - Proxmox Backup Server` |
| Home Assistant (`home_assistant`) | 8123 http | GET /manifest.json | JSON `name` is `Home Assistant` |
| Portainer (`portainer`) | 9443 https | GET /api/system/status | JSON with `Version` and `InstanceID` |
| UniFi Network (`unifi`) | 8443 https | GET /status | JSON `meta.rc` ok with `server_version` and `uuid` |
| AdGuard Home (`adguardhome`) | 3000 http | GET /control/status | `Server: AdGuardHome/...` header |
| Traefik (`traefik`) | 8080 http | GET /api/version | JSON with `Version`, `Codename`, `startDate` |
| Docker (`docker`) | 2375 http | GET /version | `Server: Docker/...` header, or JSON `ApiVersion` plus an `Engine` component |
| Caddy (`caddy`) | 2019 http | GET /config/ | `Etag` header starting `"/config/ ` (Caddy admin API) |
| Nginx Proxy Manager (`npm`) | 81 http | GET /api/ | JSON `status` OK with numeric `version.major`, `minor` and `revision` (the `setup` flag exists only from 2.13) |
| pfSense (`pfsense`) | 443 https, then 80 http | GET / | login page has `<body id="login"`, `/css/login.css` and a `usernamefld` field |
| OPNsense (`opnsense`) | 443 https, then 80 http | GET / | login page title ends `\| OPNsense` and has a `usernamefld` field |
| TrueNAS (`truenas`) | 443 https, then 80 http | GET /ui/ | web UI shell with `id="main-page-title"` and an `<ix-root>` (or `<app-root>`) element |
| Pi-hole (`pihole`) | 443 https, then 80 http | GET /admin/ | `X-Pi-hole` header (v5), or page title starting `Pi-hole` (v6) |

For pfSense, OPNsense, TrueNAS and Pi-hole, https on 443 is tried first and then http
on 80. Each product is reported at most once per host, even when both ports answer.

### Limits of recognition

Recognition is best effort. Be aware of these cases:

- AdGuard Home with web login enabled answers the status request with a bare 401, so
  it is not recognized.
- Pi-hole v6 behind a login redirect, with no page title, is not recognized.
- TrueNAS CORE's legacy UI is not recognized.
- TrueNAS is recognized at `/ui/` on 443, or on 80 only when its HTTP to HTTPS redirect
  is off.
- A product behind a reverse proxy, or on a port other than the ones listed, is not
  found. Add it by hand with the manual form.
- Products are recognized by what they return today, not by version.

The manual connector form is always available, so a product that is not found can
still be connected.

### Other connector types

Hosted services (Cloudflare, Tailscale, Netbird) and custom REST are not product
candidates. DNS Resolver is offered as an appliance add-on; TLS Probe is offered
through the certificate targets. These offers do not add new fingerprint probes
or ports.

### What discovery does not do

- No ARP discovery or background multicast listener.
- No IPv6 ranges; single fd00::/8 addresses from known hosts or network answers
  are supported.
- No public ranges, and no ranges wider than /22.
- No listing of unknown open ports.
- No reading of product versions.
- No storage of results. See [Limits](#limits) for how long results stay readable.

## Accepted ranges

A scan takes up to 16 ranges in CIDR notation. Each range must:

- be IPv4;
- lie entirely inside `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16` or
  `100.64.0.0/10`;
- have a prefix of /22 or longer (a /22 covers 1,022 usable addresses).

Host bits are masked: `192.168.1.57/24` scans `192.168.1.0/24`. Prefixes of /30 or
less skip their network and broadcast addresses. A /31 or /32 scans every address
in it. Overlapping or repeated ranges are allowed and each address is probed once.

The ranges together must fit in the **1,024-address budget**. Five disjoint /24s
are refused. Known hosts fill the space remaining after the ranges, then network
answers fill what is left. Overflow from those two sources is skipped and counted,
not rejected. Two /24s use 508 slots; with 900 known hosts outside them, 516 known
hosts are admitted and 384 are skipped. Each address keeps its first source.

With no range, **Include known hosts** must be on. Asking the network alone is not
enough to start a scan.

| Range | Why it is rejected |
|---|---|
| `8.8.8.0/24` | Public range. Only private and mesh ranges are accepted. |
| `10.0.0.0/21` | Too wide. The range may be at most a /22. |
| `127.0.0.0/24` | Loopback, outside the allowed blocks. |
| `169.254.169.0/24` | Link-local, outside the allowed blocks. |
| `fd00::/120` | IPv6 ranges are not accepted. |

Mesh ranges such as Tailscale or NetBird's `100.64.0.0/10` addresses are reachable
only when the WiseLabz server itself is on that mesh. A browser on the mesh does
not give the server access to it.

## Known hosts

Known hosts come from addresses WiseLabz's existing connectors have synced into
the entity index, including hosts outside the typed ranges. Only private or
`100.64.0.0/10` IPv4 addresses and single `fd00::/8` IPv6 addresses qualify. Public,
loopback, link-local, multicast and unparseable addresses are ignored.

Docker and Portainer **container** rows are excluded because their addresses are
usually internal to a container network. Proxmox LXC containers are kept; the
exclusion depends on both connector type and entity kind. An excluded container
address can still be probed when it is covered by a typed range.

Addresses are de-duplicated and sorted, IPv4 before IPv6, so budget truncation is
deterministic. An address already inside a typed range is not added again. A stale
address simply produces no result if nothing answers. A failed inventory read
refuses the start with 500 rather than silently leaving out known hosts.

## Asking the network

At scan start, each up, multicast-capable interface with an allowed address sends
both queries for each eligible address family:

- mDNS: a legacy-unicast PTR query for `_services._dns-sd._udp.local.` to
  `224.0.0.251:5353` or `[ff02::fb%interface]:5353`.
- SSDP: `M-SEARCH` with `ST: ssdp:all` and `MX: 2` to `239.255.255.250:1900` or
  `[ff02::c%interface]:1900`.

IPv4 and fd00::/8 interfaces are queried independently, including IPv6-only
interfaces. Sockets use ephemeral local UDP ports, never 5353 or 1900, so existing
mDNS/SSDP services keep their ports. The source listens for **3 seconds**, with
reads capped at **9,000 bytes**. It takes only each answer's UDP source address;
no address record or SSDP `LOCATION` payload can steer a probe to another host.
Link-local IPv6 answers are dropped, while fd00::/8 sources are kept.

These fixed multicast sends are the sole exception to the guarded dialer, which
blocks multicast by design. All TCP probing of responders still uses the exact
target set and the shared 1,024-address budget. The result counts distinct allowed
responders even if some were already targets or could not fit.

This generally needs **host networking** (or equivalent multicast routing). Under
Docker's default bridge network, the count is normally zero. The scan still
completes normally; no answer does not mean there are no devices on the LAN.

## Limits

| Limit | Value |
|---|---|
| Scans running at once | 1 per instance. A second start is refused with `409`, naming the running scan. |
| Scan starts | 6 per rolling hour per instance. A seventh is refused with `429` and a `Retry-After` header. A cancelled scan counts. A rejected request does not. |
| Scan duration | `60 s × ceil(max(static, 1) / 256)`, plus 60 s when asking the network with fewer than 1,024 static targets; capped at 240 s. A deadline-cut scan is **completed** and **partial** and keeps its results. |
| Result retention | Readable until 15 minutes after the scan ended, until the next scan starts, or until the server restarts, whichever comes first. |
| Ranges and budget | At most 16 ranges, /22 to /32, and 1,024 distinct usable addresses across all sources. |

`static` is the range and known-host count admitted at start. Without network
queries, 256 targets get 60 s, 257 get 120 s and 600 get 180 s; 1,024 always get
240 s. With network queries, those first three deadlines become 120, 180 and 240 s.

Limits and results are per server process, the same as the existing rate limiter. If
you run more than one instance of the server, each keeps its own count and results.

## Who can scan, and elevation

Only instance admins can start, read or cancel a scan, and read range suggestions.

- **Starting** a scan needs an elevation token for the action `discovery.scan` when
  step-up for destructive actions is on. When it is off, no elevation is needed.
- **Reading** the current scan, and **cancelling** it, need only instance admin
  access. Any instance admin can read or cancel a scan that someone else started.
- **Live progress** and candidate events go only to the admin who started the scan.
  If you reload the page during a scan, the panel reads the scan again and shows the
  candidates found so far.

## What is audited

Starts, completions, cancellations and refused requests are written to the audit
log under `discovery.scan.*`. See [AUDIT.md](AUDIT.md) for the rows and their details.

Start entries record the typed ranges, both toggles, planned range/known-host
counts and skipped known hosts. End entries add the outcome, probe and candidate
counts, admitted/skipped counts per source, distinct network responders and the
number of TLS targets. Reject entries echo at most 16 submitted ranges of at most
64 characters each, or `cidr` for the earlier request form, and a reason.

No entry records a known, answering, scanned or found host address. A typed /32
appears as that range because it is the address the admin explicitly chose.

## Range suggestions and Docker

Suggestions are built in this order, without duplicates:

1. The /24 around your request's IPv4 address when it is private or in 100.64.0.0/10.
2. The /24 around each such IPv4 address of the server.
3. Up to three /24s with the most known IPv4 hosts, ranked by count, then address.

Each suggestion identifies your network, the server's network or a known network.
Only your own range is prefilled; the others are chips. The known-host count uses
the same filtering as a scan, including IPv6 hosts, before subtracting any ranges.
When no source yields a range, the suggestions and range chips are empty.

Behind a reverse proxy, your address is used only when the proxy is a configured
trusted proxy.

Under Docker's default bridge networking, the server's interface is the container
network, for example `172.18.0.0/24`. Your own address may also show up as the bridge
gateway or Docker's proxy address instead of your LAN address. In that case the
suggestion may be missing or wrong. Type the range of the network where your services
live.

## API

Discovery is under the `discovery` tag in [openapi.yaml](openapi.yaml):
`GET /api/discovery/suggestions`, and `POST`, `GET` and `DELETE /api/discovery/scan`
to start, read and cancel a scan. The WebSocket events `discovery.progress`,
`discovery.candidate` and `discovery.complete` are described in sections 17 to 19 of
[WS_CONTRACT.md](WS_CONTRACT.md).

The start body uses the selected ranges and source toggles:

```json
{"ranges":["192.168.1.0/24","100.100.4.0/24"],"includeInventory":true,"askNetwork":true}
```

The earlier API body `{"cidr": "192.168.1.0/24"}` still scans that one range
with known hosts and network queries off. When both fields are sent, `ranges`
wins. Validation errors use `ranges[i]` for a range entry, or `cidr` for the earlier
form.

## Already-connected matching

A candidate is matched to an existing connector when the connector has the same type
and its URL has the same IP address and port as the candidate.

Connectors configured with a host name are not matched, because host names are never
resolved during a scan. Such a candidate stays selectable, and the connector form's own
duplicate handling applies.

## Certificate monitoring

Every HTTPS probe that completes a TLS handshake contributes one certificate
target per address and port, even when no product matches or the HTTP response
fails afterward. HTTP-only probes contribute no certificate target. This reuses
the scanner's existing TLS transport; it adds no separate insecure client.

On every read, existing TLS Probe configurations are checked for the same IP
literal and port, including currently imported Traefik targets. Host names are not
resolved. Matching targets are marked **Already monitored**, and removing or
editing a probe clears stale markers on the next read. The same refresh applies
to DNS Resolver add-on connectors and the product's already-connected marker.
