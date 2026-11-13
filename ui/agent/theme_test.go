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
	if err := m.ApplyTheme("dracula", "dark"); err != nil {
		t.Fatal(err)
	}
	if !m.DarkMode() || m.ThemeName() != "dracula" {
		t.Fatalf("theme state wrong: %s dark=%v", m.ThemeName(), m.DarkMode())
	}
	if m.theme.Primary != "#BD93F9" || m.theme.Accent != "#8BE9FD" {
		t.Fatalf("dracula dark palette = %+v", m.theme)
	}

	// Light variant of the same theme has different colors.
	if err := m.ApplyTheme("dracula", "light"); err != nil {
		t.Fatal(err)
	}
	if m.DarkMode() {
		t.Fatal("expected light mode")
	}
	if m.theme.Primary != "#644AC9" || m.theme.Accent != "#036A96" {
		t.Fatalf("dracula light palette = %+v", m.theme)
	}

	// Unknown theme / mode fail with a helpful error.
	if err := m.ApplyTheme("nope", ""); err == nil {
		t.Fatal("unknown theme should fail")
	}
	if err := m.ApplyTheme("dracula", "blue"); err == nil {
		t.Fatal("invalid mode should fail")
	}
}

func TestThemeRegistryComplete(t *testing.T) {
	seen := make(map[string]bool, len(themeRegistry))
	for _, th := range themeRegistry {
		if th.Name == "" || th.Description == "" {
			t.Fatalf("theme %q missing name or description", th.Name)
		}
		if seen[th.Name] {
			t.Fatalf("duplicate theme name %q", th.Name)
		}
		seen[th.Name] = true
		for mode, v := range map[string]ThemeVariant{"light": th.Light, "dark": th.Dark} {
			colors := map[string]string{
				"primary": v.Primary, "accent": v.Accent, "success": v.Success,
				"warning": v.Warning, "error": v.Error, "user": v.User, "muted": v.Muted,
			}
			for field, color := range colors {
				if color == "" {
					t.Fatalf("theme %s/%s missing %s", th.Name, mode, field)
				}
				if !strings.HasPrefix(color, "#") || len(color) != 7 {
					t.Fatalf("theme %s/%s %s = %q, want #RRGGBB", th.Name, mode, field, color)
				}
			}
		}
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
	for _, want := range []string{"violet", "catppuccin", "nord", "gruvbox", "tokyo-night", "dracula", "mono"} {
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

	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/theme tokyo-night dark")})
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.ThemeName() != "tokyo-night" || !m.DarkMode() {
		t.Fatalf("theme not applied: %s dark=%v", m.ThemeName(), m.DarkMode())
	}
	if !strings.Contains(m.View(), "已切换主题: tokyo-night") {
		t.Fatalf("confirmation missing:\n%s", m.View())
	}
}
