# Spec Delta

## MODIFIED Requirements

### Requirement: Accepted range
A scan SHALL take zero or more ranges in CIDR notation, at most 16. Each range SHALL be IPv4, SHALL lie entirely inside 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 or 100.64.0.0/10, and SHALL have a prefix length of 22 or more. Any other range SHALL be rejected with a field error naming that range and no probe SHALL be sent. For prefixes of 30 or less the network and broadcast addresses SHALL NOT be probed. Overlapping ranges SHALL be accepted and each address SHALL be probed at most once. A scan with no range SHALL be accepted only when known hosts are included. A request that names a single range in the earlier one-range form SHALL be treated as a scan of that range alone, without known hosts or network answers.

#### Scenario: Private /24
- **WHEN** an admin starts a scan of `192.168.1.0/24`
- **THEN** the scan SHALL start and SHALL cover 254 addresses.

#### Scenario: Several ranges
- **WHEN** an admin starts a scan of `192.168.1.0/24` and `192.168.20.0/24`
- **THEN** the scan SHALL start and SHALL cover 508 addresses.

#### Scenario: Mesh range
- **WHEN** an admin starts a scan of `100.100.4.0/24`
- **THEN** the scan SHALL start.

#### Scenario: Too wide
- **WHEN** an admin starts a scan of `10.0.0.0/21`
- **THEN** the request SHALL be rejected with a field error saying the range may be at most a /22.

#### Scenario: Public range
- **WHEN** an admin starts a scan of `8.8.8.0/24`
- **THEN** the request SHALL be rejected with a field error saying only private and mesh ranges are accepted.

#### Scenario: Loopback, link-local and IPv6
- **WHEN** an admin starts a scan of `127.0.0.0/24`, `169.254.169.0/24` or `fd00::/120`
- **THEN** each request SHALL be rejected with a field error.

#### Scenario: Host bits set
- **WHEN** an admin starts a scan of `192.168.1.57/24`
- **THEN** the scan SHALL cover `192.168.1.0/24` and SHALL report that range.

#### Scenario: Nothing to scan
- **WHEN** an admin starts a scan with no range and without known hosts
- **THEN** the request SHALL be rejected with a field error and no probe SHALL be sent.

#### Scenario: Earlier request form
- **WHEN** a client sends a scan request that names one range in the earlier one-range form
- **THEN** the scan SHALL cover that range only, with known hosts and network answers off.

### Requirement: Probes stay inside the range
Every connection a scan makes SHALL be to an address in the scan's target set: the addresses of its ranges, the known hosts admitted to it and the network answers admitted to it. The scan SHALL NOT follow HTTP redirects, SHALL NOT resolve host names, and SHALL remain subject to the blocked-address rules that apply to connectors. The only traffic outside the target set SHALL be the network query of the requirement "Asking the network".

#### Scenario: Redirect to another host
- **WHEN** a probed address answers with a redirect to an address outside the target set
- **THEN** the scan SHALL NOT connect to the redirect target and SHALL judge the product from the redirect response alone.

### Requirement: Product confirmation
For each port that accepts a connection the scan SHALL send at most one unauthenticated HTTP or HTTPS request per connector type listed for that port and SHALL report a candidate only when the response identifies that product. TLS certificates SHALL NOT be required to be trusted. The scan SHALL NOT send credentials. An open port whose response does not identify a listed product SHALL NOT be reported as a candidate. Each candidate SHALL carry the connector type, address, port, the connector URL to prefill, and where its address came from: a range, the known hosts, or the network.

#### Scenario: Proxmox found
- **WHEN** `10.0.0.5:8006` answers the probe as Proxmox VE with a self-signed certificate
- **THEN** the result SHALL contain a candidate of type `proxmox` at `10.0.0.5` port 8006 with the URL `https://10.0.0.5:8006/api2/json`.

#### Scenario: Open port, different product
- **WHEN** `10.0.0.9:8080` accepts a connection and answers as something other than Traefik
- **THEN** the result SHALL NOT contain a candidate for `10.0.0.9:8080`, and the count of addresses that answered SHALL include `10.0.0.9`.

#### Scenario: Two products on one host
- **WHEN** one address is confirmed as Proxmox VE on 8006 and as Portainer on 9443
- **THEN** the result SHALL contain two candidates for that address.

#### Scenario: Candidate from known hosts
- **WHEN** a known host outside every typed range is confirmed as Home Assistant
- **THEN** its candidate SHALL say that its address came from the known hosts.

### Requirement: Already connected hosts
A candidate whose address and port match the URL of an existing connector of the same type SHALL be reported with that connector's identifier. The web result list SHALL show it as already connected with a link to the connector and SHALL NOT allow it to be selected. The same rule SHALL apply to a candidate's add-on connectors, and a certificate target SHALL be shown as already monitored when an existing TLS Probe lists the same address and port.

#### Scenario: Known Proxmox
- **WHEN** a scan confirms Proxmox VE at `10.0.0.5:8006` and a Proxmox connector with URL `https://10.0.0.5:8006/api2/json` exists
- **THEN** the candidate SHALL carry that connector's identifier and SHALL be shown as already connected.

#### Scenario: Certificate already monitored
- **WHEN** a scan completes a TLS handshake with `10.0.0.5:8006` and a TLS Probe lists `10.0.0.5:8006`
- **THEN** that certificate target SHALL be shown as already monitored and SHALL NOT be offered again.

### Requirement: Scan lifecycle
Starting a scan SHALL return at once with a scan identifier while the scan runs in the background. While it runs, the admin who started it SHALL receive progress (addresses done out of total) and each candidate as it is confirmed, and a completion event when it ends. The total SHALL grow when network answers are admitted. The current or most recent scan SHALL be readable with its state (running, completed, cancelled or failed), ranges, whether known hosts and network answers were included, start time, progress, the number of addresses that answered on any listed port, the number of addresses admitted from each source and skipped for lack of budget, the number of hosts that answered the network query, candidates, and certificate targets. A scan SHALL end within 60 seconds for every 256 addresses admitted at its start, rounded up, plus 60 seconds when it asks the network and started with fewer than 1,024 addresses, and never later than 4 minutes after starting; one that has not finished by then SHALL be marked completed with the candidates found so far and flagged as partial.

#### Scenario: Start and complete
- **WHEN** an admin starts a scan of a valid range
- **THEN** the response SHALL be accepted with a scan identifier, and when probing ends the scan SHALL read as completed with its candidates and counts.

#### Scenario: Reload while running
- **WHEN** the admin reloads the page during a scan and reads the current scan
- **THEN** the response SHALL show it running with the candidates confirmed so far.

#### Scenario: Nothing found
- **WHEN** a scan completes with no confirmed product
- **THEN** it SHALL read as completed with no candidates and with the number of addresses that answered.

#### Scenario: Deadline for a large scan
- **WHEN** a scan starts with 600 addresses and without asking the network
- **THEN** it SHALL be ended no later than 3 minutes after it started.

### Requirement: Scan audit
Each started scan SHALL produce an audit entry `discovery.scan.start` with the actor, the ranges, whether known hosts and network answers were included, the number of addresses planned from ranges and from known hosts, and the number of known hosts skipped for lack of budget. Each scan SHALL produce an entry when it ends: `discovery.scan.complete` or `discovery.scan.cancel`, with the number of addresses probed, the number that answered, the number of candidates per connector type, the duration, whether the result was partial, the number of addresses admitted from each source, the number skipped from each source, the number of hosts that answered the network query, and the number of certificate targets. A request refused for an invalid range, a missing target, a running scan or the hourly limit SHALL produce `discovery.scan.reject` with the submitted ranges and the reason. Audit entries SHALL NOT contain the address of a scanned host, a known host, a host that answered the network query or a found host.

#### Scenario: Completed scan
- **WHEN** a scan of `192.168.1.0/24` with known hosts completes with one Proxmox and one Home Assistant candidate
- **THEN** the audit log SHALL hold a start entry with the range and the planned counts, and a complete entry with the addresses probed per source and one candidate each for `proxmox` and `home_assistant`, and neither entry SHALL contain a host address.

#### Scenario: Rejected range
- **WHEN** an admin submits `8.8.8.0/24`
- **THEN** the audit log SHALL hold a `discovery.scan.reject` entry with the submitted range and the reason.

### Requirement: Range suggestions
The system SHALL offer suggested ranges to an instance admin: first the /24 containing the address the admin's request came from, when that address is private or in 100.64.0.0/10, then the /24 containing each of the server's own such IPv4 interface addresses, then up to three /24s holding the most known hosts, without duplicates. Each suggestion SHALL say which of the three it is. The suggestions SHALL also report how many known hosts a scan would include. When no source yields a range the list SHALL be empty. The web range field SHALL be prefilled with the admin's own range when present and SHALL otherwise be empty.

#### Scenario: Admin on the LAN, server in a container
- **WHEN** the admin's request comes from `192.168.1.50`, the server's only interface is `172.18.0.3/16`, and there are no known hosts
- **THEN** the suggestions SHALL be `192.168.1.0/24` marked as the admin's network, then `172.18.0.0/24` marked as the server's network, the known host count SHALL be 0, and the field SHALL be prefilled with `192.168.1.0/24`.

#### Scenario: Known hosts cluster
- **WHEN** twelve known hosts are in `10.0.20.0/24`, three in `10.0.30.0/24`, and the admin's network is `192.168.1.0/24`
- **THEN** the suggestions SHALL include `10.0.20.0/24` and then `10.0.30.0/24` marked as known networks after the admin's network.

#### Scenario: Admin reaches the instance over a public address
- **WHEN** the admin's request comes from a public address, the server has no private interface address and there are no known hosts
- **THEN** the suggestions SHALL be empty and the field SHALL be empty.

### Requirement: Scan from onboarding and from the add-connector page
The web app SHALL offer the scan inside onboarding's connect step, beside adding a connector by hand, and as an action on the add-connector page. Both SHALL be shown only to instance admins. Adding a connector by hand SHALL remain available without scanning. The scan form SHALL accept several ranges, SHALL offer including known hosts (with their count) and asking the network, both selected by default, and SHALL show how much of the 1,024-address budget the ranges and known hosts use before the scan starts.

#### Scenario: Admin in onboarding
- **WHEN** an instance admin reaches onboarding's connect step
- **THEN** the step SHALL offer both scanning the network and adding a connector by hand.

#### Scenario: Non-admin in onboarding
- **WHEN** a user who is not an instance admin reaches onboarding's connect step
- **THEN** only the manual connector form SHALL be shown.

#### Scenario: Budget shown
- **WHEN** an admin enters `192.168.1.0/24` and `192.168.2.0/24` with 900 known hosts included
- **THEN** the form SHALL show the budget as fully used and SHALL say that 384 known hosts will not fit.

### Requirement: Connect queue
The admin SHALL be able to select several candidates, add-on connectors and the certificate targets, and work through them one after another. Each candidate SHALL open the standard connector form with the connector type chosen and the address filled in. An add-on SHALL open right after its candidate with its type chosen and its URL filled in. The certificate targets SHALL open the edit form of an existing TLS Probe with the targets appended, or a new TLS Probe form with the targets filled in when none exists. Each step SHALL show the position in the queue and offer save, skip and stop. Saving SHALL create or update the connector and move on. Skipping SHALL move on without changing anything. A failed save SHALL stay on that step with the error. Stopping SHALL return to the result list. Candidates connected and targets added in the queue SHALL then be shown as already connected or already monitored.

#### Scenario: Two selected, both saved
- **WHEN** the admin selects two candidates, completes and saves both forms
- **THEN** two connectors SHALL exist and both candidates SHALL be shown as already connected.

#### Scenario: Skip one
- **WHEN** the admin skips the first of two candidates and saves the second
- **THEN** only the second connector SHALL exist and the first candidate SHALL remain selectable.

#### Scenario: Save fails
- **WHEN** saving a step's form fails validation
- **THEN** the queue SHALL stay on that step showing the error, and the admin SHALL be able to correct it or skip.

#### Scenario: pfSense with DNS Resolver
- **WHEN** the admin selects a pfSense candidate and saves it
- **THEN** the next step SHALL be a DNS Resolver form with the pfSense URL filled in and its API key empty.

#### Scenario: Certificates appended to an existing probe
- **WHEN** a TLS Probe with 98 targets exists and the admin queues four new certificate targets
- **THEN** the queue SHALL open that probe's edit form with two targets appended and SHALL list the other two as not added.

## ADDED Requirements

### Requirement: Known hosts as scan targets
When the admin includes known hosts, the scan SHALL also probe every address held in the entity index of the instance's connectors that is a private or 100.64.0.0/10 IPv4 address or a single fd00::/8 IPv6 address, except container addresses reported by Docker and Portainer connectors. Known hosts SHALL be probed even when they lie outside every typed range. Known hosts already inside a typed range SHALL be probed once.

#### Scenario: Proxmox guest outside the ranges
- **WHEN** a Proxmox connector reports a VM at `10.0.50.12`, the admin scans `192.168.1.0/24` with known hosts included
- **THEN** `10.0.50.12` SHALL be probed.

#### Scenario: Docker containers are left out
- **WHEN** a Docker connector reports a container at `172.17.0.4` and known hosts are included
- **THEN** `172.17.0.4` SHALL NOT be probed unless it lies in a typed range.

#### Scenario: Public known host
- **WHEN** a connector reports an entity at `203.0.113.7` and known hosts are included
- **THEN** `203.0.113.7` SHALL NOT be probed.

### Requirement: Asking the network
When the admin asks the network, the scan SHALL, at its start, send one mDNS query and one SSDP search from each of the server's interfaces that has a private, 100.64.0.0/10 or fd00::/8 address, and SHALL listen for answers for 3 seconds. The address an answer came from SHALL be probed when it is a private or 100.64.0.0/10 IPv4 address or an fd00::/8 IPv6 address. Addresses named inside an answer SHALL NOT be used. The scan SHALL report how many distinct hosts answered.

#### Scenario: Device answers
- **WHEN** `192.168.1.77` answers the SSDP search during a scan of `10.0.0.0/24`
- **THEN** `192.168.1.77` SHALL be probed and any candidate found there SHALL say that its address came from the network.

#### Scenario: Answer names another host
- **WHEN** an mDNS answer from `192.168.1.77` contains an address record for `192.168.1.200`
- **THEN** `192.168.1.200` SHALL NOT be probed because of that answer.

#### Scenario: No multicast reaches the server
- **WHEN** the server runs on a container bridge network and nobody answers
- **THEN** the scan SHALL complete normally and SHALL report that no host answered the network query.

### Requirement: Address budget
A scan SHALL probe at most 1,024 addresses. The addresses of the typed ranges SHALL fit in the budget, otherwise the request SHALL be rejected with a field error. Known hosts SHALL fill what the ranges leave, and network answers SHALL fill what remains after that. Addresses that do not fit SHALL NOT be probed and SHALL be counted per source in the result.

#### Scenario: Ranges over budget
- **WHEN** an admin starts a scan of five /24 ranges
- **THEN** the request SHALL be rejected with a field error saying a scan may cover at most 1,024 addresses.

#### Scenario: Known hosts overflow
- **WHEN** an admin scans `192.168.1.0/24` and `192.168.2.0/24` with 900 known hosts outside those ranges
- **THEN** 516 known hosts SHALL be probed and the result SHALL report 384 known hosts skipped.

### Requirement: Certificate monitoring offer
For every probed port that completed a TLS handshake the scan SHALL record a certificate target with its address and port, whether or not a product was confirmed there. The web result list SHALL offer the certificate targets that are not already monitored as one grouped item that adds them to a TLS Probe.

#### Scenario: Certificates offered
- **WHEN** a scan completes TLS handshakes with `10.0.0.5:8006` and `10.0.0.8:443` and neither is monitored
- **THEN** the result list SHALL offer one item to monitor both certificates.

### Requirement: DNS Resolver add-on
A pfSense or OPNsense candidate SHALL carry a DNS Resolver add-on whose URL is the candidate's appliance URL. The add-on SHALL NOT carry or reuse any credential.

#### Scenario: OPNsense found
- **WHEN** a scan confirms OPNsense at `10.0.0.1:443`
- **THEN** the candidate SHALL carry a DNS Resolver add-on with the URL `https://10.0.0.1:443`.
