# Spec Delta

## Purpose

Lets an instance admin scan a private network range for products WiseLabz has connectors for and turn what is found into connectors with the address already filled in, shortening the time from install to first data.

## ADDED Requirements

### Requirement: Who may scan
Only an instance admin SHALL be able to start, read or cancel a network scan or read range suggestions. Starting a scan SHALL require elevation for the action `discovery.scan` when the instance has step-up for destructive actions enabled, and SHALL NOT require it otherwise.

#### Scenario: Non-admin is refused
- **WHEN** a user who is not an instance admin starts a scan
- **THEN** the request SHALL be refused as forbidden and no probe SHALL be sent.

#### Scenario: Step-up enabled, no elevation
- **WHEN** step-up is enabled and an instance admin starts a scan without an elevation token
- **THEN** the request SHALL be refused with the code `elevation_required` and no probe SHALL be sent.

#### Scenario: Step-up disabled
- **WHEN** step-up is disabled and an instance admin starts a scan with a valid range
- **THEN** the scan SHALL start without an elevation token.

### Requirement: Accepted range
A scan SHALL take exactly one range in CIDR notation. The range SHALL be IPv4, SHALL lie entirely inside 10.0.0.0/8, 172.16.0.0/12 or 192.168.0.0/16, and SHALL have a prefix length of 24 or more. Any other range SHALL be rejected with a field error and no probe SHALL be sent. For prefixes of 30 or less the network and broadcast addresses SHALL NOT be probed.

#### Scenario: Private /24
- **WHEN** an admin starts a scan of `192.168.1.0/24`
- **THEN** the scan SHALL start and SHALL cover 254 addresses.

#### Scenario: Too wide
- **WHEN** an admin starts a scan of `10.0.0.0/23`
- **THEN** the request SHALL be rejected with a field error saying the range may be at most a /24.

#### Scenario: Public range
- **WHEN** an admin starts a scan of `8.8.8.0/24`
- **THEN** the request SHALL be rejected with a field error saying only private ranges are accepted.

#### Scenario: Loopback, link-local and IPv6
- **WHEN** an admin starts a scan of `127.0.0.0/24`, `169.254.169.0/24` or `fd00::/120`
- **THEN** each request SHALL be rejected with a field error.

#### Scenario: Host bits set
- **WHEN** an admin starts a scan of `192.168.1.57/24`
- **THEN** the scan SHALL cover `192.168.1.0/24` and SHALL report that range.

### Requirement: Probes stay inside the range
Every connection a scan makes SHALL be to an address inside the submitted range. The scan SHALL NOT follow HTTP redirects, SHALL NOT resolve host names, and SHALL remain subject to the blocked-address rules that apply to connectors.

#### Scenario: Redirect to another host
- **WHEN** a probed address answers with a redirect to an address outside the range
- **THEN** the scan SHALL NOT connect to the redirect target and SHALL judge the product from the redirect response alone.

### Requirement: Discoverable products
A scan SHALL look for these connector types on these ports and no others: Proxmox VE (8006), Proxmox Backup Server (8007), Home Assistant (8123), Portainer (9443), UniFi (8443), AdGuard Home (3000), Traefik (8080), Docker (2375), Caddy (2019), Nginx Proxy Manager (81), and pfSense, OPNsense, TrueNAS and Pi-hole (80 and 443). Connector types for hosted services, custom REST, DNS resolver and TLS probe SHALL never be reported.

#### Scenario: Port outside the list
- **WHEN** an address in the range listens only on port 22
- **THEN** no connection SHALL be made to port 22 and the address SHALL NOT appear in the result.

### Requirement: Product confirmation
For each port that accepts a connection the scan SHALL send at most one unauthenticated HTTP or HTTPS request per connector type listed for that port and SHALL report a candidate only when the response identifies that product. TLS certificates SHALL NOT be required to be trusted. The scan SHALL NOT send credentials. An open port whose response does not identify a listed product SHALL NOT be reported as a candidate. Each candidate SHALL carry the connector type, address, port, and the connector URL to prefill.

#### Scenario: Proxmox found
- **WHEN** `10.0.0.5:8006` answers the probe as Proxmox VE with a self-signed certificate
- **THEN** the result SHALL contain a candidate of type `proxmox` at `10.0.0.5` port 8006 with the URL `https://10.0.0.5:8006/api2/json`.

#### Scenario: Open port, different product
- **WHEN** `10.0.0.9:8080` accepts a connection and answers as something other than Traefik
- **THEN** the result SHALL NOT contain a candidate for `10.0.0.9:8080`, and the count of addresses that answered SHALL include `10.0.0.9`.

#### Scenario: Two products on one host
- **WHEN** one address is confirmed as Proxmox VE on 8006 and as Portainer on 9443
- **THEN** the result SHALL contain two candidates for that address.

### Requirement: Already connected hosts
A candidate whose address and port match the URL of an existing connector of the same type SHALL be reported with that connector's identifier. The web result list SHALL show it as already connected with a link to the connector and SHALL NOT allow it to be selected.

#### Scenario: Known Proxmox
- **WHEN** a scan confirms Proxmox VE at `10.0.0.5:8006` and a Proxmox connector with URL `https://10.0.0.5:8006/api2/json` exists
- **THEN** the candidate SHALL carry that connector's identifier and SHALL be shown as already connected.

### Requirement: Scan lifecycle
Starting a scan SHALL return at once with a scan identifier while the scan runs in the background. While it runs, the admin who started it SHALL receive progress (addresses done out of total) and each candidate as it is confirmed, and a completion event when it ends. The current or most recent scan SHALL be readable with its state (running, completed, cancelled or failed), range, start time, progress, the number of addresses that answered on any listed port, and candidates. A scan SHALL end within 60 seconds of starting; one that has not finished by then SHALL be marked completed with the candidates found so far and flagged as partial.

#### Scenario: Start and complete
- **WHEN** an admin starts a scan of a valid range
- **THEN** the response SHALL be accepted with a scan identifier, and when probing ends the scan SHALL read as completed with its candidates and counts.

#### Scenario: Reload while running
- **WHEN** the admin reloads the page during a scan and reads the current scan
- **THEN** the response SHALL show it running with the candidates confirmed so far.

#### Scenario: Nothing found
- **WHEN** a scan completes with no confirmed product
- **THEN** it SHALL read as completed with no candidates and with the number of addresses that answered.

### Requirement: Cancelling a scan
An instance admin SHALL be able to cancel the running scan. Cancelling SHALL stop further probes promptly, SHALL keep the candidates already confirmed, and SHALL mark the scan cancelled. Cancelling when no scan is running SHALL have no effect and SHALL report that nothing was running.

#### Scenario: Cancel mid-scan
- **WHEN** an admin cancels a scan after two candidates were confirmed
- **THEN** the scan SHALL read as cancelled with those two candidates and no further candidate SHALL be added.

### Requirement: Results are not stored
Scan results SHALL be held only in memory. A finished scan's result SHALL stop being readable 15 minutes after it ended, when a new scan starts, or when the server restarts, whichever comes first. No scanned address SHALL be written to the database.

#### Scenario: Result expired
- **WHEN** an admin reads the current scan 16 minutes after the last scan ended
- **THEN** the response SHALL report that there is no scan.

### Requirement: Scan limits
At most one scan SHALL run at a time across the instance. At most six scans SHALL be started in any rolling hour across the instance; a cancelled scan counts, a rejected request does not.

#### Scenario: Scan already running
- **WHEN** an admin starts a scan while another is running
- **THEN** the request SHALL be refused as a conflict naming the running scan, and the running scan SHALL be unaffected.

#### Scenario: Hourly limit reached
- **WHEN** six scans were started in the last hour and an admin starts a seventh
- **THEN** the request SHALL be refused as rate limited with the time after which a scan may start.

#### Scenario: Rejected request does not count
- **WHEN** an admin submits an invalid range five times and then a valid one
- **THEN** the valid scan SHALL start and SHALL count as one start.

### Requirement: Scan audit
Each started scan SHALL produce an audit entry `discovery.scan.start` with the actor and the range, and an entry when it ends: `discovery.scan.complete` or `discovery.scan.cancel`, with the number of addresses probed, the number that answered, the number of candidates per connector type, the duration, and whether the result was partial. A request refused for an invalid range, a running scan or the hourly limit SHALL produce `discovery.scan.reject` with the reason. Audit entries SHALL NOT contain the addresses of scanned or found hosts.

#### Scenario: Completed scan
- **WHEN** a scan of `192.168.1.0/24` completes with one Proxmox and one Home Assistant candidate
- **THEN** the audit log SHALL hold a start entry with the range and a complete entry with 254 addresses probed and one candidate each for `proxmox` and `home_assistant`, and neither entry SHALL contain a host address.

#### Scenario: Rejected range
- **WHEN** an admin submits `8.8.8.0/24`
- **THEN** the audit log SHALL hold a `discovery.scan.reject` entry with the submitted range and the reason.

### Requirement: Range suggestions
The system SHALL offer suggested ranges to an instance admin: first the /24 containing the address the admin's request came from, when that address is in a private range, then the /24 containing each of the server's own private IPv4 interface addresses, without duplicates. Each suggestion SHALL say which of the two it is. When neither yields a private range the list SHALL be empty. The web range field SHALL be prefilled with the admin's own range when present and SHALL otherwise be empty.

#### Scenario: Admin on the LAN, server in a container
- **WHEN** the admin's request comes from `192.168.1.50` and the server's only interface is `172.18.0.3/16`
- **THEN** the suggestions SHALL be `192.168.1.0/24` marked as the admin's network, then `172.18.0.0/24` marked as the server's network, and the field SHALL be prefilled with `192.168.1.0/24`.

#### Scenario: Admin reaches the instance over a public address
- **WHEN** the admin's request comes from a public address and the server has no private interface address
- **THEN** the suggestions SHALL be empty and the field SHALL be empty.

### Requirement: Scan from onboarding and from the add-connector page
The web app SHALL offer the scan inside onboarding's connect step, beside adding a connector by hand, and as an action on the add-connector page. Both SHALL be shown only to instance admins. Adding a connector by hand SHALL remain available without scanning.

#### Scenario: Admin in onboarding
- **WHEN** an instance admin reaches onboarding's connect step
- **THEN** the step SHALL offer both scanning the network and adding a connector by hand.

#### Scenario: Non-admin in onboarding
- **WHEN** a user who is not an instance admin reaches onboarding's connect step
- **THEN** only the manual connector form SHALL be shown.

### Requirement: Connect queue
The admin SHALL be able to select several candidates and work through them one after another. Each SHALL open the standard connector form with the connector type chosen and the address filled in, showing the position in the queue and offering save, skip and stop. Saving SHALL create the connector and move to the next candidate. Skipping SHALL move on without creating anything. A failed save SHALL stay on that candidate with the error. Stopping SHALL return to the result list. Candidates connected in the queue SHALL then be shown as already connected.

#### Scenario: Two selected, both saved
- **WHEN** the admin selects two candidates, completes and saves both forms
- **THEN** two connectors SHALL exist and both candidates SHALL be shown as already connected.

#### Scenario: Skip one
- **WHEN** the admin skips the first of two candidates and saves the second
- **THEN** only the second connector SHALL exist and the first candidate SHALL remain selectable.

#### Scenario: Save fails
- **WHEN** saving a candidate's form fails validation
- **THEN** the queue SHALL stay on that candidate showing the error, and the admin SHALL be able to correct it or skip.

### Requirement: Onboarding sync after a queue
When connectors were created through the queue during onboarding, the sync step SHALL start a sync for each of them and SHALL show the progress of each before onboarding continues. When the queue created none, onboarding SHALL stay on the connect step.

#### Scenario: Two connectors created
- **WHEN** the admin finishes a queue that created two connectors during onboarding
- **THEN** the sync step SHALL show sync progress for both connectors.

#### Scenario: Everything skipped
- **WHEN** the admin skips every candidate in the queue during onboarding
- **THEN** onboarding SHALL remain on the connect step.
