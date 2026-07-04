package main

import (
	"fmt"
	"strconv"
	"testing"
)

func TestParseBoxChoicesBasic(t *testing.T) {
	input := "\u2502\n\u2502  Which color do you prefer?\n\u2502\n\u2502  1. Red\n\u2502  2. Blue\n\u2502  3. Green\n\u2502\n"
	pc := parseBoxChoices(input)
	if pc == nil {
		t.Fatal("parseBoxChoices returned nil, expected valid parsedChoices")
	}
	if pc.Prompt != "Which color do you prefer?" {
		t.Errorf("expected prompt 'Which color do you prefer?', got '%s'", pc.Prompt)
	}
	if len(pc.Choices) != 3 {
		t.Fatalf("expected 3 choices, got %d", len(pc.Choices))
	}
	if pc.Choices[0].CleanText != "Red" {
		t.Errorf("expected choice 0 'Red', got '%s'", pc.Choices[0].CleanText)
	}
	if pc.Choices[1].CleanText != "Blue" {
		t.Errorf("expected choice 1 'Blue', got '%s'", pc.Choices[1].CleanText)
	}
	if pc.Choices[2].CleanText != "Green" {
		t.Errorf("expected choice 2 'Green', got '%s'", pc.Choices[2].CleanText)
	}
}

func TestParseBoxChoicesWithContinuationLines(t *testing.T) {
	input := "\u2502\n\u2502  Which framework?\n\u2502\n\u2502  1. React\n\u2502     A JavaScript library for building user interfaces\n\u2502  2. Vue\n\u2502     Another framework with a gentle learning curve\n\u2502  3. Svelte\n\u2502\n"
	pc := parseBoxChoices(input)
	if pc == nil {
		t.Fatal("parseBoxChoices returned nil, expected valid parsedChoices")
	}
	if pc.Prompt != "Which framework?" {
		t.Errorf("expected prompt 'Which framework?', got '%s'", pc.Prompt)
	}
	if len(pc.Choices) != 3 {
		t.Fatalf("expected 3 choices, got %d", len(pc.Choices))
	}
	if pc.Choices[0].CleanText != "React" {
		t.Errorf("expected choice 0 'React', got '%s'", pc.Choices[0].CleanText)
	}
	if pc.Choices[1].CleanText != "Vue" {
		t.Errorf("expected choice 1 'Vue', got '%s'", pc.Choices[1].CleanText)
	}
	if pc.Choices[2].CleanText != "Svelte" {
		t.Errorf("expected choice 2 'Svelte', got '%s'", pc.Choices[2].CleanText)
	}
}

func TestParseBoxChoicesWithParentheticalNumbering(t *testing.T) {
	input := "\u2502\n\u2502  Pick one:\n\u2502\n\u2502  1) Option Alpha\n\u2502  2) Option Beta\n\u2502  3) Option Gamma\n\u2502\n"
	pc := parseBoxChoices(input)
	if pc == nil {
		t.Fatal("parseBoxChoices returned nil")
	}
	if len(pc.Choices) != 3 {
		t.Fatalf("expected 3 choices, got %d", len(pc.Choices))
	}
	if pc.Choices[0].CleanText != "Option Alpha" {
		t.Errorf("expected 'Option Alpha', got '%s'", pc.Choices[0].CleanText)
	}
}

func TestParseBoxChoicesWithTypeYourOwn(t *testing.T) {
	input := "\u2502\n\u2502  How to proceed?\n\u2502\n\u2502  1. Continue\n\u2502  2. Stop\n\u2502  3. Type your own answer\n\u2502\n"
	pc := parseBoxChoices(input)
	if pc == nil {
		t.Fatal("parseBoxChoices returned nil")
	}
	if len(pc.Choices) != 3 {
		t.Fatalf("expected 3 choices, got %d", len(pc.Choices))
	}
	if pc.Choices[2].CleanText != "Type your own answer" {
		t.Errorf("expected 'Type your own answer', got '%s'", pc.Choices[2].CleanText)
	}
}

func TestParseBoxChoicesWithHeaderLines(t *testing.T) {
	input := "\u2192 Asked 1 question\n\u25A3  Sisyphus - Ultraworker \u00B7 Big Pickle\n\u2502\n\u2502  Which approach?\n\u2502\n\u2502  1. Option A\n\u2502     Description of option A\n\u2502  2. Option B\n\u2502  3. Type your own answer\n\u2502\n"
	pc := parseBoxChoices(input)
	if pc == nil {
		t.Fatal("parseBoxChoices returned nil")
	}
	if pc.Prompt != "Which approach?" {
		t.Errorf("expected prompt 'Which approach?', got '%s'", pc.Prompt)
	}
	if len(pc.Choices) != 3 {
		t.Fatalf("expected 3 choices, got %d", len(pc.Choices))
	}
}

func TestParseBoxChoicesNoBoxContent(t *testing.T) {
	input := "just some random text\nwith no box drawing chars"
	pc := parseBoxChoices(input)
	if pc != nil {
		t.Fatal("expected nil for non-box content")
	}
}

func TestParseBoxChoicesTooFewLines(t *testing.T) {
	input := "\u2502\n\u2502  only one line\n"
	pc := parseBoxChoices(input)
	if pc != nil {
		t.Fatal("expected nil for too few lines")
	}
}

func TestParseBoxChoicesNoChoices(t *testing.T) {
	input := "\u2502\n\u2502  Question only?\n\u2502\n\u2502  No actual choices here\n"
	pc := parseBoxChoices(input)
	if pc != nil {
		t.Fatal("expected nil when no numbered choices present")
	}
}

func TestParseBoxChoicesReturnsCorrectPrompt(t *testing.T) {
	input := "\u2502\n\u2502  What is your favorite programming language?\n\u2502\n\u2502  1. Go\n\u2502  2. Rust\n\u2502  3. TypeScript\n\u2502\n"
	pc := parseBoxChoices(input)
	if pc == nil {
		t.Fatal("parseBoxChoices returned nil")
	}
	if pc.Prompt != "What is your favorite programming language?" {
		t.Errorf("expected 'What is your favorite programming language?', got '%s'", pc.Prompt)
	}
}

// TestParseChoicesClackSingleSelect guards the @clack single-select format:
// only the active option carries the \u276f cursor; inactive options are plain.
// All options must be captured -- a naive "require a cursor" filter would
// drop the inactive ones.
func TestParseChoicesClackSingleSelect(t *testing.T) {
	input := "? Select a framework\n" +
		"\u2502\n" +
		"\u2502  \u276f  Next.js\n" +
		"\u2502     Nuxt\n" +
		"\u2502     Remix\n" +
		"\u2502\n" +
		"\u2514  \u2191\u2193 to navigate \u00b7 enter to select\n"
	pc := parseChoices(input)
	if pc == nil {
		t.Fatal("parseChoices returned nil")
	}
	if pc.Prompt != "? Select a framework" {
		t.Errorf("expected prompt '? Select a framework', got '%s'", pc.Prompt)
	}
	if len(pc.Choices) != 3 {
		t.Fatalf("expected 3 choices, got %d: %+v", len(pc.Choices), pc.Choices)
	}
	want := []string{"Next.js", "Nuxt", "Remix"}
	for i, w := range want {
		if pc.Choices[i].CleanText != w {
			t.Errorf("choice %d: expected '%s', got '%s'", i, w, pc.Choices[i].CleanText)
		}
	}
}

// TestParseChoicesStripsBorderAndCursor confirms the \u276f cursor and \u2502 border
// are stripped from the active option's clean text.
func TestParseChoicesStripsBorderAndCursor(t *testing.T) {
	input := "? Pick one\n\n\u276f  First\n   Second\n   Third\n\nenter to select\n"
	pc := parseChoices(input)
	if pc == nil {
		t.Fatal("parseChoices returned nil")
	}
	if pc.Choices[0].CleanText != "First" {
		t.Errorf("expected 'First' (cursor stripped), got '%s'", pc.Choices[0].CleanText)
	}
}

// TestParseChoicesNoHelpBar: without a recognizable help bar there is no menu.
func TestParseChoicesNoHelpBar(t *testing.T) {
	input := "? Pick one\n\u276f  A\n   B\n"
	if pc := parseChoices(input); pc != nil {
		t.Fatalf("expected nil for input with no help bar, got %+v", pc)
	}
}

// TestParseChoicesAllCursors: when every option carries a cursor (multiselect
// style with \u25cf/\u25cb), all are captured and the cursors stripped.
func TestParseChoicesAllCursors(t *testing.T) {
	input := "? Select items\n" +
		"\u2502\n" +
		"\u2502  \u25cf  Option A\n" +
		"\u2502  \u25cb  Option B\n" +
		"\u2502\n" +
		"\u2514  \u2191\u2193 navigate \u00b7 space to select \u00b7 enter to submit\n"
	pc := parseChoices(input)
	if pc == nil {
		t.Fatal("parseChoices returned nil")
	}
	if len(pc.Choices) != 2 {
		t.Fatalf("expected 2 choices, got %d: %+v", len(pc.Choices), pc.Choices)
	}
	if pc.Choices[0].CleanText != "Option A" {
		t.Errorf("choice 0: got '%s'", pc.Choices[0].CleanText)
	}
	if pc.Choices[1].CleanText != "Option B" {
		t.Errorf("choice 1: got '%s'", pc.Choices[1].CleanText)
	}
}

// TestBuildChoiceKeyboard verifies the layout (5 per row) and the 1-based
// "ch|{paneID}|{index}" callback data that choiceCallbackHandler relies on.
func TestBuildChoiceKeyboard(t *testing.T) {
	pc := &parsedChoices{Choices: []parsedChoice{
		{"A"}, {"B"}, {"C"}, {"D"}, {"E"}, {"F"},
	}}
	kb := buildChoiceKeyboard(pc, "wA:p1")
	if kb == nil {
		t.Fatal("nil keyboard")
	}
	// 6 choices, 5 per row -> 2 rows.
	if got := len(kb.InlineKeyboard); got != 2 {
		t.Fatalf("expected 2 rows, got %d", got)
	}
	if got := len(kb.InlineKeyboard[0]); got != 5 {
		t.Fatalf("expected 5 buttons in row 0, got %d", got)
	}
	if got := len(kb.InlineKeyboard[1]); got != 1 {
		t.Fatalf("expected 1 button in row 1, got %d", got)
	}
	for i, btn := range kb.InlineKeyboard[0] {
		wantData := fmt.Sprintf("ch|wA:p1|%d", i+1)
		if btn.CallbackData != wantData {
			t.Errorf("row0[%d] data=%q, want %q", i, btn.CallbackData, wantData)
		}
		if btn.Text != strconv.Itoa(i+1) {
			t.Errorf("row0[%d] text=%q, want %q", i, btn.Text, strconv.Itoa(i+1))
		}
	}
	if kb.InlineKeyboard[1][0].CallbackData != "ch|wA:p1|6" {
		t.Errorf("row1[0] data=%q, want \"ch|wA:p1|6\"", kb.InlineKeyboard[1][0].CallbackData)
	}
}
