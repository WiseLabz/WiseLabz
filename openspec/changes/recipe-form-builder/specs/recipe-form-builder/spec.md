# Spec Delta

## Purpose

Lets an administrator build and edit a custom connector recipe through structured form rows instead of raw YAML, while keeping the YAML text as the single thing that is stored, so both ways of editing stay interchangeable.

## ADDED Requirements

### Requirement: Two views of one recipe
The recipe field of a custom connector SHALL offer a Form view and a YAML view of the same recipe text. The user SHALL be able to switch between them at any time without saving, and an edit made in one view SHALL be visible in the other. The text shown in the YAML view SHALL be exactly what is sent when the recipe is tested or the connector is saved. The view switch SHALL be operable with the keyboard and SHALL expose its selected state to assistive technology.

#### Scenario: Form edit appears in YAML
- **WHEN** the user changes an endpoint's path in the Form view and switches to the YAML view
- **THEN** the YAML SHALL show the new path.

#### Scenario: YAML edit appears in the form
- **WHEN** the user adds an attribute in the YAML view and switches to the Form view
- **THEN** the form SHALL show a row for that attribute.

#### Scenario: Saving from the form
- **WHEN** the user edits the recipe only in the Form view and saves the connector
- **THEN** the connector SHALL be saved with the YAML text the YAML view shows.

#### Scenario: Empty recipe
- **WHEN** a custom connector has no recipe and the user opens the Form view
- **THEN** the form SHALL offer to start a recipe, and SHALL NOT add any recipe text until the user accepts.

### Requirement: Form coverage
The Form view SHALL provide a row or field for every element of the recipe format: category; authentication mode, name and prefix; and for each endpoint its name, path, method, query parameters, headers, body, items path, pagination style with the fields that style uses, entity kind, the entity's name, external ID, hostname, IP address, MAC address and aliases mappings, attributes, and dependencies; and the recipe's own dependencies. For an attribute the form SHALL let the user choose its source (path, constant or template), its type, its value map entries and its default. The user SHALL be able to add, remove and reorder endpoints, and to add and remove attributes, value map entries, query parameters, headers and dependencies. Once recipes can declare actions, the form SHALL provide rows for entity actions and service actions with every field an action defines.

#### Scenario: Building a recipe without YAML
- **WHEN** the user starts from an empty recipe and fills in only form rows for one endpoint with a paginated list, three attributes and one dependency
- **THEN** the resulting text SHALL be a recipe the server accepts.

#### Scenario: Pagination fields follow the style
- **WHEN** the user selects the cursor pagination style
- **THEN** the form SHALL show the fields that style uses and SHALL NOT write fields of other styles.

#### Scenario: Attribute with a value map
- **WHEN** the user gives an attribute a path, the type string, two value map entries and a default
- **THEN** the YAML SHALL contain that attribute with the path, type, both map entries and the default.

#### Scenario: Constant keeps its type
- **WHEN** the user enters a constant and marks it as a number
- **THEN** the YAML SHALL hold a number, not a quoted string.

#### Scenario: Action rows
- **WHEN** recipes can declare actions and the user adds a restart action to an endpoint's entity through the form
- **THEN** the YAML SHALL contain that action under the endpoint's entity with the method and path entered.

### Requirement: Hand-written YAML is preserved
An edit made in the Form view SHALL change only the part of the text that the edited row represents. Comments, blank lines, key order, quoting and indentation elsewhere in the text SHALL remain as they were. Opening the Form view and making no edit SHALL leave the text unchanged.

#### Scenario: No edit, no change
- **WHEN** a hand-written recipe with comments is opened in the Form view and the user switches back without editing
- **THEN** the text SHALL be identical, byte for byte.

#### Scenario: Comment survives an edit
- **WHEN** a recipe has a comment above an endpoint and the user changes that endpoint's items path in the form
- **THEN** the comment SHALL still be above the endpoint and only the items path line SHALL differ.

#### Scenario: Shipped examples round-trip
- **WHEN** each example recipe shipped with the documentation is opened in the Form view and closed without editing
- **THEN** its text SHALL be identical.

### Requirement: Text the form cannot represent
When the recipe text is not valid YAML, the Form view SHALL be unavailable and SHALL state the line and the reason, and the YAML view SHALL remain usable. The same SHALL apply to text that is valid YAML but is not a single mapping document, or that uses anchors, aliases or merge keys. When the text is a valid single document that contains keys the form has no row for, the Form view SHALL open, SHALL leave those keys and their values untouched in the text through every form edit, and SHALL indicate where they are and that they can be edited in the YAML view. The form SHALL never remove or rewrite content it does not represent.

#### Scenario: Syntax error
- **WHEN** the YAML has an unclosed bracket on line 7
- **THEN** the Form view SHALL be unavailable with a message naming line 7, and the YAML view SHALL still be editable.

#### Scenario: Unknown key is kept
- **WHEN** an endpoint contains a key the form has no row for, and the user edits another field of that endpoint in the form
- **THEN** the unknown key and its value SHALL still be in the text, and the endpoint's row SHALL indicate that it has content editable in YAML only.

#### Scenario: Removing a row with unknown content
- **WHEN** the user removes an endpoint that contains a key the form has no row for
- **THEN** the form SHALL say that the endpoint has content it does not show before removing it.

#### Scenario: Anchors
- **WHEN** the recipe uses a YAML anchor and alias
- **THEN** the Form view SHALL be unavailable with an explanation, and the YAML view SHALL remain usable.

### Requirement: Server validation shown in place
The server SHALL remain the only judge of whether a recipe is valid; the form SHALL enforce only what its own inputs imply, such as required fields and fixed option lists. When testing or saving returns validation errors with locations in the recipe, the Form view SHALL mark the row and field each error refers to and show its message there, and the YAML view SHALL mark the corresponding line. An error whose location has no row SHALL still be shown in the list of errors. Focus SHALL move to the first error after a failed save.

#### Scenario: Error on a nested field
- **WHEN** saving returns an error located at the second endpoint's entity external ID
- **THEN** the Form view SHALL open that endpoint, mark its external ID field and show the message there.

#### Scenario: Same error in YAML
- **WHEN** the user switches to the YAML view while that error is present
- **THEN** the line holding the second endpoint's external ID mapping SHALL be marked with the message.

#### Scenario: Error without a row
- **WHEN** an error is located at a key the form has no row for
- **THEN** the error SHALL appear in the error list with its location.

#### Scenario: Errors clear on change
- **WHEN** the user edits the recipe after a failed save
- **THEN** the marks from that save SHALL no longer be shown as current once the recipe is tested or saved again.

### Requirement: Testing from either view
The recipe test SHALL be available below both views and SHALL test the current text, saved or not. Per-endpoint results and errors SHALL be attributable to the endpoint they belong to in the Form view.

#### Scenario: Test from the form
- **WHEN** the user edits an endpoint in the Form view and runs the test without saving
- **THEN** the test SHALL use the edited recipe.

#### Scenario: Failing endpoint
- **WHEN** the test reports an error for one endpoint
- **THEN** that endpoint's row in the Form view SHALL show that it failed.

### Requirement: YAML editor
The YAML view SHALL be a code editor with YAML syntax highlighting and line numbers, usable with the keyboard, and SHALL keep the existing protection against leaving the page with unsaved recipe changes.

#### Scenario: Highlighting
- **WHEN** the YAML view shows a recipe
- **THEN** keys, strings, numbers and comments SHALL be visually distinct.

#### Scenario: Unsaved changes
- **WHEN** the user has edited the recipe in either view and tries to leave the page
- **THEN** the existing unsaved-changes warning SHALL appear.

### Requirement: Editing rights and languages
The Form view SHALL follow the same rule as the recipe text for who may edit: where the recipe is read-only for a user, the form SHALL be read-only too. All text of the builder SHALL be available in English and Brazilian Portuguese.

#### Scenario: User who may not edit the recipe
- **WHEN** a user who is not allowed to change the recipe opens the connector
- **THEN** the Form view SHALL NOT let them change it.

#### Scenario: Portuguese
- **WHEN** the interface language is Brazilian Portuguese
- **THEN** the builder's labels and messages SHALL be in Portuguese.
