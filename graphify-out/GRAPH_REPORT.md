# Graph Report - WiseLabz  (2026-06-29)

## Corpus Check
- 169 files · ~98,171 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 694 nodes · 1020 edges · 34 communities detected
- Extraction: 64% EXTRACTED · 36% INFERRED · 0% AMBIGUOUS · INFERRED: 370 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 4|Community 4]]
- [[_COMMUNITY_Community 5|Community 5]]
- [[_COMMUNITY_Community 6|Community 6]]
- [[_COMMUNITY_Community 7|Community 7]]
- [[_COMMUNITY_Community 8|Community 8]]
- [[_COMMUNITY_Community 9|Community 9]]
- [[_COMMUNITY_Community 10|Community 10]]
- [[_COMMUNITY_Community 11|Community 11]]
- [[_COMMUNITY_Community 12|Community 12]]
- [[_COMMUNITY_Community 13|Community 13]]
- [[_COMMUNITY_Community 14|Community 14]]
- [[_COMMUNITY_Community 15|Community 15]]
- [[_COMMUNITY_Community 16|Community 16]]
- [[_COMMUNITY_Community 17|Community 17]]
- [[_COMMUNITY_Community 18|Community 18]]
- [[_COMMUNITY_Community 19|Community 19]]
- [[_COMMUNITY_Community 20|Community 20]]
- [[_COMMUNITY_Community 21|Community 21]]
- [[_COMMUNITY_Community 22|Community 22]]
- [[_COMMUNITY_Community 23|Community 23]]
- [[_COMMUNITY_Community 24|Community 24]]
- [[_COMMUNITY_Community 25|Community 25]]
- [[_COMMUNITY_Community 27|Community 27]]
- [[_COMMUNITY_Community 29|Community 29]]
- [[_COMMUNITY_Community 30|Community 30]]
- [[_COMMUNITY_Community 31|Community 31]]
- [[_COMMUNITY_Community 33|Community 33]]
- [[_COMMUNITY_Community 35|Community 35]]
- [[_COMMUNITY_Community 36|Community 36]]
- [[_COMMUNITY_Community 38|Community 38]]
- [[_COMMUNITY_Community 48|Community 48]]

## God Nodes (most connected - your core abstractions)
1. `Errorf()` - 127 edges
2. `Store` - 67 edges
3. `Error()` - 56 edges
4. `JSON()` - 41 edges
5. `New()` - 22 edges
6. `Handler` - 19 edges
7. `NoContent()` - 14 edges
8. `UserIDFromContext()` - 13 edges
9. `main()` - 12 edges
10. `NewService()` - 12 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `NewEngine()`  [INFERRED]
  backend/cmd/server/main.go → backend/internal/sync/engine.go
- `main()` --calls--> `NewHub()`  [INFERRED]
  backend/cmd/server/main.go → backend/internal/ws/ws.go
- `RequestID()` --calls--> `New()`  [INFERRED]
  backend/internal/api/middleware/requestid.go → backend/internal/store/store.go
- `buildDocDiff()` --calls--> `lineDiff()`  [INFERRED]
  web/src/lib/docdiffmodel.ts → web/src/lib/linediff.ts
- `buildDocDiff()` --calls--> `diffStats()`  [INFERRED]
  web/src/lib/docdiffmodel.ts → web/src/lib/linediff.ts

## Communities

### Community 0 - "Community 0"
Cohesion: 0.05
Nodes (24): Handler, Handler, readIP(), sanitizeSessions(), sanitizeUser(), setRefreshCookie(), submit(), UserIDFromContext() (+16 more)

### Community 1 - "Community 1"
Cohesion: 0.04
Nodes (18): Errorf(), MarshalConnectorConfig(), nilToStr(), nullStrToStr(), ParseConnectorConfig(), scanConnectors(), ConnectorRecord, Session (+10 more)

### Community 2 - "Community 2"
Cohesion: 0.06
Nodes (27): AISettings, AuthSettings, Config, applyEnvOverrides(), boolEnv(), intEnv(), Load(), TestLoadDefaults() (+19 more)

### Community 4 - "Community 4"
Cohesion: 0.11
Nodes (23): NewHandler(), Config, NewRouter(), contextKey, NewService(), TestElevationExpired(), TestElevationWrongAction(), TestExpiredAccessToken() (+15 more)

### Community 5 - "Community 5"
Cohesion: 0.08
Nodes (10): Provider, Registry, StubProvider, SuggestChunk, SuggestRequest, broadcastMsg, Client, Envelope (+2 more)

### Community 6 - "Community 6"
Cohesion: 0.08
Nodes (7): Register(), Connector, init(), Connector, init(), Connector, init()

### Community 7 - "Community 7"
Cohesion: 0.2
Nodes (12): Decrypt(), DeriveKey(), Encrypt(), TestDecryptTampered(), TestDecryptWrongKey(), TestDeriveKeyDeterministic(), TestDeriveKeyDifferent(), TestEncryptBadKeySize() (+4 more)

### Community 8 - "Community 8"
Cohesion: 0.13
Nodes (15): Factory, Get(), GetTypeSchema(), ListSchemas(), SchemaField, TypeSchema, Compare(), lineCount() (+7 more)

### Community 9 - "Community 9"
Cohesion: 0.19
Nodes (6): MockWebSocket, env(), heartbeat(), newId(), startTimeline(), syncProgress()

### Community 10 - "Community 10"
Cohesion: 0.2
Nodes (6): Claims, ElevationClaims, ElevationToken, newTokenID(), Service, TokenPair

### Community 11 - "Community 11"
Cohesion: 0.26
Nodes (6): Connector, init(), isDangerousIP(), newGuardedClient(), setHeaders(), validateCustomURL()

### Community 12 - "Community 12"
Cohesion: 0.22
Nodes (3): dashboardOverview(), minsAgo(), serviceSnapshot()

### Community 13 - "Community 13"
Cohesion: 0.18
Nodes (6): Logger(), Recoverer(), GetRequestID(), RequestID(), requestIDKey, responseWriter

### Community 14 - "Community 14"
Cohesion: 0.25
Nodes (4): buildCommands(), onKeyDown(), run(), registeredCommands()

### Community 15 - "Community 15"
Cohesion: 0.25
Nodes (3): setAccessToken(), apply(), clear()

### Community 16 - "Community 16"
Cohesion: 0.31
Nodes (6): buildDocDiff(), fold(), toUnits(), diffStats(), lineDiff(), wordDiff()

### Community 17 - "Community 17"
Cohesion: 0.39
Nodes (5): addSection(), moveSection(), removeSection(), update(), updateSection()

### Community 18 - "Community 18"
Cohesion: 0.36
Nodes (6): applyTokens(), makePalette(), presetOpts(), commit(), load(), tokensFor()

### Community 19 - "Community 19"
Cohesion: 0.29
Nodes (2): relativeTime(), cn()

### Community 20 - "Community 20"
Cohesion: 0.62
Nodes (6): getResponse(), handleRequest(), resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 21 - "Community 21"
Cohesion: 0.29
Nodes (2): OIDCClaims, OIDCProvider

### Community 22 - "Community 22"
Cohesion: 0.33
Nodes (3): enableMocks(), bootstrap(), installMockWebSocket()

### Community 23 - "Community 23"
Cohesion: 0.4
Nodes (2): findRoute(), rowSeverity()

### Community 24 - "Community 24"
Cohesion: 0.33
Nodes (3): Engine, GenerateResult, templateData

### Community 25 - "Community 25"
Cohesion: 0.6
Nodes (3): fillBody(), generatePreview(), renderTemplate()

### Community 27 - "Community 27"
Cohesion: 0.5
Nodes (3): useCanMutate(), useRole(), RoleGate()

### Community 29 - "Community 29"
Cohesion: 0.4
Nodes (1): Handler

### Community 30 - "Community 30"
Cohesion: 0.4
Nodes (4): DocRecord, DocVersionRecord, TemplateRecord, TemplateSectionRecord

### Community 31 - "Community 31"
Cohesion: 0.4
Nodes (3): Connector, ServiceSnapshot, SnapshotSection

### Community 33 - "Community 33"
Cohesion: 0.83
Nodes (3): close(), onKey(), reset()

### Community 35 - "Community 35"
Cohesion: 0.67
Nodes (2): handle(), jump()

### Community 36 - "Community 36"
Cohesion: 0.5
Nodes (2): runSync(), triggerMockSync()

### Community 38 - "Community 38"
Cohesion: 0.67
Nodes (2): apply(), css()

### Community 48 - "Community 48"
Cohesion: 0.67
Nodes (2): AlertRecord, ChangeRecord

## Knowledge Gaps
- **40 isolated node(s):** `contextKey`, `Claims`, `ElevationClaims`, `TokenPair`, `ElevationToken` (+35 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Community 19`** (7 nodes): `clockTime()`, `fullDate()`, `relativeTime()`, `cn()`, `TimeAgo()`, `TimeAgo.tsx`, `time.ts`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 21`** (7 nodes): `OIDCClaims`, `OIDCProvider`, `.AuthURL()`, `.Exchange()`, `.Initialize()`, `.IsInitialized()`, `oidc.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 23`** (6 nodes): `eventLabel()`, `findRoute()`, `rowSeverity()`, `setCell()`, `setRowSeverity()`, `EventRoutingTable.tsx`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 29`** (5 nodes): `handlers.go`, `Handler`, `.Health()`, `.Version()`, `NewHandler()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 35`** (4 nodes): `WebSocketProvider.tsx`, `handle()`, `jump()`, `WebSocketProvider()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 36`** (4 nodes): `runSync()`, `runSync.ts`, `triggerSync.ts`, `triggerMockSync()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 38`** (4 nodes): `apply()`, `css()`, `seed()`, `appearance.ts`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 48`** (3 nodes): `change.go`, `AlertRecord`, `ChangeRecord`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Errorf()` connect `Community 1` to `Community 0`, `Community 2`, `Community 5`, `Community 6`, `Community 7`, `Community 8`, `Community 10`, `Community 11`, `Community 21`, `Community 24`?**
  _High betweenness centrality (0.233) - this node is a cross-community bridge._
- **Why does `Error()` connect `Community 0` to `Community 1`, `Community 2`, `Community 4`, `Community 5`, `Community 7`, `Community 8`, `Community 13`?**
  _High betweenness centrality (0.067) - this node is a cross-community bridge._
- **Why does `Load()` connect `Community 2` to `Community 1`?**
  _High betweenness centrality (0.039) - this node is a cross-community bridge._
- **Are the 125 inferred relationships involving `Errorf()` (e.g. with `.IssuePair()` and `.IssueElevation()`) actually correct?**
  _`Errorf()` has 125 INFERRED edges - model-reasoned connections that need verification._
- **Are the 53 inferred relationships involving `Error()` (e.g. with `AuthMiddleware()` and `RequireRole()`) actually correct?**
  _`Error()` has 53 INFERRED edges - model-reasoned connections that need verification._
- **Are the 38 inferred relationships involving `JSON()` (e.g. with `.Create()` and `.Get()`) actually correct?**
  _`JSON()` has 38 INFERRED edges - model-reasoned connections that need verification._
- **Are the 21 inferred relationships involving `New()` (e.g. with `main()` and `newLogger()`) actually correct?**
  _`New()` has 21 INFERRED edges - model-reasoned connections that need verification._