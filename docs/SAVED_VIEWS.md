# Saved Views

Lets a user save a named, reusable filter set on a list surface — services,
changes, or alerts — and re-apply it later without rebuilding the filter by
hand.

## Endpoints

- `GET /api/saved-views?surface=X` — list the calling user's saved views for
  one surface. `surface` is required and must be one of `services`,
  `changes`, `alerts` (400 otherwise).
- `POST /api/saved-views` — create a saved view: `{ surface, name, filters }`.
  `filters` is a JSON object; the backend stores it opaquely and never
  inspects its shape — each frontend list page owns what goes in it.
- `DELETE /api/saved-views/{id}` — delete one of the calling user's saved
  views. 403 if the view belongs to someone else, 404 if it doesn't exist.

There is no update endpoint: delete and recreate to change a saved view
(scope cut — see below).

## Scope

- **Personal, not shared.** Every endpoint is scoped to the caller's own
  `user_id` (`auth.UserIDFromContext`). Any authenticated role — viewer or
  operator — can save and manage their own views; this is not gated behind
  `operatorOnly` since it never touches another user's data.
- **No update endpoint.** Saving under a new name (or deleting and
  recreating) covers the v1 need; a `PUT` can be added later if editing a
  saved view in place turns out to matter.
- **Opaque filters.** The backend does not validate the *contents* of
  `filters` — only that it's a JSON object — so it can evolve per-surface on
  the frontend without a backend migration. It does validate `surface`
  against the fixed enum above.

## Frontend

Each of the three list pages (`ServicesPage`, `ChangesPage`, `AlertsPage`)
renders a "Views" dropdown (`web/src/components/views/SavedViewsMenu.tsx`)
next to its filter controls: apply a saved view, save the current filter
state under a name, or delete a saved view. The shape of `filters` is
per-page (e.g. `{ q }` for the services search box, `{ severity }` for the
changes/alerts severity filter) — the component round-trips whatever object
it's given through JSON, so adding a saved-views-aware filter to a future
surface only means passing a new `filters`/`onApply` pair.
