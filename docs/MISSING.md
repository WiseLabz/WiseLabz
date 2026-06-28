# MISSING — deferred & future frontend features

Anything raised but intentionally **not** in V1 lands here instead of expanding scope.
When a new idea comes up mid-build that isn't already planned, add a row rather than
growing the current phase. Promote to a real plan when it's time.

## Deferred from V1 (decided during planning)

| Feature | Why deferred / context |
|---|---|
| Phone-grade responsive (<768px) on dense surfaces | V1 is desktop-first, mobile-tolerable to ~768px. Diffs/tables/dashboard-grid reflow for phones is a separate pass. |
| Topbar notification-center (bell + dropdown of unread) | V1 surfaces live events via toasts + activity feed + dashboard motion. A dedicated notification center is additive. |
| Soft-lock on concurrent doc editing ("X is editing") | V1 uses last-write-wins + "newer version available" banner. Soft-lock needs presence signalling. |
| SSE endpoint for AI suggestions | V1 AI is a batched single-request review-diff (no streaming). SSE only matters if/when streaming is reintroduced. |
| AI-suggestion token *streaming* (inline live paint) | Dropped deliberately for the cheaper batched review-diff pattern. Revisit if real-time feel is wanted. |
| Per-user dashboard layout with admin-defined default | V1 is per-user layout only. Admin default-then-override is a v2 extension. |
| Lab-mutating manager ops (service start/stop/restart, config push) | Out of v1 manager scope per PRODUCT.md; gated on the permission/confirmation model. |

## Suggested-later (raised in build, not yet planned)

_Empty — add rows as ideas surface._
