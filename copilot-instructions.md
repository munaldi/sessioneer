# Sessioneer — Copilot Instructions

This is a Go project that manages AI coding session files for Claude Code (`.jsonl`) and GitHub Copilot (`.json`). It supports both Bubble Tea TUI mode and Web UI mode.

## Code style

- Go 1.22+; modules at `github.com/munaldi/sessioneer`
- No classes — plain structs + functions only
- No `any` type; no unchecked type assertions
- Early return preferred over nested conditionals
- All exported types live in `pkg/types/types.go`
- All file I/O lives in `internal/session/`
- All business logic (fork, merge, prune, trim…) lives in `internal/actions/`
- TUI (Bubble Tea) lives in `internal/tui/`
- Web server/UI lives in `internal/web/`

## Import direction (strict — never reverse)

```
main.go → internal/tui/ or internal/web/ → internal/actions/ → internal/session/ → internal/provider/ → pkg/types/
```

## Naming conventions

- Functions: `VerbNoun` (e.g., `LoadSession`, `ForkSession`, `PruneMessages`)
- Types: `PascalCase` noun (e.g., `Session`, `TokenUsage`, `PruneOptions`)
- Files: `snake_case.go`
- Test files: `snake_case_test.go`, package `foo_test` (external)

## Adding features

- New provider → `internal/provider/` + `internal/session/<provider>.go`
- New action → `internal/actions/<action>.go` + entry in `internal/tui/model.go`
- Web/API change → `internal/web/server.go` (+ `internal/web/static/` when UI changes)
- New type → `pkg/types/types.go`

## Testing

- Run: `go test ./...`
- Manual run (TUI): `go run .`
- Manual run (Web): `go run . --web --port 8080`
- Tests are table-driven where multiple cases exist
- Each action package must have a `*_test.go`

## Key dependencies

| Package | Purpose |
|---|---|
| `github.com/charmbracelet/bubbletea` | TUI event loop |
| `github.com/charmbracelet/bubbles` | Text input, list components |
| `github.com/spf13/cobra` | CLI argument parsing |
