# Network discovery

Network discovery lets an instance admin scan one private IPv4 range for services
WiseLabz has connectors for. Each product it confirms becomes a candidate such as
"Found Proxmox at 10.0.0.5 - connect?", and you choose which ones to connect. Results
are held in memory only; nothing found is stored in the database.

## Using it

Scanning is available to instance admins only. Other users see the manual connector
form and nothing else.

You can start a scan in two places:

- **Onboarding**, on the connect step, beside adding a connector by hand.
- **Add connector** → **Scan network**.

Then:

1. The **range** field is prefilled with your own /24 when one is found. Suggested
   ranges are shown as chips; click one to use it, or type a range yourself.
2. Click **Scan**. A progress bar shows addresses done out of the total.
3. Results appear as they are confirmed. A candidate that already has a connector of
   the same type at that address and port is marked **Already connected**, links to
   that connector, and cannot be selected.
4. Select one or more candidates and click **Connect N selected**. Each candidate opens
   the normal connector form with the type and address filled in, with its position
   in the queue shown:
   - **Save** creates the connector and moves to the next candidate.
   - **Skip** moves on without creating anything. The skipped candidate stays selectable.
   - **Stop** returns to the result list.
   - If a save fails validation, the queue stays on that candidate and shows the error.
     Correct it or skip it.
5. Candidates you connected are then shown as **Already connected**.

In onboarding, the sync step runs after the queue. It starts a sync for every connector
the queue created and shows each one's progress before onboarding continues. If you
skipped every candidate, onboarding stays on the connect step so you can add a
connector by hand or scan again. If you stop the queue after connecting some
candidates, the connect step offers **Continue with N connected** to move on to the
sync step.

## What a scan sends

For every address in the range, a scan:

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
- Every connection goes through a dialer that refuses any address outside the submitted
  range. This applies on top of the usual block on loopback, link-local, unspecified
  and multicast addresses.

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
| OPNsense (`opnsense`) | 443 https, then 80 http | GET / | login page title ends `| OPNsense` and has a `usernamefld` field |
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

### Never reported

These connector types are never reported by a scan: hosted services (Cloudflare,
Tailscale, Netbird), custom REST, DNS resolver and TLS probe.

### What discovery does not do

- No passive discovery (mDNS, SSDP, ARP).
- No IPv6.
- No public or CGNAT ranges, and no ranges wider than /24.
- No listing of unknown open ports.
- No reading of product versions.
- No storage of results. See [Limits](#limits) for how long results stay readable.

## Accepted ranges

A scan takes exactly one range in CIDR notation. The range must:

- be IPv4;
- lie entirely inside `10.0.0.0/8`, `172.16.0.0/12` or `192.168.0.0/16`;
- have a prefix of /24 or longer (a /24 covers 254 addresses).

Host bits are masked. `192.168.1.57/24` scans `192.168.1.0/24`, and the page reports
that range. For ranges with a prefix of /30 or less (/24 to /30), the network and
broadcast addresses are skipped. A /31 or /32 scans every address in it.

Ranges that are rejected each show a field error and send no probes:

| Range | Why it is rejected |
|---|---|
| `8.8.8.0/24` | Public range. Only private ranges are accepted. |
| `10.0.0.0/23` | Too wide. The range may be at most a /24. |
| `127.0.0.0/24` | Not inside `10.0.0.0/8`, `172.16.0.0/12` or `192.168.0.0/16`. |
| `169.254.169.0/24` | Not inside `10.0.0.0/8`, `172.16.0.0/12` or `192.168.0.0/16`. |
| `fd00::/120` | IPv6. Only IPv4 is accepted. |

## Limits

| Limit | Value |
|---|---|
| Scans running at once | 1 per instance. A second start is refused with `409`, naming the running scan. |
| Scan starts | 6 per rolling hour per instance. A seventh is refused with `429` and a `Retry-After` header. A cancelled scan counts. A rejected request does not. |
| Scan duration | At most 60 s. A scan cut short is marked **completed** and **partial**, and keeps the candidates found so far. |
| Result retention | Readable until 15 minutes after the scan ended, until the next scan starts, or until the server restarts, whichever comes first. |
| Range size | Wider than a /24 is refused. A /24 covers 254 addresses. |

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

The audit log records the range, the outcome and counts. It never records the address
of a scanned or found host. If you scan a single address (a /32), the range recorded
is that one address, since it is the one the admin chose.

## Range suggestions and Docker

Suggestions are built in this order, without duplicates:

1. The /24 around the address your request came from, when that address is private.
2. The /24 around each private IPv4 address of the server.

Each suggestion says whether it is your network or the server's. Only your own range is
prefilled in the range field; the server's ranges are offered as chips only. When
neither gives a private range, the list is empty and the field is empty.

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

## Already-connected matching

A candidate is matched to an existing connector when the connector has the same type
and its URL has the same IP address and port as the candidate.

Connectors configured with a host name are not matched, because host names are never
resolved during a scan. Such a candidate stays selectable, and the connector form's own
duplicate handling applies.
