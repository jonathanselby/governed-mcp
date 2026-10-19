# Governed MCP

[![test](https://github.com/jonathanselby/governed-mcp/actions/workflows/test.yml/badge.svg)](https://github.com/jonathanselby/governed-mcp/actions/workflows/test.yml)

**The adapter exposes commands. It does not invent a second approval system.**

`governed-mcp` is a deliberately small Go adapter for exposing an existing application command API as a governed MCP tool surface. It is designed for applications that already own authorisation, workflow and approval state and do not want an MCP edge to become a second control plane.

## What V0.1 proves

V0.1 demonstrates that a tools-only MCP edge can remain thin while preserving four controls:

- the MCP protocol era is explicit;
- the host, not the model, supplies agent and named-human attribution;
- read calls remain reads;
- state-changing calls are translated into **proposals**, not direct mutations.

The implementation is executable, race-tested, cross-built for `linux/arm64`, and compared against its hand-written design through a code-derived EDD.

## Supported protocol surface

V0.1 deliberately supports a narrow subset:

| Area | V0.1 |
|---|---|
| Transport | JSON-RPC 2.0 over stdio |
| Modern MCP | `2026-07-28` via `server/discover` and per-request `_meta` envelope |
| Legacy MCP | `2025-11-25` via `initialize` |
| Tools | `tools/list`, `tools/call` |
| Modern response identity | `io.modelcontextprotocol/serverInfo` in response `_meta` |
| Modern cache hints | `ttlMs: 0`, `cacheScope: private` on cacheable results |
| Unsupported revision | JSON-RPC `-32022` |

See [`PROTOCOL.md`](PROTOCOL.md) for the exact standards boundary and deliberate omissions.

## Governance boundary

A configured tool has one of two modes:

- `read` — forwards the application command without adding a mutation command;
- `proposal` — adds `command: propose` before calling the application command API.

The component does **not** implement maker-checker, approval queues, policy evaluation or business authorisation. Those stay with the application that owns the state.

The client host must attach:

- `jonno.garage/actor` — the invoking agent/runtime identity;
- `jonno.garage/human` — the named human on whose behalf the host is operating.

Those values are taken from request `_meta`, not model arguments.

## Install

Requires Go 1.26+.

```bash
go install github.com/jonathanselby/governed-mcp/cmd/governed-mcp@main
```

Check the executable:

```bash
governed-mcp -version
```

Expected output:

```text
governed-mcp 0.1.0
```

## Run the Bafflewick example

```bash
go build -o ./bin/governed-mcp ./cmd/governed-mcp
cat examples/bafflewick/modern-session.jsonl \
  | ./bin/governed-mcp -config examples/bafflewick/config.json
```

That input establishes the modern era and then lists the configured tools in deterministic order. Audit records are emitted to stderr.

See [`examples/bafflewick/README.md`](examples/bafflewick/README.md) for the legacy handshake and proposal boundary examples.

## Exit codes

The process is a long-running stdio adapter. Request errors remain JSON-RPC responses and do not normally terminate the process.

| Code | Meaning |
|---:|---|
| `0` | clean process completion |
| `2` | invalid CLI usage |
| `3` | configuration, input stream or execution failure |

Governed refusals are protocol-level results/errors, not process exit status `1`.

## Failure behaviour

- invalid JSON-RPC framing is rejected before dispatch;
- a modern request without the required protocol envelope is rejected;
- an unsupported protocol revision is rejected with `-32022`;
- a protocol era cannot switch mid stdio connection;
- unknown or actor-disallowed tools are refused before downstream I/O;
- missing host attribution is refused before downstream I/O;
- downstream application failures become visible MCP tool errors (`isError: true`), rather than being hidden or converted into false success;
- there is no retry loop, queue or failover mechanism in this component.

## Security and operational boundary

This repository is a protocol adapter, not an identity or authorisation system. The configured downstream endpoint is trusted to enforce its own application controls. The optional bearer token is read from `GOVERNED_MCP_BEARER`; it is never written to the audit stream.

V0.1 does not provide Streamable HTTP, OAuth, TLS termination, secret storage, rate limiting, durable audit storage, replay protection, request signing, user sessions, prompts, resources, elicitation, sampling or subscriptions.

## Repository map

```text
governed-mcp/
├── .github/workflows/test.yml
├── PRD.md
├── EDD.md
├── PROTOCOL.md
├── README.md
├── SKILL.md
├── RELEASE.md
├── VERSION
├── CHANGELOG.md
├── LICENSE
├── architecture.txt
├── go.mod
├── cmd/governed-mcp/main.go
├── internal/mcp/
├── examples/bafflewick/
└── generated/
    ├── EDD.md
    └── COMPARISON.md
```

## Engineering evidence

- [`PRD.md`](PRD.md) — problem, product boundary and acceptance criteria;
- [`EDD.md`](EDD.md) — intended engineering design;
- [`PROTOCOL.md`](PROTOCOL.md) — MCP revision boundary and deliberate deviations;
- [`architecture.txt`](architecture.txt) — runtime and trust-boundary view;
- [`internal/mcp`](internal/mcp) — reusable implementation and behavioural tests;
- [`examples/bafflewick`](examples/bafflewick) — worked fictional acceptance case;
- [`generated/EDD.md`](generated/EDD.md) — implementation-derived evidence pinned to a source commit;
- [`generated/COMPARISON.md`](generated/COMPARISON.md) — intended design versus recovered implementation;
- [`RELEASE.md`](RELEASE.md) — release gate.

## Design rule

> **The adapter exposes commands. It does not invent a second approval system.**

## Licence

MIT. See [`LICENSE`](LICENSE).
