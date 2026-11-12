package agent

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestThemeSwitching(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	if m.ThemeName() != "violet" {
		t.Fatalf("default theme = %q, want violet", m.ThemeName())
	}

	// Force dark variant for deterministic assertions.
	if err := m.ApplyTheme("ocean", "dark"); err != nil {
		t.Fatal(err)
	}
	if !m.DarkMode() || m.ThemeName() != "ocean" {
		t.Fatalf("theme state wrong: %s dark=%v", m.ThemeName(), m.DarkMode())
	}
	if m.theme.Accent != "#2DD4BF" {
		t.Fatalf("dark accent = %q", m.theme.Accent)
	}

	// Light variant of the same theme has different colors.
	if err := m.ApplyTheme("ocean", "light"); err != nil {
		t.Fatal(err)
	}
	if m.DarkMode() {
		t.Fatal("expected light mode")
	}
	if m.theme.Accent != "#0D9488" {
		t.Fatalf("light accent = %q", m.theme.Accent)
	}

	// Unknown theme / mode fail with a helpful error.
	if err := m.ApplyTheme("nope", ""); err == nil {
		t.Fatal("unknown theme should fail")
	}
	if err := m.ApplyTheme("ocean", "blue"); err == nil {
		t.Fatal("invalid mode should fail")
	}
}

func TestThemePalette(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 110, Height: 30})
	m.SetSlashCommands([]SlashCommand{
		{Name: "theme", Args: "<name> [light|dark]", Description: "切换颜色主题", Values: themeNames()},
	}, nil)

	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/theme ")})
	out := m.View()
	for _, want := range []string{"violet", "ocean", "forest", "sunset", "mono"} {
		if !strings.Contains(out, want) {
			t.Fatalf("theme palette missing %s:\n%s", want, out)
		}
	}
}

func TestThemeCommandAppliesViaHandler(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 110, Height: 30})

	m.SetSlashCommands([]SlashCommand{
		{Name: "theme", Args: "<name> [light|dark]", Description: "切换颜色主题", Values: themeNames()},
	}, func(input string) CommandResult {
		fields := strings.Fields(input)
		mode := ""
		if len(fields) > 2 {
			mode = fields[2]
		}
		if err := m.ApplyTheme(fields[1], mode); err != nil {
			return CommandResult{Message: err.Error(), Error: true}
		}
		return CommandResult{Message: "已切换主题: " + m.ThemeName()}
	})

	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/theme forest dark")})
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.ThemeName() != "forest" || !m.DarkMode() {
		t.Fatalf("theme not applied: %s dark=%v", m.ThemeName(), m.DarkMode())
	}
	if !strings.Contains(m.View(), "已切换主题: forest") {
		t.Fatalf("confirmation missing:\n%s", m.View())
	}
}
