# Spec Delta

## Purpose

Lets MCP clients search documentation and runbooks, trace paths through the lab topology, and read runbooks while respecting each caller's connector grants and API-key scope.

## ADDED Requirements

### Requirement: Full-text search tool
The MCP server SHALL provide a `search` tool performing keyword full-text search over docs and runbooks, available regardless of whether AI providers are configured, returning only items the caller may view.

#### Scenario: Search without AI configured
- **WHEN** a client calls `search` with a query and no AI/embedding provider is enabled
- **THEN** matching docs and runbooks are returned ranked by relevance

#### Scenario: Results respect viewing permissions
- **WHEN** the caller lacks viewer access to a doc that matches the query
- **THEN** that doc does not appear in results

#### Scenario: Index stays current
- **WHEN** a doc or runbook is created, edited or deleted
- **THEN** subsequent searches reflect the change

### Requirement: Topology path tool
The MCP server SHALL provide a `topology_path` tool returning a shortest path between two entities using persisted topology edges, and MUST NOT expose entities, or traverse through connectors, the caller cannot view.

#### Scenario: Path found
- **WHEN** two entities are connected through visible entities
- **THEN** the ordered entity path with connector and edge kinds is returned

#### Scenario: Hidden intermediate
- **WHEN** the only path passes through an entity of a connector the caller cannot view
- **THEN** the tool reports no path and reveals nothing about the hidden entity

#### Scenario: Edges refreshed on sync
- **WHEN** a connector sync completes
- **THEN** that connector's topology edges are rebuilt from its latest snapshot and declared dependencies

### Requirement: Runbook read tools
The MCP server SHALL provide `list_runbooks` and `get_runbook` tools. Steps referencing a connector the caller cannot access MUST be redacted.

#### Scenario: Redacted step
- **WHEN** a runbook step targets an inaccessible connector
- **THEN** the step is returned without connector details or entity reference

### Requirement: Scope-restricted API keys
Read-only or connector-limited API keys SHALL be honoured by every new tool.

#### Scenario: Connector-limited key
- **WHEN** a key limited to connector A calls `topology_path` or `get_runbook`
- **THEN** only data from connector A is visible
