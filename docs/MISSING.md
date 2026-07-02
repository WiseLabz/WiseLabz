# MISSING — deferred & future frontend features

Anything raised but intentionally **not** in V1 lands here instead of expanding scope.
When a new idea comes up mid-build that isn't already planned, add a row rather than
growing the current phase. Promote to a real plan when it's time.

## Deferred from V1 (decided during planning)

| Feature                                                            | Why deferred / context                                                                                              |
|--------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------|
| Phone-grade responsive (<768px) on dense surfaces                  | V1 is desktop-first, mobile-tolerable to ~768px. Diffs/tables/dashboard-grid reflow for phones is a separate pass.  |
| Topbar notification-center (bell + dropdown of unread)             | V1 surfaces live events via toasts + activity feed + dashboard motion. A dedicated notification center is additive. |
| Soft-lock on concurrent doc editing ("X is editing")               | V1 uses last-write-wins + "newer version available" banner. Soft-lock needs presence signalling.                    |
| SSE endpoint for AI suggestions                                    | V1 AI is a batched single-request review-diff (no streaming). SSE only matters if/when streaming is reintroduced.   |
| AI-suggestion token *streaming* (inline live paint)                | Dropped deliberately for the cheaper batched review-diff pattern. Revisit if real-time feel is wanted.              |
| Per-user dashboard layout with admin-defined default               | V1 is per-user layout only. Admin default-then-override is a v2 extension.                                          |
| Lab-mutating manager ops (service start/stop/restart, config push) | Out of v1 manager scope per PRODUCT.md; gated on the permission/confirmation model.                                 |

## Suggested-later (raised in build, not yet planned)

| Feature                                               | Why deferred / context                                                                                                                                                                                                                                                                                                                        |
|-------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| All-docs flat list w/ search (any authenticated user) | Requested before V1. `GET /api/docs` (flat list) was dead code (no spec entry, no caller, tree nav covered browsing) and deleted this session — needs rebuilding as a real feature with search, proper spec entry, and a screen. Open to viewers too, same tier as existing doc reads. Scoped as medium/big, see `docs/V1_AUDIT_FINDINGS.md`. |
| `http.Flusher` on `middleware.responseWriter`         | Logging middleware wraps `http.ResponseWriter` but only implements `Hijacker` (added for WebSocket). `Flusher` is not needed yet — no endpoints stream responses. Add `Flush()` method (type-assert underlying writer, delegate) when SSE, file export streaming, or AI token streaming is added.                                             |
