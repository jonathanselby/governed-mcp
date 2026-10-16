# Bafflewick acceptance example

Bafflewick is fictional. The example exists to demonstrate the component boundary without depending on CRAB or a real customer system.

## 1. Modern session

Build the component:

```bash
go build -o ./bin/governed-mcp ./cmd/governed-mcp
```

Run a two-request modern stdio session:

```bash
cat examples/bafflewick/modern-session.jsonl \
  | ./bin/governed-mcp -config examples/bafflewick/config.json
```

The first request is `server/discover` and carries the `2026-07-28` request envelope. The second is `tools/list` with the same modern envelope. Expected properties include:

- discovery advertises `2026-07-28` and `2025-11-25`;
- modern cache hints are conservative (`ttlMs: 0`, `cacheScope: private`);
- response `_meta` contains server identity;
- tools are listed deterministically;
- stderr contains one audit record per request.

## 2. Legacy handshake

```bash
cat examples/bafflewick/legacy-session.jsonl \
  | ./bin/governed-mcp -config examples/bafflewick/config.json
```

The initialize result selects `2025-11-25` and uses the legacy response shape.

## 3. Proposal boundary

`propose_fact` is configured as `proposal`. A valid `tools/call` therefore reaches the downstream API only after the adapter adds:

```json
{"command":"propose"}
```

The behavioural test `TestProposalBoundary` proves that translation without requiring a live downstream Bafflewick process.

## 4. Host attribution

A `tools/call` must carry these host-supplied values in `_meta`:

```text
jonno.garage/actor
jonno.garage/human
```

The component rejects calls missing either field before downstream I/O.
