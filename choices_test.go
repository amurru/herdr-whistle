package main

import (
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
