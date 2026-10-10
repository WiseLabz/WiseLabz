# BookStack and Wiki.js pulls

This package fetches a BookStack instance and converts its content into the existing staged documentation import preview. It requires BookStack **v25.07 or newer**: v24.12 introduced the portable ZIP format, while v25.07 added the ZIP export API endpoints used here ([BookStack v25.07 release notes](https://www.bookstackapp.com/blog/bookstack-release-v25-07/)). The token user needs BookStack’s “Access System API” role permission. The API token must be allowed to read the shelf, book, and page listings and export books as ZIP files. A missing export endpoint is reported with a version and token-permission hint.

## Imported content

- Shelves become parent documents, books become children of their shelf, and chapters and pages follow the hierarchy in each book's ZIP export. Books not listed under a shelf are imported at the root.
- If a book appears on multiple shelves, it is imported under the first shelf returned by the shelf listing and a warning identifies the other placement.
- Markdown content is used when present. ZIP-exported images and supported attachments are carried into the preview; BookStack references and same-site page links are rewritten where a target can be resolved. Unsupported Markdown attachments are skipped and reported by the shared import analyzer.
- Default pull limits are 100 MiB for each downloaded/exported ZIP, 2,000 archive entries, 500 MiB expanded data, 5 MiB per note, and 25 MiB per attachment. Pull archives skip only the per-entry compression-ratio check because remote exports can be highly compressible; upload archives retain the 100:1 ratio limit. Listing and archive entry counts are also bounded by the entry limit.

## Pull lifecycle and security

Only one pull can run at a time. A pull has a one-hour execution timeout and can be cancelled while fetching. Successful pulls produce the normal staged import preview; failed or cancelled pulls remove their partial staging data. Staged imports expire after one hour and are swept periodically. Pull job status is held in memory, so a server restart does not restore the job status or resume a fetch.

Starting a pull requires an instance administrator. The start route uses the instance's existing destructive-action step-up setting: it requires elevation when that setting is enabled. TLS certificate verification is enabled by default. The `skipTlsVerify` option turns verification off for that request; its value is included in the pull audit event. Outbound requests use the guarded address policy, which blocks loopback, link-local, and metadata addresses, and redirects are not followed.

The submitted token ID and secret are held only by the fetch source while the pull runs. They are not stored in staged metadata, job status, or audit events. The URL must be HTTP(S) and cannot include user information, a query, or a fragment.

## Current verification limits

BookStack pages without Markdown, and shelf, book and chapter descriptions, are converted from HTML with `docimport/htmlmd`. Automated tests use mocked BookStack responses built from the upstream portable ZIP format; this behavior has not been verified against a real BookStack instance.

## Wiki.js pull

`WikiJS` pulls a Wiki.js 2.x instance over GraphQL (`POST <url>/graphql`) with an API key (bearer) that has the `read:pages`, `read:source` and `read:assets` permissions. The key is sent in the request's `tokenSecret`; there is no token ID. It lists pages (`pages.list`), reads each one (`pages.single`), walks asset folders (`assets.folders`, `assets.list`) and downloads each asset by its site path, then writes the same layout a Wiki.js storage export has: `<path>.md` or `<path>.html` with the metadata block, pages of non-default locales under `<locale>/`, and assets at their site path. Analysis then runs the Wiki.js export parser (`docimport.AnalyzeWikiJS`), so HTML conversion, AsciiDoc skipping, unpublished-page warnings and link rewriting are shared with the zip import. The most common locale is treated as the default and left unprefixed.

Publication state is taken from `pages.list` because `Page.isPublished` requires `write:pages` upstream. Pages with an unsupported content type or an unsafe path, assets over the attachment limit and assets that return 404 are skipped and reported as warnings. GraphQL errors are reported generically (the remote's messages are never echoed), and the same guards, limits, retry and credential rules as the BookStack pull apply. Fixtures come from the upstream v2.5 schema (`server/graph/schemas/page.graphql`, `asset.graphql`); this has not been verified against a real Wiki.js instance.
