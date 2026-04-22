# Sessioneer

✨ An interactive **CLI** to browse and manage **Claude Code** and **GitHub Copilot** sessions from the terminal.

---

## 💡 Why

Both Claude Code and GitHub Copilot store local session/conversation files, but neither provides a built-in way to search, browse, fork, merge, prune, or trim them.

**Sessioneer** gives you a single keyboard-driven interface for both tools:

- Browse all sessions sorted by date, across both providers
- Search text across all sessions with highlighted results
- Preview conversations before acting
- Fork, merge, prune, trim, rename, or delete sessions
- View session and project stats (duration, tokens, costs, tool usage)
- Batch-clean empty session files
- Run in either terminal TUI mode or browser Web UI mode

---

## 📦 Install

```bash
go install github.com/munaldi/sessioneer@latest
```

Or build from source:

```bash
git clone https://github.com/munaldi/sessioneer
cd sessioneer
go mod tidy
go build -o bin/sessioneer .
```

**Requirements:** Go 1.22+

---

## 🚀 Quick Start

```bash
go run .
```

The CLI auto-detects available providers and opens a provider picker if more than one is found.

If you built a binary:

```bash
./bin/sessioneer
```

### Options

| Flag                   | Short | Description                              | Default                     |
|------------------------|-------|------------------------------------------|-----------------------------|
| `--provider <name>`    | `-P`  | `claude` or `copilot`                    | auto-detect                 |
| `--project <path>`     | `-p`  | Project path                             | Current directory           |
| `--base <path>`        | `-b`  | Session base directory                   | Provider default (see below)|
| `--web`                | `-w`  | Launch the web UI instead of terminal UI | false                       |
| `--port <number>`      |       | Web UI/API port (used with `--web`)      | `8080`                      |

### Provider defaults

| Provider       | Default session directory                                                       |
|----------------|---------------------------------------------------------------------------------|
| Claude Code    | `~/.claude/projects`                                                            |
| GitHub Copilot | `~/.config/github-copilot/conversations` (Linux) · `~/Library/…` (macOS) · `%APPDATA%\…` (Windows) |

---

## How It Works

1. Detects provider session directories (or prompts you to pick one)
2. Scans for session files (`.jsonl` for Claude, `.json` for Copilot)
3. In TUI mode: shows a **main menu** (Sessions, Search, Project Stats, Clean empty sessions, Switch provider, Quit)
4. Shows a **paginated session list** sorted by date (newest first) with metadata:
  - Provider
  - Message count
  - Last updated time
  - Session file path
5. Shows **search results** with the same metadata plus a matched snippet
6. Opens an **action menu** with one-line descriptions for each action

### Web UI mode

Run with `--web` to start the browser app and REST API:

```bash
go run . --web
go run . --web --port 9090
```

When started, Sessioneer prints a localhost URL and opens your default browser automatically.

---

## 🛠️ Actions

> **Note:** Actions that modify session files require restarting the extension to take effect.
> In VS Code: `Ctrl+Shift+P` / `Cmd+Shift+P` → `Developer: Reload Window`

### ◌ Open
Reveals the session file directory in your native file manager (Finder, Explorer, xdg-open).

### ◌ Fork
Creates an independent copy of the session with all UUIDs remapped. Prompts for a new title.

### ◌ Merge
Combines two or more sessions into a single new session, concatenated in chronological order. All UUIDs are remapped to avoid conflicts. Prompts for a title.

### ◌ Prune
Removes noise from a session:
- **Tool blocks** — `tool_use` and `tool_result` entries
- **Empty messages** — no text content after stripping tools
- **System/IDE tags** — `<system-reminder>`, `<ide_selection>`, `<ide_opened_file>`

Repairs the parent-child UUID chain after removal.

### ◌ Trim
Removes newer messages from the end of a session (safe default cutoff).

### ◌ Stats
Shows per-session or project-wide statistics:
- Duration (first → last message)
- Message counts (user vs assistant)
- Token usage (input, output, cache creation, cache read)
- Estimated cost (USD, Claude sessions only)
- Tool usage frequency

### ◌ Rename
Writes a new title to the session file. For Claude sessions this appends a `custom-title` entry (same format as the Claude Code extension). For Copilot sessions it updates the `title` field.

### ◌ Delete
Removes the session file from disk.

### ◌ Clean _(main menu)_
Batch-deletes all empty session files. Only shown when empty files exist.

---

## ⌨️ Navigation

| Key         | Action |
|-------------|--------|
| `↑` / `k`   | Move cursor up |
| `↓` / `j`   | Move cursor down |
| `Enter`     | Confirm / open |
| `Esc` / `q` | Go back one level |

Quit is available from the **Quit** option in the main menu.

Note: this section applies to **TUI mode**. In Web mode, actions are available through buttons/forms in the browser UI.

---

## 🧱 Architecture

```
main.go               CLI entry point (cobra)
internal/
  actions/            Business logic: fork, merge, prune, trim, rename, delete, search, open
  config/             CLI flag resolution
  provider/           Provider detection + platform default directories
  session/            File I/O: Claude (.jsonl) and Copilot (.json) parsers + stats
  tui/                Bubble Tea model, views, styles, key bindings
  web/                Embedded static site + HTTP API server
pkg/types/            Shared types — single source of truth
```

Import direction (never reversed):
```
main.go → internal/tui/ or internal/web/ → internal/actions/ → internal/session/ → internal/provider/ → pkg/types/
```

---

## 🧪 Development

```bash
go mod tidy
go test ./...
go run .
go run . --provider claude
go run . --provider copilot
go run . --web
go run . --web --port 9090
```

---

## Contributing

Please open an Issue before submitting a PR for new features, so the API can be discussed first.

---

## License

MIT
