package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
)

// agentWatcher polls herdr agent list and notifies when an agent becomes blocked.
// It tracks the last-known status per pane_id to detect transitions.
func agentWatcher(ctx context.Context, b *bot.Bot, chatID int64) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Track agent session keys to detect stale entries.
	// A session key uniquely identifies a running agent instance.
	type sessionKey struct {
		PaneID string
		// agent_session.value is a UUID that changes when an agent restarts.
		// We track it so if an agent pane is closed and a new one starts with
		// the same pane_id, we treat it as a fresh session.
		SessionID string
	}

	var mu sync.Mutex
	prevStatus := map[sessionKey]string{}

	refresh := func() {
		out, err := herdrAgentList()
		if err != nil {
			log.Printf("WARN agentWatcher: herdr agent list failed: %v", err)
			return
		}

		var env agentListEnvelope
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			log.Printf("WARN agentWatcher: parsing envelope: %v", err)
			return
		}
		var lr agentListResult
		if err := json.Unmarshal([]byte(env.Result), &lr); err != nil {
			log.Printf("WARN agentWatcher: parsing result: %v", err)
			return
		}

		mu.Lock()
		defer mu.Unlock()

		// Collect pane_ids seen in this poll.
		seen := map[string]bool{}
		for _, a := range lr.Agents {
			if a.PaneID == "" {
				continue
			}
			sk := sessionKey{PaneID: a.PaneID, SessionID: a.AgentSession.Value}
			seen[a.PaneID] = true

			oldStatus, exists := prevStatus[sk]
			currStatus := strings.ToLower(a.AgentStatus)

			if exists && oldStatus != "blocked" && currStatus == "blocked" {
				notifyBlocked(ctx, b, chatID, a)
			}

			prevStatus[sk] = currStatus
		}

		// Remove stale entries for panes that no longer exist.
		for sk := range prevStatus {
			if !seen[sk.PaneID] {
				delete(prevStatus, sk)
			}
		}
	}

	// Do an immediate refresh on start to seed state.
	refresh()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh()
		}
	}
}

// notifyBlocked sends a Telegram message when an agent becomes blocked.
var notifyBlocked = func(ctx context.Context, b *bot.Bot, chatID int64, a agentInfo) {
	shortCwd := shortenPath(a.Cwd)
	lines := strings.Builder{}
	lines.WriteString("⏸ <b>")
	lines.WriteString(escapeHTML(a.Agent))
	lines.WriteString("</b> is blocked and waiting for input.\n")
	if shortCwd != "" {
		lines.WriteString("   ")
		lines.WriteString(escapeHTML(shortCwd))
		lines.WriteString("\n")
	}
	lines.WriteString("Pane: ")
	lines.WriteString(escapeHTML(a.PaneID))

	if a.Focused {
		lines.WriteString(" 👁")
	}

	// Try to read the last line of agent output to give context.
	if text, err := herdrAgentRead(a.PaneID, 3); err == nil {
		text = strings.TrimSpace(text)
		if text != "" {
			lines.WriteString("\n\n<code>")
			lines.WriteString(escapeHTML(text))
			lines.WriteString("</code>")
		}
	}

	sendFormatted(ctx, b, chatID, lines.String())
}
