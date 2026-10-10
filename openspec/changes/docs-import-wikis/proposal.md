## Why

The docs import (#514) reads Obsidian vaults only. Labs also keep documentation in BookStack and Wiki.js (#611).

## What Changes

- Wiki.js 2.x storage-export zip import (`source=wikijs`), HTML converted to Markdown in Go.
- BookStack API pull (URL plus API token) and Wiki.js GraphQL pull as one-at-a-time background jobs with status polling and cancel, gated by instance admin plus elevation (`docs.import.pull`).
- A source picker in the import dialog; pt-BR strings for `docs.import.*`.

## Impact

`backend/internal/docimport` (+ `htmlmd`, job manager, sources), `api/docs/import.go`, elevation action list, OpenAPI, web import dialog. New dependency `github.com/JohannesKaufmann/html-to-markdown/v2`. Closes #611 (three PRs).
