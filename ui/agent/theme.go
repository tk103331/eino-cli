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

// themeRegistry lists built-in themes in palette order. Each entry is a
// complete, cohesive terminal palette with a matching light companion
// (Catppuccin Mocha/Latte, Nord, Gruvbox, Tokyo Night, Dracula/Alucard), so
// the accent colors stay harmonious instead of mixing unrelated hues.
var themeRegistry = []Theme{
	{
		Name:        "violet",
		Description: "紫罗兰（默认）",
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
		// Catppuccin Mocha / Latte: mauve + blue, pastel accents.
		Name:        "catppuccin",
		Description: "Catppuccin",
		Light: ThemeVariant{
			Primary: "#8839EF", Accent: "#1E66F5", Success: "#40A02B",
			Warning: "#DF8E1D", Error: "#D20F39", User: "#D156A8", Muted: "#6C6F85",
		},
		Dark: ThemeVariant{
			Primary: "#CBA6F7", Accent: "#89B4FA", Success: "#A6E3A1",
			Warning: "#F9E2AF", Error: "#F38BA8", User: "#F5C2E7", Muted: "#A6ADC8",
		},
	},
	{
		// Nord: frost blue against aurora accents.
		Name:        "nord",
		Description: "Nord",
		Light: ThemeVariant{
			Primary: "#5E81AC", Accent: "#8F6E96", Success: "#6E8B4C",
			Warning: "#C08A3E", Error: "#BF616A", User: "#8F6E96", Muted: "#5B6678",
		},
		Dark: ThemeVariant{
			Primary: "#88C0D0", Accent: "#81A1C1", Success: "#A3BE8C",
			Warning: "#EBCB8B", Error: "#BF616A", User: "#B48EAD", Muted: "#9AA5B8",
		},
	},
	{
		// Gruvbox dark / light: warm background, muted retro accents.
		Name:        "gruvbox",
		Description: "Gruvbox",
		Light: ThemeVariant{
			Primary: "#8F3F71", Accent: "#076678", Success: "#79740E",
			Warning: "#B57614", Error: "#9D0006", User: "#AF3A03", Muted: "#7C6F64",
		},
		Dark: ThemeVariant{
			Primary: "#D3869B", Accent: "#83A598", Success: "#B8BB26",
			Warning: "#FABD2F", Error: "#FB4934", User: "#FE8019", Muted: "#928374",
		},
	},
	{
		// Tokyo Night / Day.
		Name:        "tokyo-night",
		Description: "Tokyo Night",
		Light: ThemeVariant{
			Primary: "#9854F1", Accent: "#2E7DE9", Success: "#587539",
			Warning: "#B15C00", Error: "#F52A65", User: "#007197", Muted: "#5F6B94",
		},
		Dark: ThemeVariant{
			Primary: "#BB9AF7", Accent: "#7AA2F7", Success: "#9ECE6A",
			Warning: "#E0AF68", Error: "#F7768E", User: "#2AC3DE", Muted: "#737AA2",
		},
	},
	{
		// Dracula / Alucard.
		Name:        "dracula",
		Description: "Dracula",
		Light: ThemeVariant{
			Primary: "#644AC9", Accent: "#036A96", Success: "#14710A",
			Warning: "#846E15", Error: "#CB3A2A", User: "#A3144D", Muted: "#6C664B",
		},
		Dark: ThemeVariant{
			Primary: "#BD93F9", Accent: "#8BE9FD", Success: "#50FA7B",
			Warning: "#FFB86C", Error: "#FF5555", User: "#FF79C6", Muted: "#6272A4",
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
