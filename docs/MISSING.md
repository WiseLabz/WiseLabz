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

| Feature                                                    | Why deferred / context                                                                                                                                                                                                                                                                                                          |
|------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `POST /api/docs/generate` frontend                         | Backend & route fully wired. Not in OpenAPI spec — add `/docs/generate` POST to spec, regenerate orval client, then build UI (template picker + connector picker → "Generate" button). Handler calls `DocEngine.GenerateFromTemplate(templateId, connectorId)` — fills template placeholders with live connector snapshot data. |
| `POST /api/changes/{changeId}/ai-update` wiring & frontend | Backend handler (`changeH.AIUpdate`) fully implemented. Not in router. Frontend `ChangeDetailPage` has i18n labels + generated client but no UI button yet. Wire route in operator group, then add "AI Update" button to `ChangeDetailPage` that calls `postChangesChangeIdAiUpdate`.                                           |
| `GET /api/version` documented or used                      | Backend wired, unauthenticated, returns Go build info. Not in OpenAPI spec, no frontend client. `SystemPage` already displays version from `GET /api/system/info`. Either add to spec + generate client, or keep as undocumented ops endpoint.                                                                                  |
| Elevation on user/template destructive ops                 | `RequireElevation` wired for connector delete only. Frontend `StepUp`/`ConfirmDestructive` only used for connectors. Wire elevation on `DELETE /api/users/{id}`, `POST /api/users/{id}/reset-password`, `DELETE /api/templates/{id}` once frontend StepUp UI built for those flows.                                             |
| All-docs flat list w/ search (any authenticated user)      | `GET /api/docs` (flat list) handler kept — planned frontend later. Needs search, proper spec entry, and a screen. Open to viewers too, same tier as existing doc reads.                                                                                                                                                         |
| `http.Flusher` on `middleware.responseWriter`              | Logging middleware wraps `http.ResponseWriter` but only implements `Hijacker` (added for WebSocket). `Flusher` is not needed yet — no endpoints stream responses. Add `Flush()` method (type-assert underlying writer, delegate) when SSE, file export streaming, or AI token streaming is added.                               |
