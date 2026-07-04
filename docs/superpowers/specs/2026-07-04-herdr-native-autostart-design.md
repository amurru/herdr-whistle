# herdr-whistle autostart, fully contained in herdr

- Date: 2026-07-04
- Status: Design (awaiting user review)
- Owner: Amurru Zerouk

## Goal

Start the herdr-whistle Telegram bot automatically when herdr starts, with the
entire solution living inside the herdr plugin system and inside the single Go
binary. No shell scripts, no systemd, no `loginctl enable-linger`, no cron, no
external supervisor. Everything ships in the plugin directory, the manifest, and
the binary itself.

## Context

herdr-whistle is a herdr plugin (`herdr-plugin.toml`) that is already linked and
enabled. Today it exposes a single manual `[[actions]]` entry (`start`) that runs
`./herdr-whistle`. The bot is a long-running daemon: `main.go` blocks on the
Telegram long-poll loop until SIGINT/SIGTERM.

herdr's plugin runtime (v1) offers four manifest entrypoint types: `[[actions]]`,
`[[events]]`, `[[panes]]`, `[[link_handlers]]`. There is **no** "start this
plugin when the server boots" flag. `[[events]]` hooks fire on workspace / pane /
tab / worktree lifecycle events; there is no `server.started` event. herdr
event/action commands are designed for commands that do work and exit, not for
supervised daemons. (Source: https://herdr.dev/docs/plugins/ and
https://herdr.dev/docs/socket-api/)

The herdr server is currently started interactively: a `herdr` launcher process
spawns `/usr/bin/herdr server` as a child. It is not under systemd, and
`loginctl` linger is off.

Constraints the user set:

- The solution must be completely contained in the herdr system.
- The autostart logic must live in the Go binary itself - no side-car shell
  scripts.
- Crash recovery may be lazy (the user accepted the bot staying down until the
  next workspace event relaunches it).

Verified facts that shape the design:

- The bot already survives the herdr server being temporarily down. In
  `watcher.go`, a failed `herdrAgentList()` logs a WARN and returns; the poll
  loop continues. The bot does not crash.
- herdr injects `HERDR_PLUGIN_ROOT`, `HERDR_PLUGIN_CONFIG_DIR`,
  `HERDR_PLUGIN_STATE_DIR`, `HERDR_BIN_PATH`, and `HERDR_ENV=1` into runtime
  commands. The bot already reads config from `HERDR_PLUGIN_CONFIG_DIR` and
  resolves the herdr binary via `HERDR_BIN_PATH`.
- Plugin config (token + owner_id) lives at
  `/home/amurru/.config/herdr/plugins/config/herdr-whistle/config.toml`.
- The bot has no single-instance guard today. `main.go` goes straight into
  `startBot` -> `b.Start` (the Telegram long-poll loop). Telegram allows only one
  `getUpdates` poller per token, so a second instance causes 409 conflicts and
  splits/drops messages.

## Design overview

herdr's `command` is an argv array. The manifest points the `start` action and
the workspace event hooks at `./herdr-whistle start`. The `start` subcommand
probes the single-instance lock and, if free, spawns a detached copy of itself
in `run` mode, then exits 0 immediately so herdr is not blocked. The detached
`run` process is the actual bot.

```
workspace.created / workspace.focused   (or the `start` action)
        |
        v
herdr runs:  ./herdr-whistle start
        |
        v
start subcommand (short-lived, returns to herdr at once)
   probe canonical flock (~/.config/herdr-whistle/instance.lock)
      already held?  -> exit 0   (another instance is live)
   else spawn detached worker:
       exec os.Executable() "run"
       SysProcAttr{Setsid: true}        # new session -> survives herdr
       stdio -> ~/.config/herdr-whistle/bot.log
       env inherited (HERDR_PLUGIN_CONFIG_DIR, HERDR_BIN_PATH, ...)
   exit 0
        |
        v (detached, new session)
run subcommand (the long-running bot)
   acquire canonical flock (LOCK_EX|LOCK_NB)
      fails?  -> exit 0   (race loser; exits before opening Telegram)
   write ~/.config/herdr-whistle/bot.pid
   load config; start bot + watcher; block on b.Start(ctx)
   SIGINT/SIGTERM -> ctx cancel -> clean shutdown
```

Because `start` always returns immediately, frequent `workspace.focused` events
are cheap (a probe + maybe one short-lived spawn that exits if it loses the
race).

## Components

### 1. Manifest: `herdr-plugin.toml`

Keep the existing `start` action and add a `stop` action and the event hooks.
All entrypoints invoke the binary with a subcommand.

```toml
[[actions]]
id = "start"
title = "Start Telegram bot"
contexts = ["workspace"]
command = ["./herdr-whistle", "start"]

[[actions]]
id = "stop"
title = "Stop Telegram bot"
contexts = ["workspace"]
command = ["./herdr-whistle", "stop"]

[[events]]
on = "workspace.created"
command = ["./herdr-whistle", "start"]

[[events]]
on = "workspace.focused"
command = ["./herdr-whistle", "start"]
```

Notes (confirmed against https://herdr.dev/docs/plugins/ and .../socket-api/):

- `[[events]]` takes no `id` field. Only actions, panes, and link handlers have
  local ids; the docs' events example uses just `on` + `command`.
- `workspace.created` and `workspace.focused` are real event names - the
  socket-api "Event subscriptions" section lists the workspace event set as
  `workspace.created`, `.updated`, `.renamed`, `.closed`, `.focused`.
- There is no `server.started` event; the taxonomy is workspace / pane / worktree
  lifecycle only. "Start on first workspace event" is therefore the closest
  herdr-native trigger. This is an inherent herdr limitation, not a design gap.
- Event-name validation at link time is non-fatal: an unknown `on` value is not
  an error, it surfaces in the `warnings` field of `plugin.link` / `plugin.list`.
- herdr does not run commands through a shell, so `["./herdr-whistle", "start"]`
  execs the binary directly with one argv. No shell, no quoting risk.

Why both `workspace.created` and `workspace.focused`:

- `workspace.created` covers the fresh-start case (first workspace after server
  boot).
- `workspace.focused` covers server restart with restored/persistent workspaces
  (where the workspace already exists, so `created` may not re-fire) and acts as
  a cheap respawn trigger if the bot has died.

### 2. Subcommand dispatch and lifecycle (`lifecycle.go`, new file)

The project is a single `package main` with one file per concern. Add
`lifecycle.go` for the start/run/stop plumbing; keep `main.go` for dispatch and
the existing run path.

- `main.go` inspects `os.Args[1]`:
  - `"start"` -> call `runStart()`, exit.
  - `"stop"` -> call `runStop()`, exit.
  - absent or `"run"` -> the existing flow: load config, acquire the instance
    lock, write the pidfile, `startBot(ctx, cfg)`. (Default is `run` so a bare
    `./herdr-whistle` or `go run .` for manual debugging starts the bot in the
    foreground, not a detach.)
- `runStart()`:
  - Resolve the canonical state dir and lock path (see component 3).
  - Probe the lock non-blocking; if held, log and `return` (exit 0).
  - Else spawn a detached worker: `exec.Command(os.Executable(), "run")` with
    `SysProcAttr{Setsid: true}` (new session), stdin `/dev/null`, stdout+stderr
    appended to `bot.log`, env inherited. `cmd.Start()` then return (exit 0).
- `runStop()`:
  - Read `bot.pid`; if the pid is alive, `SIGTERM` it. The bot's existing
    `signal.NotifyContext` handler cancels ctx and shuts down cleanly.
  - If no pidfile or the pid is dead, remove any stale pidfile, print "not
    running", exit 0. Idempotent.

### 3. Single-instance lock (the hard guarantee) - in the binary

This is the guarantee that exactly one `herdr-whistle` process polls Telegram,
regardless of launch path. It must be a property of the bot itself.

- Canonical, machine-global path, independent of launch source:
  `os.UserConfigDir()/herdr-whistle/instance.lock` (Linux:
  `~/.config/herdr-whistle/instance.lock`). Do NOT derive it from
  `HERDR_PLUGIN_STATE_DIR`, which differs between herdr-injected and manual
  launches - that divergence is itself a double-instance bug. All state files
  (`instance.lock`, `bot.pid`, `bot.log`) live under this one canonical dir.
- `run` acquires the lock before `startBot`: open `O_CREATE|O_RDWR`,
  `flock(LOCK_EX|LOCK_NB)` (via `golang.org/x/sys/unix` or `syscall.Flock`). On
  failure, log `another herdr-whistle instance is running (lock %s), exiting` and
  exit 0. Hold the fd for the whole process lifetime (do not close it early); the
  lock auto-releases only when the bot truly dies.
- `start` probes the same lock before spawning, so it can bow out without even
  exec'ing a worker. The race window (two `start`s probe a free lock, both spawn)
  is harmless: both workers try to acquire, one wins, the loser exits 0 before
  opening the Telegram connection - no 409 ever reaches the API.

This closes all four double-instance gaps:

- (A) direct/manual `./herdr-whistle` launches hit the same canonical lock;
- (B) the path is fixed, not env-derived, so every launch converges on it;
- (C) an orphaned bot (e.g. its parent died) keeps holding its own lock until it
  actually dies, so a new launch cannot acquire it;
- (D) multiple herdr sessions share one machine-global path.

### 4. Self-detach without scripts

`runStart` uses Go's `syscall.SysProcAttr{Setsid: true}` to put the worker in a
new session and `os/exec` to redirect stdio to `bot.log`. This replaces the
shell `setsid` + redirect and is portable to Linux and macOS (Go supports
`Setsid` on both), removing the earlier shell-`flock`/`setsid` portability
caveat. The worker inherits the parent env, so `HERDR_PLUGIN_CONFIG_DIR` and
`HERDR_BIN_PATH` propagate to the detached bot.

### 5. Optional: throttle herdr-down WARN spam

While herdr is down, `watcher.go` logs a WARN every 5s. Optional refinement: log
the first failure, suppress repeats, then log recovery once. Small, isolated
change in `refresh()`. No behavior change; the poll loop already continues on
error.

### 6. Optional: in-process crash recovery (pure binary, cheap)

Because there is no supervisor, a Go panic in a handler would take the bot down
until the next workspace event. Optional defensive wrap: run the bot's main loop
inside a `recover()` that logs and restarts the loop after a short backoff,
within the same process (so the lock and pidfile stay valid). This covers the
common crash class (panics) immediately while staying single-binary. OS-level
kills (SIGKILL, OOM, segfault) still fall back to lazy event-based respawn. Off
by default unless the user opts in.

## Lifecycle semantics

- **Start:** first `workspace.created` or `workspace.focused` after herdr server
  boot runs `./herdr-whistle start`, which spawns the detached bot. ~equivalent
  to "when herdr starts" because herdr opens a workspace whenever it is used.
- **Single-instance:** exactly one bot polls Telegram, across all launch paths
  and all herdr sessions, enforced by the canonical flock in `run`.
- **Crash recovery (lazy):** if the bot process exits unexpectedly, it stays down
  until the next `workspace.created`/`workspace.focused` event re-runs `start`,
  which probes the now-free lock and spawns a fresh worker. (Optional in-process
  `recover`, component 6, narrows this to OS-only kills.)
- **herdr server down:** the bot stays alive and idle. The watcher logs WARNs
  (throttled, if refinement #5 is applied) and resumes as soon as herdr is back.
  The bot does not exit. This is the user-chosen behavior.
- **Manual stop:** `herdr plugin action invoke herdr-whistle stop` (or a
  keybinding) reads the pidfile and SIGTERMs the bot. The next workspace event
  relaunches it.
- **herdr server stop / reboot:** the detached bot is NOT auto-stopped. It
  survives until the machine reboots or the user runs `stop`. See Honest gaps.

## Honest gaps (inherent to staying fully inside herdr)

1. **Start is "first workspace event after boot", not the literal server-process
   start moment.** For an interactive herdr session this is effectively the same.
2. **No auto-stop when herdr stops.** The bot stays alive (idle) when the server
   goes away. It is harmless (resilient to herdr being down) but does not exit on
   its own. This is the deliberate trade-off for a herdr-contained solution.
3. **Relies on herdr leaving a `setsid`-detached child running.** Standard
   daemonization says a new session + redirected stdio escapes the parent's
   process-group kill, but herdr's behavior with long-running event commands is
   not explicitly documented. Smoke-test step 1 proves this before anything else
   is built.
4. **Lazy crash recovery** means the bot can be down until the next local
   workspace interaction - which is weakest exactly when the user is remote (the
   Telegram use case). Mitigated by the bot's stability and the optional
   in-process `recover` (component 6).

## Verification plan

1. **Detach smoke test (do first).** Event names are already confirmed by the
   socket-api event list and the real-world `third774/herdr-last-workspace`
   plugin, and an unknown name would show in `warnings` on re-link anyway - so
   the only thing left to prove is process survival. Add a temporary
   `[[events]] on = "workspace.focused"` hook whose command runs a tiny detached
   writer and returns. Trigger a workspace focus. Confirm: (a) herdr is not
   blocked (the event command returns immediately), (b) the writer's output
   appears, and (c) the detached process is not reaped when the event completes.
   Once `start` exists, re-run against `./herdr-whistle start` and confirm the
   `run` worker survives.
2. **Single-instance (the critical correctness test).** Covers all four gaps:
   (a) run `start` twice concurrently -> exactly one bot polls, the second exits
   with the "already running" log; (b) while one bot runs, launch
   `./herdr-whistle` directly by hand -> it exits with the "already running" log
   and no Telegram 409 appears; (c) `kill -9` the worker -> it dies and releases
   the lock; fire a workspace event -> a fresh worker starts; (d) confirm via
   logs / Telegram that `getUpdates` never conflicts.
3. **Lazy respawn.** Kill the worker; confirm it stays down; trigger a workspace
   focus; confirm `start` relaunches it and the lock/pidfile are re-established.
4. **Clean stop.** Run the `stop` action; assert the bot exits and the pidfile is
   cleared; a subsequent `stop` prints "not running".
5. **herdr-down resilience.** `herdr server stop`; confirm the bot process stays
   up, logs (throttled), and resumes notifications after herdr is started again.
6. **Existing tests still pass.** `gofmt`, `go vet`, `go build`, `go test ./...`
   (per the project build command).

## Risks / open questions

- **Single-instance safety.** Resolved by design: the bot holds a canonical,
  machine-global `flock` acquired before Telegram polling (component 3), so
  exactly one instance runs across all launch paths. Verified in plan step 2.
- **Event names / required fields.** Resolved by docs: `workspace.created` /
  `workspace.focused` are valid (socket-api event list; corroborated by the
  `third774/herdr-last-workspace` plugin), and `[[events]]` takes no `id`.
- **Does herdr reap or kill event child processes?** Still open - the docs do not
  say event commands are backgrounded. This is the one thing smoke-test step 1
  must prove. `setsid` (new session) should protect the worker; verify.
- **Crash-recovery latency.** Accepted by the user (lazy respawn). The optional
  in-process `recover` (component 6) covers panic-class crashes immediately.
- **`flock` implementation.** Use `golang.org/x/sys/unix.Flock` (portable across
  Linux and macOS) rather than the stdlib `syscall.Flock`, which is deprecated on
  some targets. Small dependency add to `go.mod`.
- **Log file growth.** `bot.log` is append-only under the canonical dir.
  Acceptable for now; note for future log rotation.

## File changes summary

- `lifecycle.go` - new file, `package main`: `runStart` (probe + detached spawn),
  `runStop` (pidfile SIGTERM), lock acquire/release helpers, canonical state-dir
  resolution.
- `main.go` - subcommand dispatch (`start` / `stop` / default `run`); on the run
  path, acquire the instance lock and write the pidfile before `startBot`.
- `herdr-plugin.toml` - `start`/`stop` actions use subcommands; add
  `workspace.created` and `workspace.focused` event hooks.
- `watcher.go` - optional: throttle herdr-down WARN logging in `refresh()`.
- `go.mod` - optional: add `golang.org/x/sys` for portable `flock`.
- `AGENTS.md` - document the autostart mechanism, subcommands, and `start`/`stop`
  actions.
- `docs/superpowers/specs/2026-07-04-herdr-native-autostart-design.md` - this
  spec.

No shell scripts. No changes to the bot's command handlers or config format
(except the optional log-throttle). The autostart + single-instance logic is all
Go, in `lifecycle.go` + `main.go`.
