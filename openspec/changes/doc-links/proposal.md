## Why

Docs cannot reference each other or inventory entities except through hand-typed `/docs/<id>` links, and nothing shows what points at a doc or entity (#520).

## What Changes

- `[[title]]`, `[[title|label]]` and `[[kind:ref]]` typed in a doc are resolved on save into standard Markdown links; ambiguous or unmatched text stays and a warning is returned.
- A `doc_links` index, maintained by every store writer of doc content, backs backlink endpoints for docs and entities.
- The editor completes `[[`; the doc page and entity page show a "Referenced by" panel; internal links render as router links.

## Impact

New migration `000069_doc_links`, package `backend/internal/doclink`, store, `api/docs`, entities API, OpenAPI, web docs editor and Markdown renderer, en and pt-BR strings. Closes #520.
