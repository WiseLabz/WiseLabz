## Decisions

- Stored form is standard Markdown (`[text](/docs/<id>)`, `[text](/entities/<id>)`); `[[...]]` is input syntax only, resolved server-side on save.
- Bare `[[name]]`: doc by exact title (case-insensitive), then entity by display name. `[[kind:ref]]`: entity by kind and connector reference over active `entity_members`, following `merged_into`. Several matches or none: text kept, warning returned. A `#heading` suffix is dropped; default label is the title or display name at save time.
- Lookups run with the saver's visibility (`viewableDocWhere`, the grant filter used by `SearchEntities`).
- `doclink.Extract` and `doclink.Resolve` ignore fenced code, code spans, `![[...]]` embeds and `wl:gen` marker sections. The code-skipping scanner moves from `docimport/links.go` into `doclink`.
- Index sync happens in the store transaction of `CreateDoc`, `UpdateDocWithVersion` and `UpdateDoc`; startup runs an idempotent backfill that only fills `doc_links`.
- Backlink endpoints list only source docs the reader may view, never soft-deleted ones; the entity variant includes links to entities merged into it. Not exposed on share routes.
- Lab Book HTML drops `/entities/` link destinations; Markdown and Git export keep them. Wikilinks apply to docs only.
