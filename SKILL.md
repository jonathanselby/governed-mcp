# Governed MCP — Skill

## Use this when

Use `governed-mcp` when an existing application needs a small MCP tools edge while keeping authorisation, workflow and approval state inside the application.

## Core rule

> **The adapter exposes commands. It does not invent a second approval system.**

## Read order

1. `PRD.md` — product boundary and acceptance criteria.
2. `PROTOCOL.md` — supported MCP revisions and deliberate omissions.
3. `EDD.md` — runtime, state, failure and trust boundaries.
4. `architecture.txt` — compact runtime view.
5. `examples/bafflewick/` — executable acceptance case.

## Change discipline

1. Run `go vet ./...` and `go test -race ./...` before and after change.
2. Preserve one protocol era per stdio connection.
3. Do not move business authorisation into this adapter.
4. Do not accept agent/human attribution from tool arguments.
5. Reject scope/metadata failures before downstream I/O.
6. Keep downstream application errors visible to the MCP caller.
7. Treat MCP revision changes as protocol changes, not constant updates.
8. After code changes, regenerate `generated/EDD.md` from code evidence and update `generated/COMPARISON.md`.
9. Do not tag a release until `RELEASE.md` is satisfied.

## Do not use this repo for

- identity/login;
- maker-checker or dual control;
- application workflow state;
- agent planning;
- Streamable HTTP or OAuth;
- prompts/resources/subscriptions;
- durable queues or durable audit storage.
