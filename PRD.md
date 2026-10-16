# Governed MCP — Product Requirements Document

**Version:** 0.1

## 1. Problem

Applications increasingly expose tools to agents through MCP. If the MCP adapter also starts deciding who may act, approving mutations, storing workflow state or interpreting business policy, the adapter becomes a second application control plane.

For an application that already owns those controls, that duplication is a design defect. It creates two places to secure, test and reconcile.

## 2. Product objective

Provide a small Go MCP adapter that exposes a configured tools surface while preserving the existing application's command and approval boundary.

> **The adapter exposes commands. It does not invent a second approval system.**

The V0.1 product question is:

> Can an MCP edge remain protocol-aware and auditable while still forcing state-changing work through the application's proposal path?

## 3. Users

Primary users are engineers integrating an existing application command API with agent hosts and MCP clients.

## 4. In scope

- JSON-RPC 2.0 over stdio;
- modern MCP `2026-07-28` discovery and request envelope handling;
- legacy MCP `2025-11-25` initialisation compatibility;
- one protocol era per stdio connection;
- `tools/list` and `tools/call` only;
- deterministic tool ordering;
- configuration-defined tools;
- `read` and `proposal` tool modes;
- actor allow-lists per tool;
- host-supplied agent and named-human attribution in request `_meta`;
- local HTTP downstream command adapter;
- visible audit record for every handled request;
- modern server identity and conservative cache hints;
- MCP-shaped downstream tool failures;
- stable CLI/version contract;
- Go 1.26 and `linux/arm64` build proof.

## 5. Out of scope

- maker-checker or dual-control decisions;
- business authorisation or policy evaluation;
- identity proofing, login or token issuance;
- Streamable HTTP MCP transport;
- MCP OAuth implementation;
- prompts, resources, sampling, elicitation or subscriptions;
- server-to-client request workflows;
- tool discovery from arbitrary code;
- retry queues, durable delivery or failover;
- durable audit persistence;
- agent planning or orchestration.

These exclusions are deliberate. A CRAB requirement does not widen this component unless it changes the reusable job of a governed MCP edge.

## 6. Functional requirements

### FR1 — Modern protocol discovery

A valid `server/discover` request carrying the `2026-07-28` request envelope establishes the modern era and advertises supported protocol revisions and tool capability.

### FR2 — Legacy initialisation

A valid `initialize` request for `2025-11-25` establishes the legacy era.

### FR3 — Era integrity

After an era is established on a stdio connection, the server refuses an attempt to switch eras.

### FR4 — Modern request envelope

Every modern request after discovery must carry `io.modelcontextprotocol/protocolVersion` and an object-valued `io.modelcontextprotocol/clientCapabilities`. `clientInfo` is optional; if present, it must be structurally valid.

### FR5 — Tools list

`tools/list` returns only configured tools in deterministic name order. On the modern era it includes conservative cache hints.

### FR6 — Read boundary

A `read` tool reaches the downstream command API without an invented mutation command.

### FR7 — Proposal boundary

A `proposal` tool reaches the downstream command API with `command: propose`. The adapter never sends a direct mutation instruction for that tool.

### FR8 — Host attribution

`tools/call` requires both `jonno.garage/actor` and `jonno.garage/human` in request `_meta`. The model cannot substitute ordinary arguments for those values.

### FR9 — Scope before I/O

Unknown tools and actor-disallowed tools are refused before downstream I/O.

### FR10 — Failure visibility

A downstream application failure is returned as an MCP tool result with `isError: true`.

### FR11 — Audit visibility

Every handled request returns an audit record to the CLI, which emits it to stderr.

## 7. Acceptance criteria

V0.1 is accepted when automated evidence shows all of the following:

1. modern discovery establishes `2026-07-28`, returns conservative cache hints and stamps server identity in response `_meta`;
2. legacy `2025-11-25` initialisation remains supported;
3. unsupported protocol revisions return `-32022`;
4. a modern request without the required envelope is refused;
5. a connection cannot switch from modern to legacy after discovery;
6. `tools/list` is deterministic and modern cacheable results contain `ttlMs: 0` and `cacheScope: private`;
7. a read tool does not gain a mutation command;
8. a proposal tool gains exactly the proposal command boundary;
9. unknown and actor-disallowed tools cause no downstream call;
10. missing named-human attribution causes no downstream call;
11. malformed call parameters cause no downstream call;
12. downstream failure is visible as an MCP tool error;
13. invalid configuration is rejected;
14. `go vet ./...`, `go test -race ./...`, native build and `linux/arm64` cross-build pass in CI;
15. the Bafflewick modern-session acceptance case passes.

## 8. Definition of done

V0.1 is done when every acceptance criterion is tied to executable evidence, the protocol boundary is documented against the current MCP revisions, the hand-written EDD and recovered EDD agree on the one-job boundary, and no unsupported platform capability is implied by documentation or code.
