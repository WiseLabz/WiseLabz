# Graph Report - WiseLabz  (2026-07-02)

## Corpus Check
- 174 files · ~95,870 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 710 nodes · 1050 edges · 33 communities detected
- Extraction: 63% EXTRACTED · 37% INFERRED · 0% AMBIGUOUS · INFERRED: 385 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
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
- [[_COMMUNITY_Community 32|Community 32]]
- [[_COMMUNITY_Community 34|Community 34]]
- [[_COMMUNITY_Community 36|Community 36]]
- [[_COMMUNITY_Community 37|Community 37]]
- [[_COMMUNITY_Community 47|Community 47]]

## God Nodes (most connected - your core abstractions)
1. `Errorf()` - 127 edges
2. `Store` - 66 edges
3. `Error()` - 57 edges
4. `JSON()` - 44 edges
5. `New()` - 25 edges
6. `Handler` - 19 edges
7. `NoContent()` - 14 edges
8. `main()` - 13 edges
9. `UserIDFromContext()` - 13 edges
10. `NewService()` - 12 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `NewEngine()`  [INFERRED]
  backend/cmd/server/main.go → backend/internal/sync/engine.go
- `main()` --calls--> `NewHub()`  [INFERRED]
  backend/cmd/server/main.go → backend/internal/ws/ws.go
- `main()` --calls--> `NewDispatcher()`  [INFERRED]
  backend/cmd/server/main.go → backend/internal/notifications/dispatcher.go
- `RequestID()` --calls--> `New()`  [INFERRED]
  backend/internal/api/middleware/requestid.go → backend/internal/store/store.go
- `buildDocDiff()` --calls--> `lineDiff()`  [INFERRED]
  web/src/lib/docdiffmodel.ts → web/src/lib/linediff.ts

## Communities

### Community 0 - "Community 0"
Cohesion: 0.05
Nodes (25): Handler, Handler, readIP(), sanitizeSessions(), sanitizeUser(), setRefreshCookie(), submit(), UserIDFromContext() (+17 more)

### Community 1 - "Community 1"
Cohesion: 0.04
Nodes (23): Engine, Errorf(), MarshalConnectorConfig(), nilToStr(), nullStrToStr(), ParseConnectorConfig(), scanConnectors(), ConnectorRecord (+15 more)

### Community 2 - "Community 2"
Cohesion: 0.1
Nodes (24): NewHandler(), Config, NewRouter(), spaHandler(), contextKey, NewService(), TestElevationExpired(), TestElevationWrongAction() (+16 more)

### Community 3 - "Community 3"
Cohesion: 0.09
Nodes (21): AISettings, AuthSettings, Config, applyEnvOverrides(), boolEnv(), intEnv(), Load(), TestLoadDefaults() (+13 more)

### Community 5 - "Community 5"
Cohesion: 0.08
Nodes (7): Register(), Connector, init(), Connector, init(), Connector, init()

### Community 6 - "Community 6"
Cohesion: 0.09
Nodes (9): Provider, StubProvider, SuggestChunk, SuggestRequest, broadcastMsg, Client, Envelope, Hub (+1 more)

### Community 7 - "Community 7"
Cohesion: 0.12
Nodes (14): Registry, Factory, Get(), SchemaField, TypeSchema, Compare(), lineCount(), severityForChange() (+6 more)

### Community 8 - "Community 8"
Cohesion: 0.12
Nodes (11): Dispatcher, generateID(), NewDispatcher(), pgPlaceholderDB, rewritePlaceholders(), TestRewritePlaceholders(), Session, User (+3 more)

### Community 9 - "Community 9"
Cohesion: 0.2
Nodes (12): Decrypt(), DeriveKey(), Encrypt(), TestDecryptTampered(), TestDecryptWrongKey(), TestDeriveKeyDeterministic(), TestDeriveKeyDifferent(), TestEncryptBadKeySize() (+4 more)

### Community 10 - "Community 10"
Cohesion: 0.19
Nodes (6): MockWebSocket, env(), heartbeat(), newId(), startTimeline(), syncProgress()

### Community 11 - "Community 11"
Cohesion: 0.16
Nodes (7): getAccessToken(), setAccessToken(), apply(), clear(), handle(), jump(), wsUrl()

### Community 12 - "Community 12"
Cohesion: 0.2
Nodes (6): Claims, ElevationClaims, ElevationToken, newTokenID(), Service, TokenPair

### Community 13 - "Community 13"
Cohesion: 0.26
Nodes (6): Connector, init(), isDangerousIP(), newGuardedClient(), setHeaders(), validateCustomURL()

### Community 14 - "Community 14"
Cohesion: 0.22
Nodes (3): dashboardOverview(), minsAgo(), serviceSnapshot()

### Community 15 - "Community 15"
Cohesion: 0.18
Nodes (6): Logger(), Recoverer(), GetRequestID(), RequestID(), requestIDKey, responseWriter

### Community 16 - "Community 16"
Cohesion: 0.25
Nodes (4): buildCommands(), onKeyDown(), run(), registeredCommands()

### Community 17 - "Community 17"
Cohesion: 0.31
Nodes (6): buildDocDiff(), fold(), toUnits(), diffStats(), lineDiff(), wordDiff()

### Community 18 - "Community 18"
Cohesion: 0.39
Nodes (5): addSection(), moveSection(), removeSection(), update(), updateSection()

### Community 19 - "Community 19"
Cohesion: 0.36
Nodes (6): applyTokens(), makePalette(), presetOpts(), commit(), load(), tokensFor()

### Community 20 - "Community 20"
Cohesion: 0.29
Nodes (2): relativeTime(), cn()

### Community 21 - "Community 21"
Cohesion: 0.62
Nodes (6): getResponse(), handleRequest(), resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 22 - "Community 22"
Cohesion: 0.29
Nodes (2): OIDCClaims, OIDCProvider

### Community 23 - "Community 23"
Cohesion: 0.33
Nodes (3): enableMocks(), bootstrap(), installMockWebSocket()

### Community 24 - "Community 24"
Cohesion: 0.4
Nodes (2): findRoute(), rowSeverity()

### Community 25 - "Community 25"
Cohesion: 0.6
Nodes (3): fillBody(), generatePreview(), renderTemplate()

### Community 27 - "Community 27"
Cohesion: 0.5
Nodes (3): useCanMutate(), useRole(), RoleGate()

### Community 29 - "Community 29"
Cohesion: 0.4
Nodes (4): DocRecord, DocVersionRecord, TemplateRecord, TemplateSectionRecord

### Community 30 - "Community 30"
Cohesion: 0.4
Nodes (3): Connector, ServiceSnapshot, SnapshotSection

### Community 32 - "Community 32"
Cohesion: 0.83
Nodes (3): close(), onKey(), reset()

### Community 34 - "Community 34"
Cohesion: 0.5
Nodes (2): runSync(), triggerMockSync()

### Community 36 - "Community 36"
Cohesion: 0.67
Nodes (2): apply(), css()

### Community 37 - "Community 37"
Cohesion: 0.5
Nodes (2): GenerateResult, templateData

### Community 47 - "Community 47"
Cohesion: 0.67
Nodes (2): AlertRecord, ChangeRecord

## Knowledge Gaps
- **40 isolated node(s):** `contextKey`, `Claims`, `ElevationClaims`, `TokenPair`, `ElevationToken` (+35 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Community 20`** (7 nodes): `clockTime()`, `fullDate()`, `relativeTime()`, `cn()`, `TimeAgo()`, `TimeAgo.tsx`, `time.ts`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 22`** (7 nodes): `OIDCClaims`, `OIDCProvider`, `.AuthURL()`, `.Exchange()`, `.Initialize()`, `.IsInitialized()`, `oidc.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 24`** (6 nodes): `eventLabel()`, `findRoute()`, `rowSeverity()`, `setCell()`, `setRowSeverity()`, `EventRoutingTable.tsx`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 34`** (4 nodes): `runSync()`, `runSync.ts`, `triggerSync.ts`, `triggerMockSync()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 36`** (4 nodes): `apply()`, `css()`, `seed()`, `appearance.ts`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 37`** (4 nodes): `engine.go`, `NewEngine()`, `GenerateResult`, `templateData`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 47`** (3 nodes): `change.go`, `AlertRecord`, `ChangeRecord`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Errorf()` connect `Community 1` to `Community 0`, `Community 3`, `Community 5`, `Community 7`, `Community 9`, `Community 12`, `Community 13`, `Community 22`?**
  _High betweenness centrality (0.235) - this node is a cross-community bridge._
- **Why does `Error()` connect `Community 0` to `Community 1`, `Community 2`, `Community 6`, `Community 7`, `Community 8`, `Community 9`, `Community 15`?**
  _High betweenness centrality (0.066) - this node is a cross-community bridge._
- **Why does `Load()` connect `Community 3` to `Community 1`?**
  _High betweenness centrality (0.040) - this node is a cross-community bridge._
- **Are the 125 inferred relationships involving `Errorf()` (e.g. with `.IssuePair()` and `.IssueElevation()`) actually correct?**
  _`Errorf()` has 125 INFERRED edges - model-reasoned connections that need verification._
- **Are the 54 inferred relationships involving `Error()` (e.g. with `AuthMiddleware()` and `RequireRole()`) actually correct?**
  _`Error()` has 54 INFERRED edges - model-reasoned connections that need verification._
- **Are the 41 inferred relationships involving `JSON()` (e.g. with `.List()` and `.Create()`) actually correct?**
  _`JSON()` has 41 INFERRED edges - model-reasoned connections that need verification._
- **Are the 24 inferred relationships involving `New()` (e.g. with `main()` and `newLogger()`) actually correct?**
  _`New()` has 24 INFERRED edges - model-reasoned connections that need verification._