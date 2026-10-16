# Governed MCP — Generated Engineering Design Document

**Evidence commit:** `1472c9cef99c74d315e71136a5e461572bbb0404`  
**Method:** code-derived review using the `codebase-to-edd` evidence discipline  
**Generated:** 2026-10-17

This document describes the implementation at the evidence commit. `PRD.md`, `EDD.md` and `PROTOCOL.md` are design inputs for comparison, not evidence for the findings below.

## 1. Executive system description

**KNOWN** — `governed-mcp` is a Go command-line adapter with one executable entry point at `cmd/governed-mcp/main.go` and one material reusable package at `internal/mcp`.

**KNOWN** — the process reads line-delimited JSON-RPC 2.0 from stdin, writes protocol responses to stdout and writes audit records to stderr.

**KNOWN** — the reusable core exposes configured tools onto an existing command API. It does not contain approval workflow or business-authorisation state.

## 2. Evidence basis

Evidence inspected at the pinned commit:

- `cmd/governed-mcp/main.go` — CLI, process lifecycle, stream routing and exit behaviour;
- `internal/mcp/mcp.go` — protocol state, request validation, tool dispatch, HTTP adapter and failure mapping;
- `internal/mcp/mcp_test.go` — executable behavioural evidence;
- `.github/workflows/test.yml` — CI contract;
- `examples/bafflewick/` — executable acceptance fixtures;
- `go.mod` — language/dependency boundary;
- `VERSION` — executable version contract.

## 3. Repository/runtime map

```text
stdin JSON-RPC
     |
     v
cmd/governed-mcp
     |
     v
internal/mcp.Server
  |          |
  |          +--> audit record returned to CLI -> stderr
  |
  +--> CommandClient -> HTTPClient -> existing application API
     |
     v
stdout JSON-RPC
```

**KNOWN** — the CLI is thin: it parses configuration, owns stdio, supplies the production HTTP client and delegates protocol behaviour to `internal/mcp.Server`.

## 4. Persistent and transient state

**KNOWN** — `Server.Era` is mutable in-memory connection state. Values observed in code are empty, `modern` and `legacy`.

**KNOWN** — there is no database, file-backed runtime state, cache or queue in the component.

**INFERRED** — one process instance is intended to own one stdio MCP connection because the protocol era is stored on the `Server` instance used by the input loop.

## 5. Modern protocol path

**KNOWN** — `server/discover` calls `requestMeta`, then `validateModernEnvelope`.

**KNOWN** — the modern envelope requires:

- `io.modelcontextprotocol/protocolVersion` equal to `2026-07-28`;
- object-valued `io.modelcontextprotocol/clientCapabilities`;
- when supplied, `io.modelcontextprotocol/clientInfo` must be an object with non-empty `name` and `version`.

**KNOWN** — successful discovery sets `Server.Era` to `modern` and returns both supported revisions, a tools capability, instructions, `ttlMs: 0`, `cacheScope: private`, and `resultType: complete`.

**KNOWN** — `stampModern` adds top-level response metadata under `io.modelcontextprotocol/serverInfo` with name `governed-mcp` and version `0.1.0`.

## 6. Legacy protocol path

**KNOWN** — `initialize` unmarshals `protocolVersion` and accepts only `2025-11-25`.

**KNOWN** — successful legacy initialisation sets `Server.Era` to `legacy` and returns a legacy-shaped result containing `protocolVersion`, `serverInfo`, `capabilities` and `instructions`.

**KNOWN** — the legacy path does not stamp the modern top-level response `_meta`.

## 7. Era integrity

**KNOWN** — modern discovery is refused when `Server.Era` is already `legacy`.

**KNOWN** — legacy initialise is refused when `Server.Era` is already `modern`.

**KNOWN** — `tools/list` and `tools/call` call `validateEstablishedRequest`; requests are refused when no era has been established.

**KNOWN** — every subsequent modern request must again carry a valid modern envelope.

## 8. Tool exposure

**KNOWN** — tools are loaded from `Config.Tools`; there is no dynamic code or network discovery of tools.

**KNOWN** — `tools/list` copies and sorts tools by `Name` before rendering them.

**KNOWN** — rendered tool entries contain `name`, `description` and an object `inputSchema`.

**KNOWN** — modern `tools/list` adds `ttlMs: 0`, `cacheScope: private`, `resultType: complete` and server metadata.

## 9. Tool-call governance boundary

**KNOWN** — `tools/call` requires a parsed tool name plus host-supplied `jonno.garage/actor` and `jonno.garage/human` from request `_meta`.

**KNOWN** — an unknown tool or actor not present in a non-empty `AllowedActors` list is rejected before `CommandClient.Do` is called.

**KNOWN** — a `read` tool forwards no `command` field.

**KNOWN** — a `proposal` tool adds `command: propose` to the downstream payload.

**KNOWN** — there is no tool mode that creates a direct mutation command.

## 10. Downstream interface

**KNOWN** — the reusable dependency is:

```go
type CommandClient interface {
    Do(context.Context, string, string, any) (json.RawMessage, error)
}
```

**KNOWN** — the production `HTTPClient`:

- marshals the downstream payload as JSON;
- sends to configured endpoint plus tool path;
- sets `Content-Type: application/json`;
- optionally sets a bearer `Authorization` header;
- defaults to a ten-second `http.Client` timeout when no client is supplied;
- limits the response body read to 1 MiB;
- treats non-2xx responses as errors.

## 11. Failure semantics

**KNOWN** — invalid JSON-RPC version returns `-32600`.

**KNOWN** — malformed params/modern metadata return `-32602`.

**KNOWN** — unsupported protocol revision and attempted era switch use `-32022`.

**KNOWN** — unknown methods return `-32601`; unknown or actor-disallowed tools use `-32001`; missing downstream client uses `-32603`.

**KNOWN** — a downstream error after a valid tool dispatch does not become a JSON-RPC framing error. `callToolResult` returns text content and `isError: true`.

**KNOWN** — there is no generic retry loop or failover path.

## 12. Audit behaviour

**KNOWN** — `Server.Handle` creates and returns an `Audit` value for each handled request.

**KNOWN** — audit fields include time, method, tool, actor, human, protocol, era, outcome and detail.

**KNOWN** — the CLI serialises that audit value to stderr for each successfully parsed JSON-RPC request; JSON parse failures also create a rejected audit record in the CLI.

**UNKNOWN** — durable collection, retention, tamper protection and operational monitoring of stderr are outside this repository.

## 13. Configuration and secret handling

**KNOWN** — `ValidateConfig` requires a non-empty endpoint, unique non-empty tool names, non-empty paths and tool modes restricted to `read` or `proposal`.

**KNOWN** — the CLI reads the optional bearer value from `GOVERNED_MCP_BEARER` and passes it into `HTTPClient`.

**KNOWN** — no code writes the bearer value into response or audit structs.

## 14. Build and dependency boundary

**KNOWN** — `go.mod` declares Go `1.26` and no third-party module dependencies.

**KNOWN** — the implementation imports only Go standard-library packages plus its own internal package.

**KNOWN** — CI runs formatting, vet, race tests, native build, executable version assertion, a `linux/arm64` cross-build and Bafflewick modern-session acceptance.

## 15. Test evidence

Named behavioural tests at the evidence commit include:

- `TestModernDiscoveryContract`;
- `TestLegacyInitializeContract`;
- `TestUnsupportedProtocolRejected`;
- `TestModernEnvelopeRequiredOnEveryRequest`;
- `TestToolsListDeterministicAndCacheable`;
- `TestProposalBoundary`;
- `TestReadToolDoesNotInventMutationCommand`;
- `TestUnknownAndDisallowedRefusedWithoutDownstream`;
- `TestHumanMetadataRequired`;
- `TestMalformedParamsRefusedWithoutDownstream`;
- `TestDownstreamFailureIsToolError`;
- `TestEraCannotSwitchMidConnection`;
- `TestValidateConfig`.

## 16. Known / inferred / unknown

| Classification | Finding |
|---|---|
| KNOWN | tools-only JSON-RPC stdio adapter |
| KNOWN | one explicit modern/legacy era held in process memory |
| KNOWN | modern request envelope validated on discovery and subsequent requests |
| KNOWN | read and proposal modes are distinct before downstream I/O |
| KNOWN | host actor/human attribution is required before downstream tool calls |
| KNOWN | no third-party Go modules |
| KNOWN | no durable store, retry queue, Streamable HTTP or OAuth implementation |
| INFERRED | `CommandClient` exists to keep the application API replaceable/testable without widening protocol logic |
| UNKNOWN | production throughput, latency and memory use on constrained hardware |
| UNKNOWN | correctness of the consuming application's own authorisation rules |
| UNKNOWN | durability/tamper-resistance of audit capture outside stderr |
| UNKNOWN | interoperability with MCP features not implemented by this tools-only subset |

## 17. Evidence trace

| Property recovered from code | Primary source | Executable evidence |
|---|---|---|
| modern discovery + server identity/cache hints | `Server.Handle`, `stampModern` | `TestModernDiscoveryContract` |
| legacy initialise | `Server.Handle` | `TestLegacyInitializeContract` |
| unsupported revision `-32022` | `protocolVersionError` | `TestUnsupportedProtocolRejected` |
| modern envelope on subsequent requests | `validateEstablishedRequest` | `TestModernEnvelopeRequiredOnEveryRequest` |
| one era per connection | `Server.Era`, discover/initialize guards | `TestEraCannotSwitchMidConnection` |
| deterministic tool list | sort in `tools/list` | `TestToolsListDeterministicAndCacheable` |
| proposal translation | `payload["command"] = "propose"` | `TestProposalBoundary` |
| read preservation | absence of command for read mode | `TestReadToolDoesNotInventMutationCommand` |
| scope before I/O | `find`, `allowed` before client call | `TestUnknownAndDisallowedRefusedWithoutDownstream` |
| host attribution before I/O | call metadata extraction | `TestHumanMetadataRequired` |
| downstream error as tool error | `callToolResult` | `TestDownstreamFailureIsToolError` |
| configuration contract | `ValidateConfig` | `TestValidateConfig` |
| Go/race/ARM64/acceptance | CI workflow | `.github/workflows/test.yml` |

The code supports the declared narrow adapter boundary. It does **not** provide evidence for a full MCP platform, identity system, approval engine, durable audit ledger or consuming-application authorisation model.
