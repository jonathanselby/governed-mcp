# Governed MCP — Intended vs Recovered Design

**Implementation evidence:** `1472c9cef99c74d315e71136a5e461572bbb0404`  
**Compared:** 2026-10-17

This comparison tests whether the hand-written V0.1 design survived implementation. `generated/EDD.md` is the recovered side of the comparison.

| Design claim | Recovered implementation | Result |
|---|---|---|
| one reusable job: governed MCP tools edge onto an existing command API | one CLI + one `internal/mcp` package; no application workflow engine | **MATCH** |
| Go 1.26 baseline | `go.mod` declares 1.26 | **MATCH** |
| standard-library implementation | no third-party modules/imports | **MATCH** |
| modern `2026-07-28` discovery | `server/discover` validates modern `_meta`, establishes modern era | **MATCH** |
| legacy `2025-11-25` compatibility | `initialize` accepts only the legacy revision | **MATCH** |
| one protocol era per stdio connection | `Server.Era` plus switch guards | **MATCH** |
| modern server identity | `stampModern` writes `io.modelcontextprotocol/serverInfo` | **MATCH** |
| conservative modern cache policy | discovery/list return `ttlMs: 0`, `cacheScope: private` | **MATCH** |
| unsupported revision is explicit | `protocolVersionError` uses `-32022` and reports requested/supported revisions | **MATCH** |
| tools are configured and deterministic | configuration list is copied and sorted by name | **MATCH** |
| read calls remain reads | read mode adds no downstream mutation command | **MATCH** |
| state-changing calls become proposals | proposal mode adds `command: propose` | **MATCH** |
| host supplies agent and human attribution | both values are read from request `_meta` and required before I/O | **MATCH** |
| tool/actor scope checked before I/O | unknown/disallowed paths return before `Client.Do` | **MATCH** |
| downstream failure remains visible | error becomes tool result with `isError: true` | **MATCH** |
| audit exists for every handled request | core returns `Audit`; CLI emits it to stderr, including parse-error audit | **MATCH** |
| application remains authority for approvals/business policy | no maker-checker, policy engine or approval state exists in code | **MATCH** |
| stdio only; not full MCP platform | no Streamable HTTP, OAuth, prompts/resources/subscriptions etc. | **MATCH — DELIBERATE LIMIT** |
| durable audit storage | no durable audit store exists | **NOT IMPLEMENTED — DELIBERATE LIMIT** |
| constrained-hardware runtime proof | ARM64 cross-build exists; runtime performance is unmeasured | **UNKNOWN / FUTURE BENCHMARK** |
| consuming application authorisation correctness | outside repository and test adapters | **UNKNOWN / EXTERNAL** |

## Result

The V0.1 implementation preserves the intended component boundary. The implementation is slightly more explicit than the abstract design in one useful respect: protocol era is represented as concrete process state and tested against mid-connection switching.

No material design mismatch was found. The remaining unknowns are deployment/runtime concerns rather than hidden implementation gaps.
