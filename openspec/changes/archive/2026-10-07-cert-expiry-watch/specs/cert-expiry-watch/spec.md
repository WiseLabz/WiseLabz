# Spec Delta

## Purpose

Tracks when TLS certificates in the lab expire, from connector data and direct TLS probes, and turns approaching expiry into findings and a dashboard view before it causes an outage.

## ADDED Requirements

### Requirement: TLS probe connector
The system SHALL offer a connector type whose configuration is a list of targets, each a host name or IP address with a port. A connector SHALL accept at most 100 targets in total, counting imported ones. On each sync the connector SHALL attempt a TLS handshake with every target, sending the target's host name as the server name, with a timeout of 5 seconds per target, and SHALL produce one `certificate` entity per target identified by its host and port. Targets SHALL be subject to the same blocked-address rules as every other connector.

#### Scenario: Targets are probed
- **WHEN** a TLS probe connector with targets `a.lab:443` and `b.lab:8443` is synced and both complete a handshake
- **THEN** the connector SHALL have two `certificate` entities, one per target.

#### Scenario: Too many targets
- **WHEN** a TLS probe connector is saved with 101 targets
- **THEN** the save SHALL be rejected with a field error naming the targets.

#### Scenario: Malformed target
- **WHEN** a TLS probe connector is saved with a target that has no valid port
- **THEN** the save SHALL be rejected with a field error naming that target.

#### Scenario: Blocked address
- **WHEN** a target resolves to a loopback or link-local address
- **THEN** no handshake SHALL be attempted with it and the target SHALL be reported as unreachable with that reason.

### Requirement: Certificate attributes
Each `certificate` entity produced by a TLS probe SHALL carry: the host and port; `not_after` and `not_before` as UTC timestamps; the issuer; the subject; the DNS names the certificate covers; whether it is self-signed; whether the target was reachable; and how the target was obtained (listed by hand or imported). These attributes SHALL be declared so that compliance rule authoring can offer them. The probe SHALL read the certificate without requiring it to be trusted, and SHALL NOT raise a finding because a certificate is self-signed, untrusted or does not match the host name.

#### Scenario: Self-signed certificate
- **WHEN** a target presents a self-signed certificate
- **THEN** its entity SHALL carry the certificate's `not_after` and SHALL be marked self-signed, and the sync SHALL succeed.

#### Scenario: Private certificate authority
- **WHEN** a target presents a certificate issued by an authority the WiseLabz host does not trust
- **THEN** its entity SHALL carry `not_after` and the issuer, and no finding SHALL be raised for trust.

### Requirement: Unreachable targets
When a target cannot be resolved, cannot be connected to or does not complete a handshake, the sync SHALL still succeed. The target's entity SHALL remain, marked unreachable with the error, and SHALL keep the certificate attributes last observed for that target, if any. The connector SHALL be reported offline only when it has at least one target and every target is unreachable.

#### Scenario: One target down
- **WHEN** one of three targets refuses the connection during a sync and it was reachable on the previous sync
- **THEN** the sync SHALL succeed, that target's entity SHALL be marked unreachable with the error and SHALL keep its previous `not_after`, and the connector SHALL be online.

#### Scenario: Never reachable
- **WHEN** a target has never completed a handshake
- **THEN** its entity SHALL exist, marked unreachable, without a `not_after`.

#### Scenario: Every target down
- **WHEN** every target of a connector is unreachable
- **THEN** the connector's health SHALL be reported offline.

#### Scenario: No targets
- **WHEN** a connector has no listed targets and its import yields none
- **THEN** the sync SHALL succeed with no certificate entities and the connector SHALL be online.

### Requirement: Importing hosts from Traefik
A TLS probe connector SHALL optionally reference one Traefik connector and a port for imported hosts, defaulting to 443. On each sync the probe SHALL add as targets the host names that appear as literal `Host(...)` matchers in the rules of that Traefik connector's routers that terminate TLS, as of that connector's most recent sync. Rules that match hosts by pattern SHALL contribute nothing. Imported hosts SHALL be resolved by name like any listed target. A host both listed and imported with the same port SHALL be probed once. When listed and imported targets together exceed 100, listed targets SHALL be kept, imported hosts SHALL be taken in alphabetical order up to the limit, and the sync SHALL report how many were left out.

#### Scenario: Routers become targets
- **WHEN** the referenced Traefik connector's last sync has TLS routers with rules ``Host(`a.lab`)`` and ``Host(`b.lab`) || Host(`c.lab`)``
- **THEN** the next probe sync SHALL probe `a.lab`, `b.lab` and `c.lab` on port 443 and mark their entities as imported.

#### Scenario: Non-TLS and pattern rules skipped
- **WHEN** a router does not terminate TLS, or its rule uses only a host pattern matcher
- **THEN** it SHALL add no target.

#### Scenario: Router removed
- **WHEN** a router whose host was imported no longer exists after a Traefik sync
- **THEN** after the next probe sync that host's certificate entity SHALL no longer exist, unless the host is also listed by hand.

#### Scenario: Duplicate of a listed target
- **WHEN** `a.lab:443` is listed by hand and `a.lab` is also imported on port 443
- **THEN** there SHALL be one entity for `a.lab:443`.

#### Scenario: Limit exceeded by import
- **WHEN** a connector lists 90 targets and the import yields 30 hosts
- **THEN** the 90 listed targets and the first 10 imported hosts alphabetically SHALL be probed, and the sync result SHALL state that 20 hosts were left out.

#### Scenario: Referenced connector never synced or removed
- **WHEN** the referenced Traefik connector has no completed sync or has been deleted
- **THEN** the probe sync SHALL succeed using only the listed targets.

### Requirement: Access to the import source
Saving a TLS probe connector that references a Traefik connector SHALL require that the caller is permitted to view that Traefik connector, and that the referenced connector exists and is of the Traefik type.

#### Scenario: Reference without access
- **WHEN** a user who cannot view a Traefik connector saves a TLS probe connector referencing it
- **THEN** the save SHALL be rejected with status 403.

#### Scenario: Reference to another connector type
- **WHEN** a TLS probe connector is saved referencing a connector that is not of the Traefik type
- **THEN** the save SHALL be rejected with a field error naming the reference.

### Requirement: Normalized expiry on Nginx Proxy Manager certificates
Certificate entities from an Nginx Proxy Manager connector SHALL carry a `not_after` attribute holding the expiry as a UTC timestamp in the same format the TLS probe uses, in addition to the existing `expires_on` attribute. When the reported expiry cannot be parsed, `not_after` SHALL be absent.

#### Scenario: Expiry normalized
- **WHEN** Nginx Proxy Manager reports a certificate expiring `2026-11-15T04:17:54.000Z`
- **THEN** its entity SHALL have `not_after` equal to `2026-11-15T04:17:54Z` and `expires_on` unchanged.

#### Scenario: Unparseable expiry
- **WHEN** Nginx Proxy Manager reports an expiry that is not a timestamp
- **THEN** the entity SHALL have no `not_after` and the sync SHALL succeed.

### Requirement: Days-left compliance operators
Compliance rule conditions SHALL support two operators, `days_left_lt` and `days_left_gt`, that take a whole number of days and apply to an attribute holding a timestamp. Days left SHALL be the number of whole days from the time of evaluation until the timestamp, rounded down, and negative once the timestamp has passed. `days_left_lt` SHALL hold when days left is less than the value and `days_left_gt` when it is greater. A condition using either operator SHALL NOT hold when the attribute is absent or is not a timestamp. Saving a rule with either operator and a value that is not a whole number SHALL be rejected. Both operators SHALL be offered wherever rule operators are listed.

#### Scenario: Inside the window
- **WHEN** a rule has the condition `not_after days_left_lt 8` and an entity's `not_after` is 5 days and 3 hours away
- **THEN** the condition SHALL hold.

#### Scenario: Already expired
- **WHEN** an entity's `not_after` was 2 days ago and the condition is `not_after days_left_lt 2`
- **THEN** the condition SHALL hold.

#### Scenario: Band with two conditions
- **WHEN** a rule has conditions `not_after days_left_lt 8` and `not_after days_left_gt 1`, and an entity has 1 day and 12 hours left
- **THEN** the rule SHALL NOT match that entity.

#### Scenario: Missing attribute
- **WHEN** an entity has no `not_after`
- **THEN** a condition using `days_left_lt` or `days_left_gt` on `not_after` SHALL NOT hold.

#### Scenario: Non-integer value
- **WHEN** a rule is saved with `days_left_lt` and the value `soon`
- **THEN** the save SHALL be rejected with a field error naming that condition.

### Requirement: Certificate expiry rule pack
The system SHALL ship a rule pack named "Certificate expiry" that a user installs like any other pack. For each of the Nginx Proxy Manager and TLS probe connector types it SHALL contain three rules on `certificate` entities over `not_after`: 8 to 30 days left with severity info, 2 to 7 days left with severity warning, and 1 day or fewer left, including already expired, with severity critical. For any number of days left, at most one of the three rules SHALL match a certificate. Installed rules SHALL be ordinary rules a user can edit, disable or delete. The pack listing SHALL state for each pack whether it is installed.

#### Scenario: Three weeks left
- **WHEN** the pack is installed and a certificate has 21 days left when rules are evaluated
- **THEN** the certificate SHALL have exactly one open finding, with severity info.

#### Scenario: Escalation
- **WHEN** a certificate with an open info finding reaches 7 days left and rules are evaluated
- **THEN** the info finding SHALL be resolved and one warning finding SHALL be open, and a notification SHALL be dispatched as for any new finding of that severity.

#### Scenario: Expired
- **WHEN** a certificate's `not_after` is in the past when rules are evaluated
- **THEN** it SHALL have exactly one open finding, with severity critical.

#### Scenario: Renewed
- **WHEN** a certificate with an open warning finding is renewed to 90 days left and rules are evaluated after the next sync
- **THEN** it SHALL have no open expiry finding.

#### Scenario: Unreachable target still tracked
- **WHEN** a TLS probe target is unreachable and its last observed `not_after` is 3 days away
- **THEN** the warning rule SHALL match its entity.

#### Scenario: Pack not installed
- **WHEN** the pack has not been installed
- **THEN** no expiry finding SHALL be raised, and the pack listing SHALL report it as not installed.

### Requirement: Offer to install the pack
When a user who may install packs creates a TLS probe connector or enables the expiring-certificates widget and the Certificate expiry pack is not installed, the web app SHALL offer to install it in one action. Declining SHALL leave the connector or widget working without rules. The system SHALL NOT install the pack without that action.

#### Scenario: Prompt after creating a probe
- **WHEN** an admin creates the first TLS probe connector and the pack is not installed
- **THEN** the web app SHALL offer to install the pack, and accepting SHALL create its rules.

#### Scenario: Pack already installed
- **WHEN** a TLS probe connector is created and the pack is installed
- **THEN** no offer SHALL be shown.

#### Scenario: User without permission
- **WHEN** a user who may not install packs enables the widget
- **THEN** no offer SHALL be shown.

### Requirement: Expiring certificates listing
The system SHALL provide a listing of `certificate` entities that have a `not_after`, across all connectors the caller is permitted to view, ordered by `not_after` ascending, with a limit. Each item SHALL include the certificate's name, its connector, `not_after`, the days left computed as for the days-left operators, and whether the source reported it unreachable. Certificates on connectors the caller cannot view SHALL NOT appear. API keys restricted to connectors SHALL see only certificates of those connectors. The listing SHALL NOT depend on any rule or pack being installed.

#### Scenario: Soonest first
- **WHEN** a user who can view an NPM connector and a TLS probe connector requests the listing
- **THEN** certificates from both SHALL be returned in one list ordered by `not_after`, expired ones first.

#### Scenario: Hidden connector
- **WHEN** the caller has no grant on a connector that holds certificates
- **THEN** none of that connector's certificates SHALL be returned.

#### Scenario: No rules installed
- **WHEN** the Certificate expiry pack is not installed
- **THEN** the listing SHALL still return certificates with their days left.

### Requirement: Expiring certificates widget
The dashboard SHALL offer an "Expiring certificates" widget, disabled by default, that shows the soonest-expiring certificates from the listing with their days left, distinguishes expired, 7 days or fewer, 30 days or fewer and later, marks certificates whose source is unreachable, and links each to its entity. With no certificates it SHALL show an empty state that explains how to add a source.

#### Scenario: Widget enabled
- **WHEN** a user enables the widget and has viewable certificates
- **THEN** it SHALL list them soonest first with days left.

#### Scenario: Default state
- **WHEN** a user opens the dashboard without having changed widget settings
- **THEN** the widget SHALL NOT be shown.

#### Scenario: Nothing to show
- **WHEN** the widget is enabled and the caller can view no certificate with an expiry
- **THEN** it SHALL show the empty state.
