# BookStack pull

This package fetches a BookStack instance and converts its content into the existing staged documentation import preview. It requires BookStack **v25.07 or newer**: v24.12 introduced the portable ZIP format, while v25.07 added the ZIP export API endpoints used here ([BookStack v25.07 release notes](https://www.bookstackapp.com/blog/bookstack-release-v25-07/)). The token user needs BookStack’s “Access System API” role permission. The API token must be allowed to read the shelf, book, and page listings and export books as ZIP files. A missing export endpoint is reported with a version and token-permission hint.

## Imported content

- Shelves become parent documents, books become children of their shelf, and chapters and pages follow the hierarchy in each book's ZIP export. Books not listed under a shelf are imported at the root.
- If a book appears on multiple shelves, it is imported under the first shelf returned by the shelf listing and a warning identifies the other placement.
- Markdown content is used when present. ZIP-exported images and supported attachments are carried into the preview; BookStack references and same-site page links are rewritten where a target can be resolved. Unsupported Markdown attachments are skipped and reported by the shared import analyzer.
- Default import limits are 100 MiB for the downloaded/exported ZIP, 2,000 archive entries, 500 MiB expanded data, a 100:1 compression ratio, 5 MiB per note, and 25 MiB per attachment. Listing and archive entry counts are also bounded by the entry limit.

## Pull lifecycle and security

Only one pull can run at a time. A pull has a one-hour execution timeout and can be cancelled while fetching. Successful pulls produce the normal staged import preview; failed or cancelled pulls remove their partial staging data. Staged imports expire after one hour and are swept periodically. Pull job status is held in memory, so a server restart does not restore the job status or resume a fetch.

Starting a pull requires an instance administrator. The start route uses the instance's existing destructive-action step-up setting: it requires elevation when that setting is enabled. TLS certificate verification is enabled by default. The `skipTlsVerify` option turns verification off for that request; its value is included in the pull audit event. Outbound requests use the guarded address policy, which blocks loopback, link-local, and metadata addresses, and redirects are not followed.

The submitted token ID and secret are held only by the fetch source while the pull runs. They are not stored in staged metadata, job status, or audit events. The URL must be HTTP(S) and cannot include user information, a query, or a fragment.

## Current verification limits

Pages without Markdown, and shelf, book and chapter descriptions, are converted from HTML with `docimport/htmlmd`. Automated tests use mocked BookStack responses built from the upstream portable ZIP format; this behavior has not been verified against a real BookStack instance.
