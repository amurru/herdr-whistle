package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// TestParseStartAgentArgs covers the /startagent tokenization, including the
// cases the old strings.Contains(rest, "--") splitter got wrong: a "--" inside
// a command flag must not be treated as the name/command separator.
func TestParseStartAgentArgs(t *testing.T) {
	tests := []struct {
		in      string
		name    string
		cmdArgs []string
	}{
		{"myagent", "myagent", nil},
		{"myagent echo hi", "myagent", []string{"echo", "hi"}},
		{"myagent -- echo hi", "myagent", []string{"echo", "hi"}},
		// Flag after the real separator is preserved verbatim.
		{"myagent -- echo --verbose", "myagent", []string{"echo", "--verbose"}},
		// No "--" at all: everything after the name is the command, flags kept.
		{"myagent claude --model x", "myagent", []string{"claude", "--model", "x"}},
		// Trailing separator with nothing after -> no command args.
		{"myagent --", "myagent", nil},
		{"", "", nil},
		{"   ", "", nil},
	}
	for _, tt := range tests {
		name, cmdArgs := parseStartAgentArgs(tt.in)
		if name != tt.name {
			t.Errorf("parseStartAgentArgs(%q) name = %q, want %q", tt.in, name, tt.name)
		}
		if !reflect.DeepEqual(cmdArgs, tt.cmdArgs) {
			t.Errorf("parseStartAgentArgs(%q) cmdArgs = %v, want %v", tt.in, cmdArgs, tt.cmdArgs)
		}
	}
}

// TestChoiceKeys locks in the arrow-key sequence that drives TUI selection
// menus: option 1 is just Enter, option N is (N-1) Downs then Enter.
func TestChoiceKeys(t *testing.T) {
	tests := []struct {
		idx  int
		want []string
	}{
		{1, []string{"Enter"}},
		{2, []string{"Down", "Enter"}},
		{3, []string{"Down", "Down", "Enter"}},
		{5, []string{"Down", "Down", "Down", "Down", "Enter"}},
	}
	for _, tt := range tests {
		got := choiceKeys(tt.idx)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("choiceKeys(%d) = %v, want %v", tt.idx, got, tt.want)
		}
	}
}

func TestFormatAgentFromGet(t *testing.T) {
	t.Run("valid agent", func(t *testing.T) {
		in := `{"id":"cli:agent:get","result":{"agent":{"agent":"claude","agent_status":"blocked","pane_id":"wA:p1","workspace_id":"wA","cwd":"/home/user/proj"}}}`
		got := formatAgentFromGet(in)
		want := "<b>claude</b>\n\nStatus: blocked\nPane: wA:p1\nWorkspace: wA\nCwd: /home/user/proj"
		if got != want {
			t.Errorf("formatAgentFromGet(valid) = %q, want %q", got, want)
		}
	})
	t.Run("unparseable falls back to raw escaped", func(t *testing.T) {
		got := formatAgentFromGet("not json <at> all")
		want := "<pre><code>not json &lt;at&gt; all</code></pre>"
		if got != want {
			t.Errorf("formatAgentFromGet(fallback) = %q, want %q", got, want)
		}
	})
	t.Run("escapes HTML in fields", func(t *testing.T) {
		in := `{"id":"x","result":{"agent":{"agent":"<a&b>","agent_status":"idle","pane_id":"p","workspace_id":"w","cwd":"/c"}}}`
		got := formatAgentFromGet(in)
		if !strings.Contains(got, "<b>&lt;a&amp;b&gt;</b>") {
			t.Errorf("expected escaped agent name in %q", got)
		}
	})
}

func TestShortenPathIn(t *testing.T) {
	tests := []struct {
		path, home, want string
	}{
		{"/home/user/proj", "/home/user", "~/proj"},
		{"/home/user", "/home/user", "~"},
		{"/home/user/sub/deep", "/home/user", "~/sub/deep"},
		// Boundary: /home/user must not match /home/user2.
		{"/home/user2/proj", "/home/user", "/home/user2/proj"},
		{"/var/other", "/home/user", "/var/other"},
		{"/anything", "", "/anything"},
	}
	for _, tt := range tests {
		got := shortenPathIn(tt.path, tt.home)
		if got != tt.want {
			t.Errorf("shortenPathIn(%q, %q) = %q, want %q", tt.path, tt.home, got, tt.want)
		}
	}
}

func TestEscapeHTML(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", "plain"},
		{"a & b", "a &amp; b"},
		{"<tag>", "&lt;tag&gt;"},
		{"a<b>&c", "a&lt;b&gt;&amp;c"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := escapeHTML(tt.in); got != tt.want {
			t.Errorf("escapeHTML(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSanitizeTTY(t *testing.T) {
	// Control chars below 0x20 are stripped except \n, \r, \t.
	in := "clean\x00text\x07bell\nline2\r\ttab"
	want := "cleantextbell\nline2\r\ttab"
	if got := sanitizeTTY(in); got != want {
		t.Errorf("sanitizeTTY = %q, want %q", got, want)
	}
}

func TestParseCommandArgs(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"/send agent hi", []string{"/send", "agent", "hi"}},
		{"  /read   x   10  ", []string{"/read", "x", "10"}},
	}
	for _, tt := range tests {
		got := parseCommandArgs(tt.in)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parseCommandArgs(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// TestDefaultHandlerNilFromNoPanic guards the nil-From fix: a message whose
// From is nil (e.g. anonymous group admin) must not crash the bot.
func TestDefaultHandlerNilFromNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("defaultHandler panicked on nil From: %v", r)
		}
	}()
	// From is nil; the handler must return before touching the bot or cfgGlobal.
	defaultHandler(context.Background(), &bot.Bot{}, &models.Update{Message: &models.Message{}})
}

// TestOwnerAuthNilMessage: ownerAuth must reject updates without a Message/From
// without touching the bot.
func TestOwnerAuthNilMessage(t *testing.T) {
	if ownerAuth(context.Background(), &bot.Bot{}, &models.Update{}) {
		t.Error("expected ownerAuth=false for update with no Message")
	}
}

const emptyAgentListJSON = `{"id":"cli:agent:list","result":{"agents":[]}}`

func TestBuildAgentListEmpty(t *testing.T) {
	orig := herdrAgentList
	herdrAgentList = func() (string, error) { return emptyAgentListJSON, nil }
	defer func() { herdrAgentList = orig }()

	msg, kb, err := buildAgentList()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "No agents running") {
		t.Errorf("expected empty-list message, got %q", msg)
	}
	if kb != nil {
		t.Errorf("expected nil keyboard for empty list, got %+v", kb)
	}
}

func TestBuildAgentList(t *testing.T) {
	orig := herdrAgentList
	herdrAgentList = func() (string, error) {
		return fmt.Sprintf(sampleAgentListJSON, "idle", "idle"), nil
	}
	defer func() { herdrAgentList = orig }()

	msg, kb, err := buildAgentList()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "Agents (2)") {
		t.Errorf("expected count header, got %q", msg)
	}
	if !strings.Contains(msg, "claude") || !strings.Contains(msg, "opencode") {
		t.Errorf("expected both agent names in %q", msg)
	}
	if kb == nil || len(kb.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 keyboard rows (2 agents + refresh), got %v", kb)
	}
	// Row 0: status/read/close buttons wired to the first pane.
	row := kb.InlineKeyboard[0]
	if len(row) != 3 {
		t.Fatalf("expected 3 buttons in row 0, got %d", len(row))
	}
	want := []string{"al|status|wA:p1", "al|read|wA:p1", "al|close|wA:p1"}
	for i, w := range want {
		if row[i].CallbackData != w {
			t.Errorf("row0[%d] data=%q, want %q", i, row[i].CallbackData, w)
		}
	}
	// Last row: the refresh button.
	last := kb.InlineKeyboard[2]
	if len(last) != 1 || last[0].CallbackData != "al|refresh" {
		t.Errorf("unexpected last row: %+v", last)
	}
}
