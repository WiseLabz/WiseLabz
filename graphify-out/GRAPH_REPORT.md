# Graph Report - WiseLabz  (2026-06-28)

## Corpus Check
- 68 files · ~45,386 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 192 nodes · 154 edges · 11 communities detected
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 12 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]
- [[_COMMUNITY_Community 5|Community 5]]
- [[_COMMUNITY_Community 7|Community 7]]
- [[_COMMUNITY_Community 9|Community 9]]
- [[_COMMUNITY_Community 10|Community 10]]
- [[_COMMUNITY_Community 11|Community 11]]
- [[_COMMUNITY_Community 13|Community 13]]
- [[_COMMUNITY_Community 19|Community 19]]

## God Nodes (most connected - your core abstractions)
1. `MockWebSocket` - 7 edges
2. `buildDocDiff()` - 5 edges
3. `handleRequest()` - 5 edges
4. `getResponse()` - 5 edges
5. `env()` - 4 edges
6. `close()` - 3 edges
7. `toUnits()` - 3 edges
8. `enableMocks()` - 3 edges
9. `heartbeat()` - 3 edges
10. `tokensFor()` - 3 edges

## Surprising Connections (you probably didn't know these)
- `buildDocDiff()` --calls--> `lineDiff()`  [INFERRED]
  web/src/lib/docdiffmodel.ts → web/src/lib/linediff.ts
- `buildDocDiff()` --calls--> `diffStats()`  [INFERRED]
  web/src/lib/docdiffmodel.ts → web/src/lib/linediff.ts
- `tokensFor()` --calls--> `makePalette()`  [INFERRED]
  web/src/store/theme.ts → web/src/theme.ts
- `load()` --calls--> `presetOpts()`  [INFERRED]
  web/src/store/theme.ts → web/src/theme.ts
- `commit()` --calls--> `applyTokens()`  [INFERRED]
  web/src/store/theme.ts → web/src/theme.ts

## Communities

### Community 1 - "Community 1"
Cohesion: 0.19
Nodes (6): MockWebSocket, env(), heartbeat(), newId(), startTimeline(), syncProgress()

### Community 2 - "Community 2"
Cohesion: 0.22
Nodes (2): dashboardOverview(), minsAgo()

### Community 3 - "Community 3"
Cohesion: 0.31
Nodes (6): buildDocDiff(), fold(), toUnits(), diffStats(), lineDiff(), wordDiff()

### Community 4 - "Community 4"
Cohesion: 0.36
Nodes (6): applyTokens(), makePalette(), presetOpts(), commit(), load(), tokensFor()

### Community 5 - "Community 5"
Cohesion: 0.62
Nodes (6): getResponse(), handleRequest(), resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 7 - "Community 7"
Cohesion: 0.33
Nodes (3): enableMocks(), bootstrap(), installMockWebSocket()

### Community 9 - "Community 9"
Cohesion: 0.5
Nodes (2): onKeyDown(), run()

### Community 10 - "Community 10"
Cohesion: 0.4
Nodes (3): Store, New(), TestNew()

### Community 11 - "Community 11"
Cohesion: 0.83
Nodes (3): close(), onKey(), reset()

### Community 13 - "Community 13"
Cohesion: 0.5
Nodes (2): runSync(), triggerMockSync()

### Community 19 - "Community 19"
Cohesion: 1.0
Nodes (2): useCanMutate(), useRole()

## Knowledge Gaps
- **1 isolated node(s):** `Store`
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Community 2`** (10 nodes): `alertPage()`, `changeDetail()`, `changePage()`, `dashboardOverview()`, `daysAgo()`, `hrsAgo()`, `minsAgo()`, `removalImpact()`, `svcDoc()`, `fixtures.ts`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 9`** (5 nodes): `buildCommands()`, `CommandPalette()`, `onKeyDown()`, `run()`, `CommandPalette.tsx`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 13`** (4 nodes): `runSync()`, `runSync.ts`, `triggerSync.ts`, `triggerMockSync()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 19`** (3 nodes): `useCanMutate()`, `useRole()`, `useRole.ts`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Are the 2 inferred relationships involving `buildDocDiff()` (e.g. with `lineDiff()` and `diffStats()`) actually correct?**
  _`buildDocDiff()` has 2 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Store` to the rest of the system?**
  _1 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.07 - nodes in this community are weakly interconnected._