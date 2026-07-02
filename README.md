# Herdr Whistle

Herdr plugin for remote agent management. List agents, read their output, send text to unblock them, close panes, and start new agents -- all from Telegram.

## Prerequisites

- Go 1.26+
- [herdr](https://herdr.dev) 0.7.0+
- A Telegram bot token (from [@BotFather](https://t.me/BotFather))
- Your Telegram user ID

## Installation

### 1. Build the binary

```sh
git clone https://github.com/amurru/herdr-whistle
cd herdr-whistle
go build -o herdr-whistle .
```

### 2. Link the plugin in herdr

```sh
herdr plugin link /path/to/herdr-whistle
```

### 3. Create configuration

herdr sets `HERDR_PLUGIN_CONFIG_DIR` automatically. Find your plugin config directory:

```sh
herdr plugin config-dir herdr-whistle
```

Create `config.toml` in that directory:

```toml
token = "1234567890:ABCdefGHIjklmNOPqrStuVWXyz"
owner_id = 00000000
```

| Field      | Description                                                          |
| ---------- | -------------------------------------------------------------------- |
| `token`    | Bot token from [@BotFather](https://t.me/BotFather)                  |
| `owner_id` | Your Telegram user ID (integer) -- only this user can issue commands |

## Usage

### Start the plugin

```sh
herdr plugin action invoke herdr-whistle.start
```

The plugin runs as a persistent daemon inside herdr. It polls Telegram for updates and shuts down cleanly when herdr stops or sends SIGTERM.

### Check status

```sh
herdr plugin log list --plugin herdr-whistle
```

Look for a log with `"status":"running"`.

### Stop the plugin

The plugin stops automatically when herdr exits. To stop it manually, kill the process:

```sh
pkill -f herdr-whistle
```

### View logs

herdr captures plugin stderr as log output:

```sh
herdr plugin log list --plugin herdr-whistle --limit 10
```

For real-time output, check herdr's plugin pane or tail the plugin log file.

### Update the plugin

After pulling changes or rebuilding:

```sh
go build -o herdr-whistle .
pkill -f herdr-whistle
herdr plugin action invoke herdr-whistle.start
```

## Commands

All commands are restricted to the configured `owner_id`. Unauthorized users receive a silent "Unauthorized" response.

| Command        | Arguments            | Description                                    |
| -------------- | -------------------- | ---------------------------------------------- |
| `/start`       | --                   | Welcome message and command list               |
| `/help`        | --                   | Available commands                             |
| `/agents`      | --                   | List all agents as JSON                        |
| `/status`      | `<target>`           | Show agent status and explanation              |
| `/read`        | `<target>` `[N]`     | Read last N lines of agent output (default 20) |
| `/send`        | `<target>` `<text>`  | Send text to an agent (unblocks it)            |
| `/close`       | `<target>`           | Close an agent's terminal pane                 |
| `/startagent` | `<name> -- <cmd...>` | Start a new agent with a command               |

### Examples

```
/status my-agent
/read my-agent 50
/send my-agent continue solving the problem
/close my-agent
/startagent code-helper -- python3 -m venv .venv
```

### Command details

**`/send`** is the primary way to unblock agents. When an agent is waiting for user input, `/send <target> <text>` sends the text followed by Enter, executing it in the agent's terminal.

**`/close`** gets the agent's pane ID from `herdr agent get` and runs `herdr pane close <pane_id>`. The agent process is killed. Start a new one with `/startagent`.

**`/startagent`** parses arguments: if `--` separator is present, everything before is the agent name and everything after is the command. Without `--`, the first word is the agent name and the rest is the command.

**`/read`** fetches output via `herdr agent read --source recent-unwrapped` to show the agent's most recent lines.

## Architecture

```
Telegram  <--long-poll-->  herdr-whistle  --herdr CLI-->  herdr daemon
   user                    (herdr plugin)
```

- The plugin connects to Telegram via long-polling (no webhooks)
- All herdr interactions use the `herdr` CLI via `os/exec` with 30-second timeouts
- Authentication is enforced server-side: only `owner_id` can invoke commands
- The binary is a single self-contained executable, no runtime dependencies

## Files

| File                | Purpose                                                            |
| ------------------- | ------------------------------------------------------------------ |
| `herdr-plugin.toml` | Plugin manifest (id, version, start action)                        |
| `main.go`           | Entry point, config loading, signal handling                       |
| `bot.go`            | Telegram bot setup, handler registration, message senders          |
| `handlers.go`       | Command handlers, owner auth, MarkdownV2 escaping                  |
| `herdrcli.go`       | herdr CLI command wrappers (agent list/get/read/send, pane run/close) |
| `config.go`         | TOML config loading                                                |
| `watcher.go`        | Agent status watcher -- notifies on blocked transitions            |
| `go.mod` / `go.sum` | Go module dependencies                                             |

## Security

- Only the configured `owner_id` can interact with the bot
- No command parsing or execution beyond `herdr` CLI calls
- herdr CLI is invoked via the `HERDR_BIN_PATH` env var (falls back to `herdr` PATH lookup)
- All bot messages use plain text or properly escaped HTML
