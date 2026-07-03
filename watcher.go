package main

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
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

func readPaneText(paneID string, lines int) string {
	raw, err := herdrAgentRead(paneID, lines)
	if err != nil {
		return ""
	}
	var envelope agentReadEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return ""
	}
	var readResult agentReadResult
	if err := json.Unmarshal(envelope.Result, &readResult); err != nil {
		return ""
	}
	return strings.TrimSpace(readResult.Read.Text)
}

var notifyBlocked = func(ctx context.Context, b *bot.Bot, chatID int64, a agentInfo) {
	shortCwd := shortenPath(a.Cwd)
	paneID := a.PaneID

	var sb strings.Builder
	sb.WriteString("⏸ <b>")
	sb.WriteString(escapeHTML(a.Agent))
	sb.WriteString("</b> is blocked and waiting for input.\n")
	if shortCwd != "" {
		sb.WriteString("   ")
		sb.WriteString(escapeHTML(shortCwd))
		sb.WriteString("\n")
	}
	sb.WriteString("Pane: ")
	sb.WriteString(escapeHTML(paneID))
	if a.Focused {
		sb.WriteString(" 👁")
	}

	// @clack/prompts redraws the selection in raw mode, so the choices may be
	// at the upper boundary of the scrollback buffer.
	text := readPaneText(paneID, 500)

	// Try @clack/prompts format parser first, then box-drawing format.
	pc := parseChoices(text)
	if pc == nil {
		pc = parseBoxChoices(text)
	}
	if pc != nil {
		sb.WriteString("\n\n<b>")
		sb.WriteString(escapeHTML(pc.Prompt))
		sb.WriteString("</b>")

		sb.WriteString("\n")
		for i, c := range pc.Choices {
			sb.WriteString("\n" + strconv.Itoa(i+1) + ". " + escapeHTML(c.CleanText))
		}

		msg := sb.String()
		kb := buildChoiceKeyboard(pc, paneID)
		params := &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        msg,
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: kb,
		}
		if _, err := b.SendMessage(ctx, params); err != nil {
			log.Printf("ERROR sending blocked notification with choices: %v", err)
		}
		return
	}

	if text != "" {
		lines := strings.Split(text, "\n")
		tail := 3
		if len(lines) < tail {
			tail = len(lines)
		}
		contextText := strings.TrimSpace(strings.Join(lines[len(lines)-tail:], "\n"))
		if contextText != "" {
			sb.WriteString("\n\n<code>")
			sb.WriteString(escapeHTML(contextText))
			sb.WriteString("</code>")
		}
	}

	sendFormatted(ctx, b, chatID, sb.String())
}


