package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func herdrBin() string {
	bin := os.Getenv("HERDR_BIN_PATH")
	if bin == "" {
		bin = "herdr"
	}
	return bin
}

func runCommand(ctx context.Context, args ...string) (string, error) {
	bin := herdrBin()
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("command %s %s failed: %w (stderr: %s)", bin, strings.Join(args, " "), err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("command %s %s failed: %w", bin, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

var herdrAgentList = func() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return runCommand(ctx, "agent", "list")
}

func herdrAgentGet(target string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return runCommand(ctx, "agent", "get", target)
}

func herdrAgentExplain(target string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return runCommand(ctx, "agent", "explain", target, "--json")
}

// herdrAgentRead reads recent agent output. The target may be either an agent
// name (as used by the /read command) or a pane ID (as used by the inline
// buttons and the watcher); herdr resolves both.
func herdrAgentRead(target string, lines int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lineArg := fmt.Sprintf("%d", lines)
	return runCommand(ctx, "agent", "read", target, "--source", "recent-unwrapped", "--lines", lineArg)
}

func herdrAgentStart(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmdArgs := append([]string{"agent", "start", name, "--"}, args...)
	return runCommand(ctx, cmdArgs...)
}

// herdrPaneGetFromAgent retrieves the pane_id from the JSON output of
// "herdr agent get <target>".
func herdrPaneGetFromAgent(target string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := runCommand(ctx, "agent", "get", target)
	if err != nil {
		return "", err
	}

	var result struct {
		Result struct {
			Agent struct {
				PaneID string `json:"pane_id"`
			} `json:"agent"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return "", fmt.Errorf("parsing agent get JSON for %s: %w", target, err)
	}
	if result.Result.Agent.PaneID == "" {
		return "", fmt.Errorf("no pane_id found for agent %s", target)
	}
	return result.Result.Agent.PaneID, nil
}

func herdrPaneRun(paneID, command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return runCommand(ctx, "pane", "run", paneID, command)
}

// herdrPaneSendKeys sends tmux-style key names (e.g. "Down", "Up", "Enter",
// "Space") to a pane. Used to drive interactive TUI selection menus that
// respond to arrow keys rather than typed digits.
func herdrPaneSendKeys(paneID string, keys ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return runCommand(ctx, append([]string{"pane", "send-keys", paneID}, keys...)...)
}

func herdrPaneClose(paneID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return runCommand(ctx, "pane", "close", paneID)
}
