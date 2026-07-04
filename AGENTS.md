# herdr-whistle

Herdr plugin for remote agent management. List agents, read their output, send text to unblock them, close panes, and start new agents -- all from Telegram.

## Build & Run

```sh
go build -o herdr-whistle .
herdr plugin link /path/to/herdr-whistle
```

Config reads from `$HERDR_PLUGIN_CONFIG_DIR/config.toml`, falls back to `./config.toml`.

## Test

```sh
go test ./...
```

Test quirks:
- Tests swap package-level vars (`herdrAgentList`, `notifyBlocked`) to mock herdr CLI -- always restore with `defer`.
- Timing-sensitive: watcher tests use context timeouts of 8-13s (watcher polls every 5s). No external fixtures.

## Single package -- no internal directory

All `.go` files are `package main`. No sub-packages.

| File | Role |
|---|---|
| `main.go` | Entry, config loading, SIGINT/SIGTERM shutdown |
| `bot.go` | Telegram bot init (retry with backoff), handler registration, message senders |
| `handlers.go` | Command handlers (`/agents`, `/status`, `/read`, `/send`, `/close`, `/startagent`), inline keyboard callbacks, HTML escaping, owner auth |
| `herdrcli.go` | herdr CLI wrappers via `os/exec` (30s timeout per call) |
| `watcher.go` | Background goroutine polls `herdr agent list` every 5s, notifies on `blocked` transitions |
| `choices.go` | Parses @clack/prompts and box-drawing style choice menus from agent output |
| `config.go` | TOML config loading |

## herdr CLI integration

- Binary resolved from `HERDR_BIN_PATH` env var (falls back to `herdr` in PATH).
- All commands use `exec.CommandContext` with 30-second timeout.
- Key subcommands: `agent list`, `agent get`, `agent explain --json`, `agent read --source recent-unwrapped --lines N`, `agent start`, `pane run`, `pane close`.
- `/read` reads via `--source recent-unwrapped` (not the default source).

## TOML config

```toml
token = "bot:token"
owner_id = 12345
```

Only `token` (string) and `owner_id` (int64). Hard-fails if either is empty/zero.

## Telegram handler registration

Command patterns omit the leading `/`:
- `bot.RegisterHandler(bot.HandlerTypeMessageText, "start", bot.MatchTypeCommand, handler)`
- Inline keyboard callbacks use `bot.HandlerTypeCallbackQueryData` with `bot.MatchTypePrefix`

## Inline keyboard callbacks

- Agent list buttons: `al|{action}|{paneID}` (actions: `status`, `read`, `close`, `close_confirm`, `close_cancel`, `refresh`)
- Choice buttons: `ch|{paneID}|{index}` (1-based)

## Blocked agent detection

`watcher.go` polls every 5s, tracks status per `(paneID, sessionID)`. Notifies only on transition from any other status to `"blocked"`. Notifications include choice menu parsing (two parsers: @clack/prompts first, box-drawing fallback).

## Message safety

- `sanitizeTTY()` strips control characters below 0x20 except `\n`, `\r`, `\t`
- `escapeHTML()` escapes `& < >` for Telegram HTML mode
- All formatted messages use `ParseModeHTML` and pre-escaped text
