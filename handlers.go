package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// ----- JSON types for herdr CLI responses -----

type agentListEnvelope struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
}

type agentListResult struct {
	Agents []agentInfo `json:"agents"`
}

type agentSession struct {
	Value string `json:"value"`
}

type agentInfo struct {
	Agent            string       `json:"agent"`
	AgentSession     agentSession `json:"agent_session"`
	AgentStatus      string       `json:"agent_status"`
	WorkspaceID      string       `json:"workspace_id"`
	PaneID           string       `json:"pane_id"`
	Cwd              string       `json:"cwd"`
	Focused          bool         `json:"focused"`
	ForegroundCwd    string       `json:"foreground_cwd"`
}

// agentGetEnvelope wraps the top-level herdr CLI response for agent get.
type agentGetEnvelope struct {
	ID     string               `json:"id"`
	Result agentGetNestedResult `json:"result"`
}

type agentGetNestedResult struct {
	Agent agentInfo `json:"agent"`
}// agentExplainResult is the herdr explain response (flat-ish).
type agentExplainResult struct {
	State string `json:"state"`
}

type agentReadEnvelope struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
}

type agentReadResult struct {
	Read agentReadContent `json:"read"`
}

type agentReadContent struct {
	Text string `json:"text"`
	PaneID string `json:"pane_id"`
}

// ----------------------------------------------

var cfgGlobal *Config

// ownerAuth checks whether the sender of update matches the configured owner.
// It sends an "Unauthorized" reply and returns false if the IDs do not match.
func ownerAuth(ctx context.Context, b *bot.Bot, update *models.Update) bool {
	if update.Message == nil || update.Message.From == nil {
		return false
	}
	if update.Message.From.ID != cfgGlobal.OwnerID {
		sendText(ctx, b, update.Message.Chat.ID, "Unauthorized")
		return false
	}
	return true
}

// escapeHTML escapes HTML special characters for safe use in Telegram HTML mode.
func escapeHTML(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(s)
}

// startHandler replies with a welcome message.
func startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if !ownerAuth(ctx, b, update) {
		return
	}
	msg := `Welcome to herdr-whistle -- Telegram remote control for herdr agents.

Commands:
/agents -- list all agents
/status <target> -- show agent status and explanation
/read <target> [N] -- read recent agent output (default 20 lines)
/send <target> <text> -- send text to an agent
/close <target> -- close an agent's pane
/start-agent <name> -- <cmd...> -- start a new agent
/help -- show this message`
	sendText(ctx, b, update.Message.Chat.ID, msg)
}

// shortenPath replaces the user's home directory with ~ for display.
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// buildAgentList calls herdr agent list, parses the JSON, and returns
// a formatted HTML message with an inline keyboard for each agent.
func buildAgentList(ctx context.Context) (string, *models.InlineKeyboardMarkup, error) {
	raw, err := herdrAgentList()
	if err != nil {
		return "", nil, err
	}

	var envelope agentListEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return "", nil, fmt.Errorf("parsing agent list JSON: %w", err)
	}

	var result agentListResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		return "", nil, fmt.Errorf("parsing agent list result: %w", err)
	}

	if len(result.Agents) == 0 {
		return "<b>Agents</b>\n\nNo agents running.", nil, nil
	}

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("<b>Agents (%d)</b>\n", len(result.Agents)))
	msg.WriteString("🔍 Status | 💬 Read | ✕ Close | 🔄 Refresh\n\n")

	nameCount := map[string]int{}
	for _, a := range result.Agents {
		nameCount[a.Agent]++
	}

	var rows [][]models.InlineKeyboardButton

	for _, a := range result.Agents {
		name := escapeHTML(a.Agent)
		cwdShort := shortenPath(a.Cwd)

		statusIcon := "💤"
		switch a.AgentStatus {
		case "working", "running":
			statusIcon = "⏳"
		case "done":
			statusIcon = "✅"
		}

		focusMark := ""
		if a.Focused {
			focusMark = " 👁"
		}

		disambiguator := ""
		if nameCount[a.Agent] > 1 {
			disambiguator = " " + escapeHTML(a.PaneID)
		}

		msg.WriteString(fmt.Sprintf("%s <b>%s</b>%s  %s  [%s]\n   %s\n",
			statusIcon, name, focusMark, a.AgentStatus, escapeHTML(a.PaneID), escapeHTML(cwdShort)))

		row := []models.InlineKeyboardButton{
			{Text: "🔍" + disambiguator, CallbackData: fmt.Sprintf("al|status|%s", a.PaneID)},
			{Text: "💬" + disambiguator, CallbackData: fmt.Sprintf("al|read|%s", a.PaneID)},
			{Text: "✕" + disambiguator, CallbackData: fmt.Sprintf("al|close|%s", a.PaneID)},
		}
		rows = append(rows, row)
	}

	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "🔄 Refresh", CallbackData: "al|refresh"},
	})

	kb := &models.InlineKeyboardMarkup{InlineKeyboard: rows}
	return msg.String(), kb, nil
}

// agentsHandler sends the agent list as a formatted message with inline buttons.
func agentsHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if !ownerAuth(ctx, b, update) {
		return
	}

	msg, kb, err := buildAgentList(ctx)
	if err != nil {
		log.Printf("ERROR building agent list: %v", err)
		sendText(ctx, b, update.Message.Chat.ID, "Error listing agents: "+err.Error())
		return
	}

	params := &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        msg,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: kb,
	}
	if _, err := b.SendMessage(ctx, params); err != nil {
		log.Printf("ERROR sending agent list: %v", err)
	}
}

// statusHandler shows agent status and explanation for a target.
func statusHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if !ownerAuth(ctx, b, update) {
		return
	}

	args := parseCommandArgs(update.Message.Text)
	if len(args) < 2 {
		sendText(ctx, b, update.Message.Chat.ID, "Usage: /status <target>")
		return
	}
	target := args[1]

	getOut, err := herdrAgentGet(target)
	if err != nil {
		log.Printf("ERROR getting agent %s: %v", target, err)
		sendText(ctx, b, update.Message.Chat.ID, "Error getting agent: "+err.Error())
		return
	}

	explainOut, err := herdrAgentExplain(target)
	if err != nil {
		log.Printf("ERROR explaining agent %s: %v", target, err)
		sendText(ctx, b, update.Message.Chat.ID, "Error explaining agent: "+err.Error())
		return
	}

	var sb strings.Builder
	sb.WriteString("<pre><code>")
	sb.WriteString(escapeHTML(getOut))
	sb.WriteString("</code></pre>\n\n")
	sb.WriteString("<pre><code>")
	sb.WriteString(escapeHTML(explainOut))
	sb.WriteString("</code></pre>")

	sendFormatted(ctx, b, update.Message.Chat.ID, sb.String())
}

// readHandler reads recent agent output.
func readHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if !ownerAuth(ctx, b, update) {
		return
	}

	args := parseCommandArgs(update.Message.Text)
	if len(args) < 2 {
		sendText(ctx, b, update.Message.Chat.ID, "Usage: /read <target> [N]")
		return
	}
	target := args[1]

	lines := 20
	if len(args) >= 3 {
		n, err := strconv.Atoi(args[2])
		if err == nil && n > 0 {
			lines = n
		}
	}

	out, err := herdrAgentRead(target, lines)
	if err != nil {
		log.Printf("ERROR reading agent %s: %v", target, err)
		sendText(ctx, b, update.Message.Chat.ID, "Error reading agent: "+err.Error())
		return
	}

	formatted := "<pre><code>" + escapeHTML(out) + "</code></pre>"
	sendFormatted(ctx, b, update.Message.Chat.ID, formatted)
}

// sendHandler sends text to an agent.
func sendHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if !ownerAuth(ctx, b, update) {
		return
	}

	args := parseCommandArgs(update.Message.Text)
	if len(args) < 3 {
		sendText(ctx, b, update.Message.Chat.ID, "Usage: /send <target> <text>")
		return
	}
	target := args[1]
	text := strings.Join(args[2:], " ")

	paneID, err := herdrPaneGetFromAgent(target)
	if err != nil {
		log.Printf("ERROR getting pane for agent %s: %v", target, err)
		sendText(ctx, b, update.Message.Chat.ID, "Error resolving agent: "+err.Error())
		return
	}
	out, err := herdrPaneRun(paneID, text)
	if err != nil {
		log.Printf("ERROR sending to agent %s: %v", target, err)
		sendText(ctx, b, update.Message.Chat.ID, "Error sending to agent: "+err.Error())
		return
	}

	reply := fmt.Sprintf("<b>Sent to %s:</b>\n%s", escapeHTML(target), escapeHTML(out))
	sendFormatted(ctx, b, update.Message.Chat.ID, reply)
}

// closeHandler closes an agent's pane.
func closeHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if !ownerAuth(ctx, b, update) {
		return
	}

	args := parseCommandArgs(update.Message.Text)
	if len(args) < 2 {
		sendText(ctx, b, update.Message.Chat.ID, "Usage: /close <target>")
		return
	}
	target := args[1]

	paneID, err := herdrPaneGetFromAgent(target)
	if err != nil {
		log.Printf("ERROR getting pane for agent %s: %v", target, err)
		sendText(ctx, b, update.Message.Chat.ID, "Error getting pane: "+err.Error())
		return
	}

	out, err := herdrPaneClose(paneID)
	if err != nil {
		log.Printf("ERROR closing pane %s: %v", paneID, err)
		sendText(ctx, b, update.Message.Chat.ID, "Error closing pane: "+err.Error())
		return
	}

	reply := fmt.Sprintf("Closed pane %s for agent %s:\n%s", escapeHTML(paneID), escapeHTML(target), escapeHTML(out))
	sendFormatted(ctx, b, update.Message.Chat.ID, reply)
}

// startAgentHandler starts a new agent with a command.
func startAgentHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if !ownerAuth(ctx, b, update) {
		return
	}

	text := strings.TrimSpace(update.Message.Text)
	// Remove "/start-agent" prefix
	rest := strings.TrimPrefix(text, "/start-agent")
	rest = strings.TrimSpace(rest)

	if rest == "" {
		sendText(ctx, b, update.Message.Chat.ID, "Usage: /start-agent <name> -- <cmd...>")
		return
	}

	// Split on " -- " to get name and command parts
	parts := strings.SplitN(rest, "--", 2)
	name := strings.TrimSpace(parts[0])
	if name == "" {
		sendText(ctx, b, update.Message.Chat.ID, "Usage: /start-agent <name> -- <cmd...>")
		return
	}

	var cmdArgs []string
	if len(parts) > 1 {
		cmdArgs = strings.Fields(strings.TrimSpace(parts[1]))
	}

	out, err := herdrAgentStart(name, cmdArgs...)
	if err != nil {
		log.Printf("ERROR starting agent %s: %v", name, err)
		sendText(ctx, b, update.Message.Chat.ID, "Error starting agent: "+err.Error())
		return
	}

	reply := fmt.Sprintf("Started agent %s:\n%s", escapeHTML(name), escapeHTML(out))
	sendFormatted(ctx, b, update.Message.Chat.ID, reply)
}

// helpHandler shows available commands.
func helpHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if !ownerAuth(ctx, b, update) {
		return
	}

	msg := `Available commands:

/agents -- list all agents (with inline buttons)
/status <target> -- show agent status and explanation
/read <target> [N] -- read recent output (default 20 lines)
/send <target> <text> -- send text to an agent
/close <target> -- close an agent's terminal pane
/start-agent <name> -- <cmd...> -- launch a new agent
/help -- show this message`

	sendText(ctx, b, update.Message.Chat.ID, msg)
}

// ----- Inline keyboard callback handler -----

const (
	cbPrefix = "al|" // agent list callbacks start with "al|"
)

// agentsCallbackHandler processes button presses on the agent list inline keyboard.
func agentsCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}
	data := update.CallbackQuery.Data
	if !strings.HasPrefix(data, cbPrefix) {
		return
	}

	// Extract chat/message from the MaybeInaccessibleMessage union.
	var chatID int64
	var msgID int
	if msg := update.CallbackQuery.Message.Message; msg != nil {
		chatID = msg.Chat.ID
		msgID = msg.ID
	} else if im := update.CallbackQuery.Message.InaccessibleMessage; im != nil {
		chatID = im.Chat.ID
		msgID = im.MessageID
	} else {
		return
	}
	userID := update.CallbackQuery.From.ID

	// Only the owner can interact with the buttons.
	if userID != cfgGlobal.OwnerID {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Unauthorized",
			ShowAlert:       true,
		})
		return
	}

	// Acknowledge the callback immediately to dismiss the loading spinner.
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	// Parse: al|action|paneID
	trimmed := strings.TrimPrefix(data, cbPrefix)
	parts := strings.SplitN(trimmed, "|", 2)
	action := parts[0]

	var paneID string
	if len(parts) > 1 {
		paneID = parts[1]
	}

	switch action {
	case "refresh":
		handleRefresh(ctx, b, chatID, msgID)
	case "status":
		handleAgentStatus(ctx, b, chatID, paneID)
	case "read":
		handleAgentRead(ctx, b, chatID, paneID)
	case "close":
		handleAgentClosePrompt(ctx, b, chatID, paneID)
	case "close_confirm":
		handleAgentCloseExec(ctx, b, chatID, msgID, paneID)
	case "close_cancel":
		handleAgentCloseCancel(ctx, b, chatID, msgID, paneID)
	}
}

func handleRefresh(ctx context.Context, b *bot.Bot, chatID int64, msgID int) {
	msg, kb, err := buildAgentList(ctx)
	if err != nil {
		log.Printf("ERROR rebuilding agent list: %v", err)
		editMessageText(ctx, b, chatID, msgID, "Error refreshing: "+escapeHTML(err.Error()))
		return
	}

	var kbRaw interface{}
	if kb != nil {
		kbRaw = kb
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   msgID,
		Text:        msg,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: kbRaw,
	})
}

func handleAgentStatus(ctx context.Context, b *bot.Bot, chatID int64, target string) {
	if target == "" {
		return
	}

	getOut, err := herdrAgentGet(target)
	if err != nil {
		sendText(ctx, b, chatID, "Error getting agent: "+err.Error())
		return
	}

	var env agentGetEnvelope
	if json.Unmarshal([]byte(getOut), &env) == nil && env.Result.Agent.Agent != "" {
		a := env.Result.Agent
		msg := fmt.Sprintf(
			"<b>%s</b>\n\nStatus: %s\nPane: %s\nWorkspace: %s\nCwd: %s",
			escapeHTML(a.Agent),
			a.AgentStatus,
			escapeHTML(a.PaneID),
			escapeHTML(a.WorkspaceID),
			escapeHTML(a.Cwd),
		)
		sendFormatted(ctx, b, chatID, msg)
	} else {
		// Fallback: show raw JSON if parsing fails.
		formatted := "<pre><code>" + escapeHTML(getOut) + "</code></pre>"
		sendFormatted(ctx, b, chatID, formatted)
	}
}

func handleAgentRead(ctx context.Context, b *bot.Bot, chatID int64, target string) {
	if target == "" {
		return
	}

	out, err := herdrAgentRead(target, 20)
	if err != nil {
		sendText(ctx, b, chatID, "Error reading agent: "+err.Error())
		return
	}

	// Try to extract just the text from the JSON envelope.
	var envelope agentReadEnvelope
	if json.Unmarshal([]byte(out), &envelope) == nil {
		var readResult agentReadResult
		if json.Unmarshal(envelope.Result, &readResult) == nil && readResult.Read.Text != "" {
			text := strings.TrimSpace(readResult.Read.Text)
			msg := fmt.Sprintf("<b>Output from %s:</b>\n<pre><code>%s</code></pre>",
				escapeHTML(target), escapeHTML(text))
			sendFormatted(ctx, b, chatID, msg)
			return
		}
	}

	// Fallback: show raw output.
	formatted := "<pre><code>" + escapeHTML(out) + "</code></pre>"
	sendFormatted(ctx, b, chatID, formatted)
}

func handleAgentClosePrompt(ctx context.Context, b *bot.Bot, chatID int64, paneID string) {
	if paneID == "" {
		return
	}

	msg := fmt.Sprintf("Close pane <b>%s</b>?", escapeHTML(paneID))
	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Yes", CallbackData: fmt.Sprintf("al|close_confirm|%s", paneID)},
				{Text: "No", CallbackData: fmt.Sprintf("al|close_cancel|%s", paneID)},
			},
		},
	}

	params := &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        msg,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: kb,
	}
	if _, err := b.SendMessage(ctx, params); err != nil {
		log.Printf("ERROR sending close prompt: %v", err)
	}
}

func handleAgentCloseExec(ctx context.Context, b *bot.Bot, chatID int64, msgID int, paneID string) {
	if paneID == "" {
		return
	}

	out, err := herdrPaneClose(paneID)
	if err != nil {
		editMessageText(ctx, b, chatID, msgID, "Error closing pane: "+escapeHTML(err.Error()))
		return
	}

	editMessageText(ctx, b, chatID, msgID,
		fmt.Sprintf("Pane <b>%s</b> closed.", escapeHTML(paneID)))
	_ = out
}

func handleAgentCloseCancel(ctx context.Context, b *bot.Bot, chatID int64, msgID int, paneID string) {
	if paneID == "" {
		return
	}
	editMessageText(ctx, b, chatID, msgID, "Cancelled.")
}

// editMessageText is a helper that edits a message's text (HTML).
func editMessageText(ctx context.Context, b *bot.Bot, chatID int64, msgID int, text string) {
	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: msgID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
}

// ---------------------------------------------

// defaultHandler replies to unrecognized messages.
func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	if update.Message.From.ID != cfgGlobal.OwnerID {
		sendText(ctx, b, update.Message.Chat.ID, "Unauthorized")
		return
	}
	sendText(ctx, b, update.Message.Chat.ID, "Unknown command. Use /help for available commands.")
}

// parseCommandArgs splits a message text into whitespace-separated tokens.
func parseCommandArgs(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	return strings.Fields(text)
}
