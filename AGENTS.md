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
| `main.go` | Entry, subcommand dispatch (`start`/`stop`/`run`), config loading, SIGINT/SIGTERM shutdown |
| `lifecycle.go` | Autostart + single-instance: canonical state dir, `flock`, detached worker spawn, pidfile, `start`/`stop` subcommands |
| `bot.go` | Telegram bot init (retry with backoff), handler registration, message senders |
| `handlers.go` | Command handlers (`/agents`, `/status`, `/read`, `/send`, `/close`, `/startagent`), inline keyboard callbacks, HTML escaping, owner auth |
| `herdrcli.go` | herdr CLI wrappers via `os/exec` (30s timeout per call) |
| `watcher.go` | Background goroutine polls `herdr agent list` every 5s, notifies on `blocked` transitions |
| `choices.go` | Parses @clack/prompts and box-drawing style choice menus from agent output |
| `config.go` | TOML config loading |

## Autostart & single-instance

The bot starts itself when herdr starts, fully contained in the plugin (no
systemd/cron). The manifest hooks `workspace.created` and `workspace.focused`
to the `start` subcommand; the first workspace use after server boot launches
the bot.

Subcommands (`./herdr-whistle <cmd>`):

- `start` - probe the single-instance lock; if free, spawn a detached `run`
  worker (new session via `setsid`, stdio to the state dir's `bot.log`) and
  return immediately so herdr is not blocked. No-op if an instance is running.
- `run` - the foreground bot (also the default). Acquires the single-instance
  lock, writes the pidfile, then blocks on Telegram polling until SIGINT/SIGTERM.
- `stop` - SIGTERM the running instance via its pidfile. Idempotent.

Single-instance is enforced in the binary, not the launcher: `run` takes a
non-blocking `flock` on a canonical, machine-global path before polling, because
Telegram allows only one `getUpdates` poller per token. The path is
`$XDG_CONFIG_HOME/herdr-whistle/instance.lock` (i.e. `~/.config/herdr-whistle/`),
deliberately independent of herdr's per-invocation `HERDR_PLUGIN_STATE_DIR` so
every launch path (event, action, manual) converges on one lock. `bot.pid` and
`bot.log` live in the same dir.

Crash recovery is lazy: if the worker exits unexpectedly it stays down until the
next `workspace.*` event re-runs `start`. The bot already survives herdr itself
being temporarily down (the watcher logs and retries).

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
