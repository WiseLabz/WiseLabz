# Graph Report - WiseLabz  (2026-06-29)

## Corpus Check
- 108 files · ~70,933 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 288 nodes · 225 edges · 18 communities detected
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 17 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]
- [[_COMMUNITY_Community 5|Community 5]]
- [[_COMMUNITY_Community 6|Community 6]]
- [[_COMMUNITY_Community 7|Community 7]]
- [[_COMMUNITY_Community 8|Community 8]]
- [[_COMMUNITY_Community 9|Community 9]]
- [[_COMMUNITY_Community 10|Community 10]]
- [[_COMMUNITY_Community 11|Community 11]]
- [[_COMMUNITY_Community 12|Community 12]]
- [[_COMMUNITY_Community 14|Community 14]]
- [[_COMMUNITY_Community 16|Community 16]]
- [[_COMMUNITY_Community 18|Community 18]]
- [[_COMMUNITY_Community 20|Community 20]]
- [[_COMMUNITY_Community 21|Community 21]]
- [[_COMMUNITY_Community 23|Community 23]]

## God Nodes (most connected - your core abstractions)
1. `MockWebSocket` - 7 edges
2. `buildDocDiff()` - 5 edges
3. `update()` - 5 edges
4. `handleRequest()` - 5 edges
5. `getResponse()` - 5 edges
6. `env()` - 4 edges
7. `renderTemplate()` - 3 edges
8. `minsAgo()` - 3 edges
9. `close()` - 3 edges
10. `useCanMutate()` - 3 edges

## Surprising Connections (you probably didn't know these)
- `buildDocDiff()` --calls--> `lineDiff()`  [INFERRED]
  web/src/lib/docdiffmodel.ts → web/src/lib/linediff.ts
- `buildDocDiff()` --calls--> `diffStats()`  [INFERRED]
  web/src/lib/docdiffmodel.ts → web/src/lib/linediff.ts
- `makePalette()` --calls--> `tokensFor()`  [INFERRED]
  web/src/theme.ts → web/src/store/theme.ts
- `presetOpts()` --calls--> `load()`  [INFERRED]
  web/src/theme.ts → web/src/store/theme.ts
- `applyTokens()` --calls--> `commit()`  [INFERRED]
  web/src/theme.ts → web/src/store/theme.ts

## Communities

### Community 1 - "Community 1"
Cohesion: 0.19
Nodes (6): MockWebSocket, env(), heartbeat(), newId(), startTimeline(), syncProgress()

### Community 2 - "Community 2"
Cohesion: 0.22
Nodes (3): dashboardOverview(), minsAgo(), serviceSnapshot()

### Community 3 - "Community 3"
Cohesion: 0.25
Nodes (4): buildCommands(), onKeyDown(), run(), registeredCommands()

### Community 4 - "Community 4"
Cohesion: 0.31
Nodes (6): buildDocDiff(), fold(), toUnits(), diffStats(), lineDiff(), wordDiff()

### Community 5 - "Community 5"
Cohesion: 0.33
Nodes (5): addSection(), moveSection(), removeSection(), update(), updateSection()

### Community 6 - "Community 6"
Cohesion: 0.25
Nodes (3): setAccessToken(), apply(), clear()

### Community 7 - "Community 7"
Cohesion: 0.36
Nodes (6): applyTokens(), makePalette(), presetOpts(), commit(), load(), tokensFor()

### Community 8 - "Community 8"
Cohesion: 0.29
Nodes (2): relativeTime(), cn()

### Community 9 - "Community 9"
Cohesion: 0.62
Nodes (6): getResponse(), handleRequest(), resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 10 - "Community 10"
Cohesion: 0.33
Nodes (3): enableMocks(), bootstrap(), installMockWebSocket()

### Community 11 - "Community 11"
Cohesion: 0.4
Nodes (2): findRoute(), rowSeverity()

### Community 12 - "Community 12"
Cohesion: 0.6
Nodes (3): fillBody(), generatePreview(), renderTemplate()

### Community 14 - "Community 14"
Cohesion: 0.5
Nodes (3): useCanMutate(), useRole(), RoleGate()

### Community 16 - "Community 16"
Cohesion: 0.4
Nodes (3): Store, New(), TestNew()

### Community 18 - "Community 18"
Cohesion: 0.83
Nodes (3): close(), onKey(), reset()

### Community 20 - "Community 20"
Cohesion: 0.5
Nodes (2): runSync(), triggerMockSync()

### Community 21 - "Community 21"
Cohesion: 0.67
Nodes (2): handle(), jump()

### Community 23 - "Community 23"
Cohesion: 0.67
Nodes (2): apply(), css()

## Knowledge Gaps
- **1 isolated node(s):** `Store`
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Community 8`** (7 nodes): `clockTime()`, `fullDate()`, `relativeTime()`, `cn()`, `TimeAgo()`, `TimeAgo.tsx`, `time.ts`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 11`** (6 nodes): `eventLabel()`, `findRoute()`, `rowSeverity()`, `setCell()`, `setRowSeverity()`, `EventRoutingTable.tsx`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 20`** (4 nodes): `runSync()`, `runSync.ts`, `triggerSync.ts`, `triggerMockSync()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 21`** (4 nodes): `WebSocketProvider.tsx`, `handle()`, `jump()`, `WebSocketProvider()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 23`** (4 nodes): `apply()`, `css()`, `seed()`, `appearance.ts`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Are the 2 inferred relationships involving `buildDocDiff()` (e.g. with `lineDiff()` and `diffStats()`) actually correct?**
  _`buildDocDiff()` has 2 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Store` to the rest of the system?**
  _1 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.07 - nodes in this community are weakly interconnected._