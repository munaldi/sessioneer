# Sessioneer

Interactive CLI tool to browse, search, fork, merge, prune, trim, rename, and delete AI coding sessions for **Claude Code** and **GitHub Copilot**. Written in Go with Bubble Tea for the TUI.

## Priorities (in order)

1. **Security** — never compromise; no arbitrary file writes outside session dirs
2. **Maintainability** — clean names, decoupling, single-responsibility functions
3. **Performance** — never compromising long-term maintenance

## Quick reference

- No classes; use plain structs and functions
- `types` only in `pkg/types/` — no `any`, no type assertions without guards
- Early return always
- Import direction: `main.go → internal/tui/ or internal/web/ → internal/actions/ → internal/session/ → internal/provider/ → pkg/types/` (never reverse)
- All mutation goes through `internal/actions/`; `internal/session/` handles I/O only
- Tests live next to the package they test (`*_test.go`, external `package foo_test`)

## Project layout

```
main.go               # CLI entry point (cobra)
internal/
  actions/            # Fork, merge, prune, trim, rename, delete, search, open
  config/             # CLI flag resolution → AppConfig
  provider/           # Provider detection + default dirs
  session/            # JSONL (Claude) and JSON (Copilot) I/O, stats
  tui/                # Bubble Tea model, view, styles, keys
  web/                # Embedded static UI + REST API server
pkg/types/            # Shared types — only place for type definitions
```

## Adding a new provider

1. Add a `Provider` constant in `pkg/types/types.go`
2. Add `DefaultBaseDir` case in `internal/provider/provider.go`
3. Add `loadXxx` / `saveXxx` in `internal/session/` (new file `xxx.go`)
4. Add dispatch cases in `internal/session/session.go`
5. Update `internal/provider/provider.go` `Detect()` list
6. Update README provider table

## Adding a new action

1. Add a function in `internal/actions/` (new file if substantial)
2. Add a constant and label in `internal/tui/model.go` (`actionMenuItems`)
3. Add a `case` in `dispatchAction()` in `internal/tui/model.go`
4. Update README Actions section

## Verification

```sh
go run .                          # auto-detect providers
go run . --provider claude        # force Claude Code
go run . --provider copilot       # force GitHub Copilot
go run . --provider claude --base /path/to/.claude/projects
go run . --web                    # web UI + API on :8080
go run . --web --port 9090        # web UI + API on :9090
go test ./...
```

## Auto update

When modifying code that impacts documentation, update in the same change:

1. **CLAUDE.md** — if layout, priorities, import rules, or quick reference change
2. **README.md** — if features, actions, flags, or navigation change
3. **Tests** — every new action or parser must have a corresponding `_test.go`
