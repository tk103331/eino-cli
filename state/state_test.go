package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLastAgentRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if got := LastAgent(); got != "" {
		t.Fatalf("expected empty last agent, got %q", got)
	}

	if err := SetLastAgent("minimax_agent"); err != nil {
		t.Fatal(err)
	}
	if got := LastAgent(); got != "minimax_agent" {
		t.Fatalf("last agent = %q", got)
	}

	// The state file lives under ~/.eino-cli.
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".eino-cli", "last_agent")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file missing: %v", err)
	}

	if err := ClearLastAgent(); err != nil {
		t.Fatal(err)
	}
	if got := LastAgent(); got != "" {
		t.Fatalf("last agent should be cleared, got %q", got)
	}
}
