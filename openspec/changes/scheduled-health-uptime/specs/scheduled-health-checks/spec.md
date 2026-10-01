# Spec Delta

## Purpose

Ensures connector health history is recorded continuously rather than only on manual checks, so status and uptime reflect reality.

## ADDED Requirements

### Requirement: Scheduled health checks
The system SHALL run a health check for every enabled connector on a configurable cron schedule (default every 60 seconds), record each result, and update the connector's status and status message exactly as a manual check does.

#### Scenario: Periodic recording
- **WHEN** the schedule fires
- **THEN** each enabled connector gets a new health history row and updated status

#### Scenario: Maintenance window skipped
- **WHEN** a connector is in an active maintenance window
- **THEN** no check is run and no row is recorded for it

#### Scenario: Disabled connector skipped
- **WHEN** a connector is disabled
- **THEN** it is not checked

#### Scenario: Slow connector isolated
- **WHEN** one connector's check hangs
- **THEN** it times out without delaying other connectors' checks

#### Scenario: Invalid schedule
- **WHEN** `health.cron_expr` is not a valid cron expression
- **THEN** configuration validation fails at startup

### Requirement: Manual and scheduled parity
Manual and scheduled checks SHALL share the same classification, thresholds and recording behaviour.

#### Scenario: Same result
- **WHEN** a manual check and a scheduled check run against the same connector state
- **THEN** both yield the same status classification
