# Release checklist

A release is ready only when all of the following are true.

## Source and version

- [ ] `VERSION`, CLI `-version` output and tag agree.
- [ ] `go.mod` uses the Garage Go baseline.
- [ ] `CHANGELOG.md` records the release.

## Engineering gate

- [ ] `gofmt` produces no changes.
- [ ] `go vet ./...` passes.
- [ ] `go test -race ./...` passes.
- [ ] native build passes.
- [ ] `GOOS=linux GOARCH=arm64 go build ./cmd/...` passes.
- [ ] Bafflewick modern-session acceptance passes.
- [ ] Bafflewick legacy handshake is covered by test or executable check.

## Requirements traceability

- [ ] every PRD acceptance criterion maps to a named test or explicit acceptance command.
- [ ] protocol-specific behaviour is still consistent with `PROTOCOL.md`.
- [ ] known limitations remain explicit.

## Design evidence

- [ ] `EDD.md` describes the intended release candidate.
- [ ] `generated/EDD.md` is recovered from the source commit, not copied from `EDD.md`.
- [ ] `generated/COMPARISON.md` explains matches and any mismatch.
- [ ] generated evidence names the exact source commit it analysed.

Tag format: `v0.1.0`, `v0.2.0`, and so on.
