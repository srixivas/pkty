## 🦉 What does this PR do?

<!-- One-line summary -->

## 🔍 Details

<!-- What changed and why. Link any related issues with "Fixes #N" -->

## ✅ Checklist

- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] Tests pass: `CGO_ENABLED=0 go test ./internal/widgets/... ./internal/events/... ./internal/resolve/...`
- [ ] No new global mutable state
- [ ] New widgets implement the `Widget` interface and gate on `!focused` at the top of `Update`
- [ ] Display filter wiring added if new widget (see `CLAUDE.md` → Display filter wiring)
