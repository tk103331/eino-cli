package agent

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func isQuitCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestEscNeverQuits(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(*ViewModel)
	if isQuitCmd(cmd) {
		t.Fatal("Esc must not quit")
	}
	if m.confirmQuit {
		t.Fatal("Esc should not arm quit confirmation")
	}
}

func TestCtrlCRequiresConfirmation(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// First press arms the confirmation and shows a hint.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(*ViewModel)
	if isQuitCmd(cmd) {
		t.Fatal("first Ctrl+C must not quit")
	}
	if !m.confirmQuit {
		t.Fatal("first Ctrl+C should arm confirmation")
	}
	if cmd == nil {
		t.Fatal("confirmation should schedule a timeout tick")
	}
	if !strings.Contains(m.View(), "再按一次 Ctrl+C") {
		t.Fatalf("confirmation hint missing:\n%s", m.View())
	}

	// Second press quits.
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(*ViewModel)
	if !isQuitCmd(cmd) {
		t.Fatal("second Ctrl+C should quit")
	}
}

func TestCtrlCCancelledByOtherKeyOrTimeout(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(*ViewModel)
	if !m.confirmQuit {
		t.Fatal("confirmation not armed")
	}

	// Any other key cancels the pending quit and is processed normally.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = next.(*ViewModel)
	if m.confirmQuit {
		t.Fatal("typing should cancel the pending quit")
	}
	if m.input != "a" {
		t.Fatalf("key should still be processed, input=%q", m.input)
	}

	// Timeout also cancels.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(*ViewModel)
	next, _ = m.Update(confirmQuitTimeoutMsg{})
	m = next.(*ViewModel)
	if m.confirmQuit {
		t.Fatal("timeout should cancel the pending quit")
	}

	// And after a timeout a single Ctrl+C no longer quits.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(*ViewModel)
	if isQuitCmd(cmd) || !m.confirmQuit {
		t.Fatal("Ctrl+C after timeout should re-arm confirmation")
	}
}

func TestEscDismissesPalette(t *testing.T) {
	m := NewViewModel(func(string) error { return nil })
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.SetSlashCommands([]SlashCommand{{Name: "model", Args: "<name>", Values: []string{"a"}}}, nil)

	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/model ")})
	if !m.paletteActive() {
		t.Fatal("palette should be active")
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(*ViewModel)
	if isQuitCmd(cmd) {
		t.Fatal("Esc must not quit")
	}
	if m.input != "" || m.paletteActive() {
		t.Fatalf("Esc should dismiss the palette, input=%q", m.input)
	}
}
