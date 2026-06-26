# WiseLabz — v2 Backlog

Features explicitly deferred from v1 during frontend planning. Each replaces or
extends a v1 decision (see the frontend plan, §8).

## Deferred to v2

| Feature                                             | Context                                                                                                                                                                 |
|-----------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Soft lock on doc editing ("X is currently editing") | Prevents conflicts when multiple editors open the same doc simultaneously. Replaces the v1 last-write-wins + "newer version available" banner approach (decision §8.3). |
| SSE endpoint for AI suggestions                     | Alternative to WebSocket streaming if the `/ws` channel becomes too complex to multiplex. Evaluate after the v1 AI module is stable (decision §8.4).                    |
| Per-user layout with admin-defined default          | Extends v1 per-user dashboard layout persistence. Admin sets a default layout that individual users can override (decision §8.5).                                       |
