# connector-categories Specification

## Purpose
Defines which categories a connector can belong to and how a connector's category is determined, so that grouping, templates and backups agree on one vocabulary.

## Requirements

### Requirement: Valid connector categories
The system SHALL accept exactly these connector categories: `virtualization`, `containers_paas`, `networking`, `dns`, `storage`, `monitoring`, `media` and `other`. A connector with any other category SHALL be rejected on create, update, declaration in configuration and backup import. Connectors that exist before this change SHALL keep their category.

#### Scenario: New category accepted
- **WHEN** a connector is created with category `media`
- **THEN** it SHALL be stored and listed with category `media`.

#### Scenario: Unknown category rejected
- **WHEN** a connector is created with category `gaming`
- **THEN** the request SHALL be rejected with a field error on the category.

#### Scenario: Backup with a new category
- **WHEN** a backup containing a connector with category `monitoring` is imported
- **THEN** the import SHALL succeed and the connector SHALL keep that category.

#### Scenario: Existing connectors unchanged
- **WHEN** the system is upgraded
- **THEN** every existing connector SHALL report the same category as before.

### Requirement: Category presentation
Every valid category SHALL have a translated label and an icon wherever connectors are grouped or filtered by category, and SHALL be selectable wherever a category can be chosen for a template.

#### Scenario: Grouped list
- **WHEN** a connector with category `storage` exists and connectors are shown grouped by category
- **THEN** it SHALL appear under a labelled `storage` group in English and Portuguese (Brazil).

### Requirement: Category of a recipe connector
A custom connector that has a recipe SHALL take its category from the recipe, whether it was created in the web app or declared in configuration. A category supplied alongside it that differs from the recipe SHALL be rejected. A custom connector without a recipe SHALL keep the category `virtualization`.

#### Scenario: Recipe sets the category
- **WHEN** a custom connector is saved with a recipe declaring category `media`
- **THEN** the connector SHALL be stored with category `media`.

#### Scenario: Recipe category changes
- **WHEN** the recipe of an existing custom connector is edited from category `other` to `monitoring`
- **THEN** the connector's category SHALL become `monitoring`.

#### Scenario: Declared connector with a recipe
- **WHEN** a custom connector with a recipe declaring category `storage` is declared in configuration and reconciled
- **THEN** the resulting connector SHALL have category `storage`.
