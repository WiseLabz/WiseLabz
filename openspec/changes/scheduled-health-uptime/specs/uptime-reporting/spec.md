# Spec Delta

## Purpose

Exposes and displays connector availability, MTTR and health history so operators can see reliability at a glance.

## ADDED Requirements

### Requirement: Maintenance-aware uptime
Availability and MTTR SHALL exclude maintenance-window spans; time inside a maintenance window MUST NOT count as downtime or as part of an outage.

#### Scenario: Outage during maintenance
- **WHEN** a connector is offline only while in a maintenance window
- **THEN** availability is unaffected and no outage is counted

### Requirement: Health history endpoint
`GET /connectors/{id}/uptime/history` SHALL return downsampled buckets of status and latency for 24h, 7d or 30d for viewers of the connector.

#### Scenario: Bucketed history
- **WHEN** a viewer requests `window=7d`
- **THEN** ordered buckets with worst status and average latency are returned

#### Scenario: Unauthorized
- **WHEN** the caller has no grant on the connector
- **THEN** 403/404 as with other connector routes

### Requirement: Fleet uptime endpoint
`GET /uptime` SHALL return per-connector availability for only the connectors the caller can view.

#### Scenario: Grant filtering
- **WHEN** the caller can view connectors A and B but not C
- **THEN** the response contains A and B only

### Requirement: Uptime UI
The service page SHALL show availability and MTTR for 24h/7d/30d and a status/latency sparkline; the dashboard SHALL offer a fleet uptime widget, disabled by default. Windows with no checks MUST render as "no data", not 0%.

#### Scenario: No data
- **WHEN** a window has zero checks
- **THEN** the UI shows "no data"

#### Scenario: Widget off by default
- **WHEN** an existing user's saved layout is loaded
- **THEN** the uptime widget is not shown until enabled
