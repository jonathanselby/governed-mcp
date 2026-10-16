# Governed MCP — Protocol Boundary

**Component version:** 0.1.0  
**Protocol baseline reviewed:** 2026-10-17

This file records the MCP-specific boundary separately from the application design so that protocol changes can be reviewed without silently widening the component.

## Supported eras

| Era | Revision | Entry point | V0.1 behaviour |
|---|---|---|---|
| Modern | `2026-07-28` | `server/discover` | supported |
| Legacy | `2025-11-25` | `initialize` | supported for compatibility |

The stdio process establishes one era for the connection. It does not switch eras after discovery/initialisation.

## Modern request envelope

Modern requests carry protocol bookkeeping in `params._meta`. V0.1 validates:

- `io.modelcontextprotocol/protocolVersion` = `2026-07-28`;
- `io.modelcontextprotocol/clientCapabilities` is an object;
- `io.modelcontextprotocol/clientInfo`, when present, is an object with non-empty `name` and `version`.

The Garage-specific attribution keys coexist in the same `_meta` object:

- `jonno.garage/actor`;
- `jonno.garage/human`.

Reserved MCP metadata is protocol classification. Garage metadata is application-host attribution. Neither is accepted from ordinary tool arguments.

## Modern responses

For modern responses the adapter stamps:

```text
_meta.io.modelcontextprotocol/serverInfo
```

with the component name and version.

Cacheable modern results (`server/discover` and `tools/list`) emit the conservative policy:

```text
ttlMs: 0
cacheScope: private
```

The adapter does not promise cross-user cache safety.

## Unsupported revision

A request classified with an unsupported protocol revision returns JSON-RPC error code `-32022` with the requested and supported revisions in error data.

## Tool result behaviour

A downstream application error after a configured tool has been invoked is a tool failure, not a protocol-framing failure. The adapter therefore returns a `CallToolResult`-shaped result with:

```text
isError: true
```

and exposes the downstream error text in tool content.

## Deliberate omissions

V0.1 does not claim full MCP conformance. It is a tools-only stdio adapter and intentionally does not implement:

- Streamable HTTP;
- SEP-2243 HTTP header mirroring;
- OAuth;
- prompts or resources;
- subscriptions;
- sampling or elicitation;
- multi-round-trip `input_required` flows;
- server-to-client requests;
- task extensions.

Those are not placeholders. They remain absent unless the reusable job of this repository changes.

## Upgrade rule

A future MCP revision is not adopted by changing version constants alone. An upgrade requires:

1. protocol-delta review;
2. explicit PRD/EDD change if the component boundary changes;
3. executable tests for new wire behaviour;
4. Bafflewick acceptance update;
5. regenerated code-derived EDD and comparison;
6. a new semantic version where behaviour changes.
