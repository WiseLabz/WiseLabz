# TLS Probe connector

The TLS probe tracks when TLS certificates expire for hosts that no other
connector reports. Each sync completes a TLS handshake with every target and
records one `certificate` entity per target: its expiry (`not_after`), start
(`not_before`), issuer, subject, DNS names and whether it is self-signed. The
connector category is `monitoring`.

## Configuration

| Field | Meaning |
|---|---|
| `targets` | One `host:port` per line. A host name or an IP address; IPv6 addresses go in brackets, like `[fd00::10]:8443`. No scheme, no path. Blank lines are ignored and repeated targets count once. |
| `import_connector_id` | Optional. The ID of a Traefik connector whose TLS routers contribute extra hosts (see below). |
| `import_port` | Port probed on each imported host. Defaults to `443`. |

A connector has no URL; the targets are its configuration. Saving checks the
syntax of every line and the limits below without contacting anything, and a
rejected save names the line that is wrong.

Host names are ASCII (letters, digits, `-`, `_` and dots); an internationalised
name must be given in its punycode (`xn--`) form.

Because `targets`, `import_connector_id` and `import_port` decide where the
WiseLabz host connects, creating a probe and changing any of them requires an
instance admin. Connector operators can still sync and view a probe, and can
save it with those settings unchanged.

## What is probed

On every sync the connector dials each target, sends the target's host name as
the TLS server name (SNI; not sent for an IP address), completes the handshake,
reads the **leaf certificate** from the connection and closes it. **Nothing is
written to the connection** beyond the handshake itself, and only the leaf
certificate is read. The handshake offers TLS 1.2 or newer, so an endpoint that
only speaks TLS 1.0 or 1.1 shows up as unreachable with class `handshake`.

The handshake is limited to **5 seconds per target** (connect plus handshake)
and at most **8 targets are probed at once**. A connector holds at most
**100 targets**, listed and imported together.

Connections go through the same guarded dialer as every other connector:
loopback, link-local (including cloud metadata addresses), unspecified and
multicast addresses are refused, private ranges are allowed. The address that is
checked is the one actually connected to, after the host name is resolved. A
target that resolves to a refused address is reported as unreachable with class
`blocked`.

> **Network egress.** The probe makes outbound connections from the WiseLabz
> host to the targets you list (and to the hosts a referenced Traefik connector
> exposes). Make sure that is acceptable on your network. Creating a probe, and
> changing its `targets`, `import_connector_id` or `import_port`, requires an
> instance admin, because those settings choose where the connections go;
> connector operators can still sync and view it.

## Certificates are read without trust validation

The probe deliberately **does not verify the certificate chain or the host
name**. It reads what the server presents so that expiry can be tracked, which
means a self-signed certificate, a certificate from a private certificate
authority and a certificate for a different name are all recorded like any
other, and none of them raises a finding by itself. Verification is switched
off in exactly one place, a function in the connector's dial code named
`insecureSkipVerifyConfig` (it sets `InsecureSkipVerify`), whose comment explains
why; this is safe because the probe sends no data and
acts on nothing it reads except the certificate fields it stores.

Values that come from the certificate (issuer, subject, DNS names) are untrusted
input. Non-printable characters are removed from them and they are capped
(256 bytes each, 50 DNS names) before being stored.

## Entities

Each target becomes a `certificate` entity whose name and external ID are
`host:port`, with these attributes:

| Attribute | Meaning |
|---|---|
| `host`, `port` | The target. |
| `not_after`, `not_before` | Validity as UTC timestamps, RFC 3339 with whole seconds (`2026-11-15T04:17:54Z`), the same format Nginx Proxy Manager certificates use for `not_after`. |
| `issuer`, `subject` | As presented by the certificate. |
| `dns_names` | DNS names the certificate covers, sorted. |
| `self_signed` | Whether the certificate is signed by its own key. |
| `reachable` | Whether the last probe completed a handshake. |
| `error` | Only when unreachable: a class and a short message. |
| `source` | `manual` (listed) or `imported` (from Traefik). |

There is deliberately no "last checked" or "days left" attribute: either would
change on every sync and make every snapshot look different. Days left are
computed when rules are evaluated or a listing is built.

## Unreachable targets

A target that cannot be resolved, connected to or handshaken with does not fail
the sync. Its entity stays, with `reachable` false and an `error` such as
`refused: connection refused`. The classes are:

| Class | Meaning |
|---|---|
| `dns` | The name did not resolve. |
| `refused` | The TCP connection could not be established. |
| `timeout` | No answer within 5 seconds. |
| `handshake` | The peer connected but did not complete a TLS handshake. |
| `blocked` | The address is one the guarded dialer refuses. |

The messages are fixed phrases so they do not change between syncs. While a
target is unreachable its entity keeps the certificate attributes last observed
for it (so an expiry rule keeps tracking it); a target that was never reached
has no `not_after`.

The connector is reported **offline** only when it has at least one target and
every target is unreachable, and **online** when it has no targets. This status
is set by the sync, which is the only thing that dials; the periodic health
check validates the configuration without contacting any target and leaves such
an offline status for the next sync to clear.

## Importing hosts from Traefik

Set `import_connector_id` to a Traefik connector to probe the hosts it routes.
On each sync the probe reads that connector's most recent snapshot and takes the
literal host names from the `Host(...)` matchers of routers that terminate TLS:

- ``Host(`a.example.com`)``, `Host("a.example.com")`, several arguments, and
  matchers joined with `||` or `&&` all count.
- `HostRegexp`, `HostSNI`, wildcard or `{name:regexp}` hosts, and negated
  matchers (`!Host(...)`) contribute nothing, nor do matchers inside a negated
  group (`!(...)`), as do routers without TLS.
- Names are lower-cased and de-duplicated, also against listed targets: a host
  that is both listed and imported on the same port is probed once.

Imported hosts are resolved by name and probed on `import_port`, which tests
what clients see. A name that does not resolve from the WiseLabz host appears as
an unreachable entity. When listed and imported targets together would exceed
100, listed targets are kept and imported hosts are taken in alphabetical order;
the connector's **Import** section states how many were left out.

If a router disappears, its host's entity disappears after the next probe sync.
If the Traefik connector was never synced or has been deleted, the probe simply
uses its listed targets.

**Visibility.** Imported host names appear on the probe connector's entities, so
anyone who can view the probe connector can see them. For that reason saving a
probe that references a Traefik connector requires that you can view that
Traefik connector (otherwise the save is rejected with 403, the same answer for
a connector that does not exist), and that it is a Traefik connector (otherwise
the `config.import_connector_id` field is rejected). The check is made when the
probe is saved: hosts already imported stay visible on the probe if your access
to the Traefik connector is revoked later. If the referenced connector is
deleted or stops being a Traefik connector, the import yields nothing, and
saving the probe is refused until `import_connector_id` is cleared or
corrected.

## Limits

| Limit | Value |
|---|---|
| Targets per connector (listed + imported) | 100 |
| Time per target (connect + handshake) | 5 seconds |
| Concurrent handshakes | 8 |
| Certificate fields stored | 256 bytes each, 50 DNS names |

## Connectors in config.yaml

TLS probe connectors can be declared in `config.yaml` (see
[CONNECTORS_IN_CONFIG.md](../CONNECTORS_IN_CONFIG.md)). Because the probe dials
targets directly rather than communicating with an API endpoint, a `url` is not
accepted.

To import hosts from a Traefik connector declared in the same file, specify its
`name` using `import_connector`:

```yaml
connectors:
  - name: traefik
    type: traefik
    url: http://traefik.lan:8080
  - name: probe-traefik
    type: tlsprobe
    config:
      import_connector: traefik
      import_port: 443
  - name: probe-manual
    type: tlsprobe
    config:
      targets: |
        router.lan:443
        switch.lan:8443
```

To reference a Traefik connector created in the web UI instead, use its UUID via
`import_connector_id: <uuid>`. The two settings are mutually exclusive.
