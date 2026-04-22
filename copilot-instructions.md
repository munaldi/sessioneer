# Sessioneer — Copilot Instructions

This is a Go CLI project that manages AI coding session files for Claude Code (`.jsonl`) and GitHub Copilot (`.json`). It uses Bubble Tea for the TUI.

## Code style

- Go 1.22+; modules at `github.com/munaldi/sessioneer`
- No classes — plain structs + functions only
- No `any` type; no unchecked type assertions
- Early return preferred over nested conditionals
- All exported types live in `pkg/types/types.go`
- All file I/O lives in `internal/session/`
- All business logic (fork, merge, prune, trim…) lives in `internal/actions/`
- TUI (Bubble Tea) lives in `internal/tui/`

## Import direction (strict — never reverse)

```
cmd/ → internal/tui/ → internal/actions/ → internal/session/ → internal/provider/ → pkg/types/
```

## Naming conventions

- Functions: `VerbNoun` (e.g., `LoadSession`, `ForkSession`, `PruneMessages`)
- Types: `PascalCase` noun (e.g., `Session`, `TokenUsage`, `PruneOptions`)
- Files: `snake_case.go`
- Test files: `snake_case_test.go`, package `foo_test` (external)

## Adding features

- New provider → `internal/provider/` + `internal/session/<provider>.go`
- New action → `internal/actions/<action>.go` + entry in `internal/tui/model.go`
- New type → `pkg/types/types.go`

## Testing

- Run: `go test ./...`
- Tests are table-driven where multiple cases exist
- Each action package must have a `*_test.go`

## Key dependencies

| Package | Purpose |
|---|---|
| `github.com/charmbracelet/bubbletea` | TUI event loop |
| `github.com/charmbracelet/bubbles` | Text input, list components |
| `github.com/charmbracelet/lipgloss` | Terminal styling |
| `github.com/spf13/cobra` | CLI argument parsing |
| `github.com/google/uuid` | UUID generation for fork/merge |
