package agent

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ThemeVariant is a set of interface colors tuned for one background mode.
type ThemeVariant struct {
	Primary string // brand / title / input border
	Accent  string // assistant label / selection cursor / palette
	Success string // ready status / successful tools
	Warning string // running tools / spinner
	Error   string // errors / failed tools
	User    string // user prompt marker
	Muted   string // secondary text, reasoning body
}

// Theme is a named color scheme with light and dark variants.
type Theme struct {
	Name        string
	Description string
	Light       ThemeVariant
	Dark        ThemeVariant
}

// defaultThemeName is used when no theme has been selected.
const defaultThemeName = "violet"

// themeRegistry lists built-in themes in palette order.
var themeRegistry = []Theme{
	{
		Name:        "violet",
		Description: "紫罗兰",
		Light: ThemeVariant{
			Primary: "#7C3AED", Accent: "#0891B2", Success: "#059669",
			Warning: "#D97706", Error: "#DC2626", User: "#7C3AED", Muted: "#6B7280",
		},
		Dark: ThemeVariant{
			Primary: "#A78BFA", Accent: "#22D3EE", Success: "#34D399",
			Warning: "#FBBF24", Error: "#F87171", User: "#A78BFA", Muted: "#9CA3AF",
		},
	},
	{
		Name:        "ocean",
		Description: "海洋",
		Light: ThemeVariant{
			Primary: "#2563EB", Accent: "#0D9488", Success: "#059669",
			Warning: "#D97706", Error: "#DC2626", User: "#2563EB", Muted: "#64748B",
		},
		Dark: ThemeVariant{
			Primary: "#60A5FA", Accent: "#2DD4BF", Success: "#34D399",
			Warning: "#FBBF24", Error: "#F87171", User: "#60A5FA", Muted: "#94A3B8",
		},
	},
	{
		Name:        "forest",
		Description: "森林",
		Light: ThemeVariant{
			Primary: "#15803D", Accent: "#0F766E", Success: "#16A34A",
			Warning: "#CA8A04", Error: "#DC2626", User: "#15803D", Muted: "#6B7280",
		},
		Dark: ThemeVariant{
			Primary: "#4ADE80", Accent: "#5EEAD4", Success: "#4ADE80",
			Warning: "#FDE047", Error: "#F87171", User: "#4ADE80", Muted: "#9CA3AF",
		},
	},
	{
		Name:        "sunset",
		Description: "日落",
		Light: ThemeVariant{
			Primary: "#C2410C", Accent: "#BE185D", Success: "#059669",
			Warning: "#B45309", Error: "#DC2626", User: "#C2410C", Muted: "#78716C",
		},
		Dark: ThemeVariant{
			Primary: "#FB923C", Accent: "#F472B6", Success: "#34D399",
			Warning: "#FBBF24", Error: "#F87171", User: "#FB923C", Muted: "#A8A29E",
		},
	},
	{
		Name:        "mono",
		Description: "黑白",
		Light: ThemeVariant{
			Primary: "#111827", Accent: "#374151", Success: "#047857",
			Warning: "#B45309", Error: "#B91C1C", User: "#111827", Muted: "#6B7280",
		},
		Dark: ThemeVariant{
			Primary: "#E5E7EB", Accent: "#9CA3AF", Success: "#6EE7B7",
			Warning: "#FCD34D", Error: "#FCA5A5", User: "#E5E7EB", Muted: "#9CA3AF",
		},
	},
}

// themeNames returns the built-in theme names in registry order.
func themeNames() []string {
	names := make([]string, 0, len(themeRegistry))
	for _, t := range themeRegistry {
		names = append(names, t.Name)
	}
	return names
}

// lookupTheme finds a theme by name (case-insensitive).
func lookupTheme(name string) (Theme, bool) {
	for _, t := range themeRegistry {
		if strings.EqualFold(t.Name, name) {
			return t, true
		}
	}
	return Theme{}, false
}

// resolveTheme picks the variant matching the terminal background.
func resolveTheme(name string, dark bool) ThemeVariant {
	t, ok := lookupTheme(name)
	if !ok {
		t, _ = lookupTheme(defaultThemeName)
	}
	if dark {
		return t.Dark
	}
	return t.Light
}

// ThemeName returns the active theme name.
func (m *ViewModel) ThemeName() string { return m.themeName }

// DarkMode reports whether the dark variant is active.
func (m *ViewModel) DarkMode() bool { return m.darkMode }

// ApplyTheme switches the color theme. mode may be "", "auto", "light" or
// "dark"; empty/auto follows the terminal background.
func (m *ViewModel) ApplyTheme(name, mode string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		name = m.themeName
	}
	t, ok := lookupTheme(name)
	if !ok {
		return fmt.Errorf("未知主题: %s（可用：%s）", name, strings.Join(themeNames(), ", "))
	}

	dark := m.darkMode
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "auto":
		dark = lipgloss.HasDarkBackground()
	case "dark":
		dark = true
	case "light":
		dark = false
	default:
		return fmt.Errorf("模式无效: %s（可选 light / dark / auto）", mode)
	}

	m.themeName = t.Name
	m.darkMode = dark
	if dark {
		m.theme = t.Dark
	} else {
		m.theme = t.Light
	}

	// Rebuild markdown styling for the new mode/colors.
	m.renderer = nil
	m.renderDark = !dark
	for i := range m.messages {
		m.messages[i].mdCache = ""
	}
	m.markDirty()
	return nil
}

// ---- theme-aware style helpers ----

// muted returns the muted text style for the active theme.
func (m *ViewModel) muted() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Muted))
}

// warningStyle returns the warning style for the active theme.
func (m *ViewModel) warningStyle(bold bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Warning))
	if bold {
		s = s.Bold(true)
	}
	return s
}

// accentStyle returns the accent style for the active theme.
func (m *ViewModel) accentStyle(bold bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Accent))
	if bold {
		s = s.Bold(true)
	}
	return s
}

// primaryStyle returns the primary style for the active theme.
func (m *ViewModel) primaryStyle(bold bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Primary))
	if bold {
		s = s.Bold(true)
	}
	return s
}

// errorStyle returns the error style for the active theme.
func (m *ViewModel) errorStyle(bold bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Error))
	if bold {
		s = s.Bold(true)
	}
	return s
}
