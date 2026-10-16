# Governed MCP — Engineering Design Document

**Version:** 0.1

## 1. Purpose

Governed MCP is a small protocol adapter that exposes a configured MCP tools surface over an existing application command API without duplicating the application's authorisation, workflow or approval state.

The governing design rule is:

> **The adapter exposes commands. It does not invent a second approval system.**

## 2. Engineering principles

V0.1 applies five constraints.

1. **One job.** Translate a narrow MCP tools edge onto an existing command API.
2. **Protocol state is explicit.** A stdio connection establishes one MCP era and does not switch mid-connection.
3. **Scope before I/O.** Protocol, attribution, tool and actor checks happen before downstream calls.
4. **Proposal before mutation.** A state-changing tool becomes an application proposal; the adapter does not approve it.
5. **Failure stays visible.** Downstream failure is returned as an MCP tool error, not hidden by retries or false success.

## 3. Non-goals

The component does not implement identity proofing, maker-checker, business authorisation, application workflow, Streamable HTTP, OAuth, prompts, resources, sampling, elicitation, subscriptions, task extensions, durable audit storage, queues, retries or agent planning.

These are boundaries, not unfinished placeholders.

## 4. Runtime component model

```text
trusted MCP client host
        |
        | JSON-RPC 2.0 / stdio
        v
+---------------------------+
| cmd/governed-mcp          |
| config + stream edge      |
+-------------+-------------+
              |
              v
+---------------------------+
| internal/mcp.Server       |
| protocol + tool boundary  |
+-------------+-------------+
              |
              | CommandClient
              v
+---------------------------+
| HTTPClient                |
| existing application API  |
+---------------------------+
```

The CLI owns process concerns. `internal/mcp` owns the reusable contract.

## 5. Core types

The implementation centres on these material types:

```go
type Tool struct {
    Name          string
    Description   string
    Mode          string   // read | proposal
    Path          string
    AllowedActors []string
}

type Config struct {
    Endpoint string
    Tools    []Tool
}

type Server struct {
    Config Config
    Client CommandClient
    Now    func() time.Time
    Era    string
}
```

`Server.Era` is connection state. It is deliberately small: empty, `modern`, or `legacy`.

## 6. Protocol state machine

```text
                  +----------------+
START ------------| era = empty    |
                  +--------+-------+
                           |
             +-------------+-------------+
             |                           |
     server/discover                 initialize
     valid 2026 envelope             2025-11-25
             |                           |
             v                           v
      +-------------+             +-------------+
      | modern      |             | legacy      |
      +-------------+             +-------------+
             |                           |
             +---- tools/list/call ------+

Attempted modern <-> legacy switch after establishment -> -32022
```

This models stdio as a connection-oriented transport rather than treating each line as an unrelated protocol request.

## 7. Modern MCP behaviour

`server/discover` establishes the modern era only when request `params._meta` contains:

- `io.modelcontextprotocol/protocolVersion = 2026-07-28`;
- object-valued `io.modelcontextprotocol/clientCapabilities`;
- optional structurally valid `io.modelcontextprotocol/clientInfo`.

Every subsequent modern request is validated against the same envelope rule.

Modern responses stamp server identity in top-level response `_meta` under `io.modelcontextprotocol/serverInfo`.

Cacheable modern results use the safest cache posture:

```text
ttlMs = 0
cacheScope = private
```

No cross-user cache guarantee is implied.

## 8. Legacy MCP behaviour

A legacy connection begins with `initialize` requesting `2025-11-25`. The result uses the legacy `serverInfo` field and does not add modern response `_meta`.

V0.1 intentionally supports only the latest handshake-era revision rather than implementing every historical MCP revision.

## 9. Tool exposure model

Tools are configured rather than discovered dynamically. `tools/list` copies the configuration and sorts by name before rendering the MCP list. This makes the observable surface deterministic regardless of configuration order.

Each tool has a mode:

- `read` — downstream payload contains tool, arguments, actor and human only;
- `proposal` — downstream payload additionally contains `command: propose`.

No tool mode means “direct mutation”.

## 10. Attribution and actor scope

Agent and human attribution are host-owned metadata:

```text
jonno.garage/actor
jonno.garage/human
```

For `tools/call`, both must be present in `_meta`. Ordinary tool arguments cannot substitute for either value.

A tool may define `AllowedActors`. When non-empty, the actor must match before downstream I/O occurs.

This is a local exposure guard, not a replacement for downstream application authorisation.

## 11. Downstream interface

The reusable core depends on:

```go
type CommandClient interface {
    Do(context.Context, string, string, any) (json.RawMessage, error)
}
```

The production adapter is `HTTPClient`:

- sends JSON via HTTP POST;
- joins configured base endpoint and tool path;
- sets `Content-Type: application/json`;
- optionally attaches `Authorization: Bearer ...` from the CLI-owned environment setting;
- defaults to a 10-second HTTP client timeout;
- limits response reads to 1 MiB;
- treats non-2xx status as downstream failure.

The interface keeps protocol behaviour unit-testable without a live network service.

## 12. Failure semantics

| Failure | Result |
|---|---|
| bad JSON-RPC version | `-32600` |
| era not established | `-32600` |
| malformed modern metadata / call params | `-32602` |
| unsupported protocol revision / era switch | `-32022` |
| unknown method | `-32601` |
| unknown or actor-disallowed tool | `-32001` |
| missing downstream client | `-32603` |
| downstream application failure | tool result, `isError: true` |

The last case is intentionally different: once a configured tool has been validly invoked, downstream failure is a tool execution outcome rather than a JSON-RPC framing failure.

## 13. Audit behaviour

`Server.Handle` returns an `Audit` record for every handled request. The CLI writes audit JSON to stderr so stdout remains a JSON-RPC response stream.

Audit fields include time, method, tool, actor, human, era, protocol, outcome and optional detail.

V0.1 does not make stderr durable. Production applications that require immutable audit records must capture or replace this sink outside the component.

## 14. Configuration and secrets

Configuration is JSON:

```json
{
  "endpoint": "http://127.0.0.1:8080",
  "tools": [
    {"name":"incident_status","mode":"read","path":"api/status"},
    {"name":"propose_fact","mode":"proposal","path":"api/commands"}
  ]
}
```

`ValidateConfig` rejects:

- empty endpoint;
- empty tool names or paths;
- modes other than `read` or `proposal`;
- duplicate tool names.

An optional downstream bearer token is read from `GOVERNED_MCP_BEARER`. It is not part of config files, payload output or audit fields.

## 15. Testing and acceptance

The package tests cover:

- modern discovery contract;
- legacy initialise contract;
- unsupported revision rejection;
- required modern envelope on subsequent requests;
- deterministic tools list and cache hints;
- proposal translation;
- read preservation;
- unknown/disallowed refusal before downstream;
- named-human requirement;
- malformed params before downstream;
- downstream error mapping;
- no era switching;
- configuration validation.

CI additionally performs:

- `go vet ./...`;
- `go test -race ./...`;
- native build;
- version check;
- `linux/arm64` build;
- Bafflewick modern-session acceptance.

## 16. Operational and security limits

Known V0.1 limits are explicit:

- stdio only;
- one process owns one protocol connection state;
- no durable state other than the surrounding process/log capture;
- no built-in TLS or OAuth;
- no rate limiting;
- no retry or failover;
- no request signing or replay protection;
- downstream endpoint trust and business authorisation remain external;
- no claim of complete MCP feature coverage;
- Raspberry Pi/low-power runtime performance is not benchmarked in V0.1; only ARM64 buildability is proven.

## 17. Acceptance-to-evidence map

| PRD acceptance criterion | Executable evidence |
|---|---|
| modern discovery + cache hints + server identity | `TestModernDiscoveryContract` |
| legacy initialisation | `TestLegacyInitializeContract` |
| unsupported revision `-32022` | `TestUnsupportedProtocolRejected` |
| modern envelope on every request | `TestModernEnvelopeRequiredOnEveryRequest` |
| no era switch | `TestEraCannotSwitchMidConnection` |
| deterministic tool list + cache hints | `TestToolsListDeterministicAndCacheable` |
| read remains read | `TestReadToolDoesNotInventMutationCommand` |
| proposal boundary | `TestProposalBoundary` |
| unknown/disallowed before I/O | `TestUnknownAndDisallowedRefusedWithoutDownstream` |
| human attribution before I/O | `TestHumanMetadataRequired` |
| malformed params before I/O | `TestMalformedParamsRefusedWithoutDownstream` |
| downstream failure visible | `TestDownstreamFailureIsToolError` |
| invalid config rejected | `TestValidateConfig` |
| Go/race/build/ARM64/Bafflewick | `.github/workflows/test.yml` |

`generated/EDD.md` is maintained separately as recovered implementation evidence. It must not be treated as a restatement of this intended design.
