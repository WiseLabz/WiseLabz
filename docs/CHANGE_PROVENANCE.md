# Change provenance

Document-format change details can include `provenance` entries. Each entry names
the changed snapshot path, the template section that rendered it, and the changed
line numbers in the head revision. The change-detail page exposes those line
numbers as links into the document diff.

The field is optional. Infrastructure-only diffs and older change records remain
unchanged.
