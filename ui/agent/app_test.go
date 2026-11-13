package agent

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHeaderLabel(t *testing.T) {
	cases := []struct{ agent, model, want string }{
		{"Eino", "gpt4", "Eino · gpt4"},
		{"Eino", "", "Eino"},
		{"", "gpt4", "gpt4"},
		{"", "", ""},
		{"Eino", "Eino", "Eino"},
		{" Eino ", " gpt4 ", "Eino · gpt4"},
	}
	for _, c := range cases {
		if got := headerLabel(c.agent, c.model); got != c.want {
			t.Fatalf("headerLabel(%q, %q) = %q, want %q", c.agent, c.model, got, c.want)
		}
	}
}

func TestHeaderShowsSubtitle(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 90, Height: 20})
	m.SetSubtitle(headerLabel("Eino", "gpt4"))

	out := m.View()
	if !strings.Contains(out, "Eino · gpt4") {
		t.Fatalf("header missing agent/model subtitle:\n%s", out)
	}
}

func TestHeaderTruncatesLongSubtitle(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	// Narrow enough that the full agent/model pair cannot fit next to the status.
	m = drive(m, tea.WindowSizeMsg{Width: 52, Height: 20})
	m.SetSubtitle(headerLabel("very-long-agent-name", "very-long-model-name"))

	lines := strings.Split(m.View(), "\n")
	first := lines[0]
	if !strings.Contains(first, "…") {
		t.Fatalf("long subtitle was not truncated:\n%s", first)
	}
	if !strings.Contains(first, "ready") {
		t.Fatalf("status pushed off the header row:\n%s", first)
	}
}
