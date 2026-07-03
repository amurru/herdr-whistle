package main

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"
)

// parsedChoice represents a single option in a selection prompt.
type parsedChoice struct {
	RawText   string // original line text
	CleanText string // text with selection indicator stripped
}

// parsedChoices holds the parsed selection prompt.
type parsedChoices struct {
	Prompt  string         // the question line (e.g. "? How to proceed")
	Help    string         // bottom bar (e.g. "Enter to select")
	Multi   bool           // true if multi-select (Space-based)
	Choices []parsedChoice // the available options
}

// choiceIndicators matches lines that start with a selection cursor/symbol.
var choiceIndicators = regexp.MustCompile(`^[\x{276F}\x{25C6}\x{25CF}\x{25CB}\x{25FB}\x{25A0}\x{25C7}]`)

// helpBarPattern matches the bottom help bar line (case-insensitive for "enter").
var helpBarPattern = regexp.MustCompile(`(?i)(?:enter|space).*(?:select|choose)|(?:↑|↓).*(?:select|choose|submit|navigate)`)

// separatorPattern matches divider/separator lines.
var separatorPattern = regexp.MustCompile(`^[─═\-\s＿_]{4,}$`)

// choiceLineRe matches a numbered choice line: "N. text..." or "N) text..."
var choiceLineRe = regexp.MustCompile(`^(\d+)[.)]\s+(.*)`)

// parseChoices scans terminal output for a @clack/prompts style selection menu
// and returns the parsed choices, or nil if no selection prompt is found.
func parseChoices(output string) *parsedChoices {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// --- 1. Find the help bar from the bottom ---
	helpIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		stripped := strings.TrimSpace(lines[i])
		if helpBarPattern.MatchString(stripped) ||
			strings.HasPrefix(stripped, "Enter to") ||
			strings.HasPrefix(stripped, "Space to") {
			helpIdx = i
			break
		}
	}
	if helpIdx < 0 {
		return nil
	}

	helpLine := strings.TrimSpace(lines[helpIdx])
	multi := strings.Contains(helpLine, "Space")

	// --- 2. Find the prompt question above the help bar ---
	prompt := ""
	promptIdx := -1
	for i := helpIdx - 1; i >= 0; i-- {
		stripped := strings.TrimSpace(lines[i])
		if strings.HasPrefix(stripped, "?") {
			prompt = stripped
			promptIdx = i
			break
		}
	}

	// --- 3. Collect choice lines between prompt and help ---
	var choices []parsedChoice
	for i := promptIdx + 1; i < helpIdx; i++ {
		stripped := strings.TrimSpace(lines[i])
		if stripped == "" {
			continue
		}
		if separatorPattern.MatchString(stripped) {
			continue
		}
		if strings.Contains(stripped, "Enter to") ||
			strings.Contains(stripped, "Space to") ||
			strings.Contains(stripped, "navigate") {
			continue
		}

		// Clean the choice text: strip the leading indicator character.
		clean := choiceIndicators.ReplaceAllString(stripped, "")
		clean = strings.TrimSpace(clean)
		if clean == "" {
			continue
		}

		choices = append(choices, parsedChoice{
			RawText:   lines[i],
			CleanText: clean,
		})
	}

	if len(choices) == 0 {
		return nil
	}

	return &parsedChoices{
		Prompt:  prompt,
		Help:    helpLine,
		Multi:   multi,
		Choices: choices,
	}
}

// parseBoxChoices scans terminal output for a box-drawing style selection
// menu used by agents (opencode, Claude Code) and returns parsed choices.
// The format uses ┃ (U+2502) characters as a left border:
//
//	┃
//	┃  Question text
//	┃
//	┃  1. Choice A text
//	┃  2. Choice B text
//	┃  3. Type your own answer
//	┃
func parseBoxChoices(output string) *parsedChoices {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// Extract content from lines starting with ┃ (box-drawing character)
	var contents []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "\u2502") {
			continue
		}
		content := strings.TrimLeft(strings.TrimPrefix(trimmed, "\u2502"), " ")
		contents = append(contents, content)
	}

	if len(contents) < 4 {
		return nil
	}

	// Find the question prompt: first non-empty, non-separator text
	// that does not start with a numbered choice pattern.
	prompt := ""
	promptIdx := -1
	for i, c := range contents {
		trimmed := strings.TrimSpace(c)
		if trimmed == "" {
			continue
		}
		if separatorPattern.MatchString(trimmed) {
			continue
		}
		if choiceLineRe.MatchString(trimmed) {
			continue
		}
		prompt = trimmed
		promptIdx = i
		break
	}
	if prompt == "" || promptIdx < 0 {
		return nil
	}

	// Collect numbered choice lines (N. text or N) text) after the prompt.
	var choices []parsedChoice
	for i := promptIdx + 1; i < len(contents); i++ {
		trimmed := strings.TrimSpace(contents[i])
		if trimmed == "" {
			continue
		}
		if separatorPattern.MatchString(trimmed) {
			continue
		}
		if matches := choiceLineRe.FindStringSubmatch(trimmed); matches != nil {
			choices = append(choices, parsedChoice{
				RawText:   trimmed,
				CleanText: strings.TrimSpace(matches[2]),
			})
		}
	}

	if len(choices) == 0 {
		return nil
	}

	return &parsedChoices{
		Prompt:  prompt,
		Choices: choices,
	}
}

const choiceCallbackPrefix = "ch|"

// buildChoiceKeyboard builds an inline keyboard from parsed choices.
// Each button's callback data is "ch|{paneID}|{index}" (1-based).
// Buttons show just the choice index number so they fit compactly.
func buildChoiceKeyboard(pc *parsedChoices, paneID string) *models.InlineKeyboardMarkup {
	const buttonsPerRow = 5
	var rows [][]models.InlineKeyboardButton
	var row []models.InlineKeyboardButton

	for i := range pc.Choices {
		label := strconv.Itoa(i + 1)
		btn := models.InlineKeyboardButton{
			Text:         label,
			CallbackData: choiceCallbackPrefix + paneID + "|" + label,
		}
		row = append(row, btn)
		if len(row) >= buttonsPerRow {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}
