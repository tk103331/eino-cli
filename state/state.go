// Package state stores small pieces of CLI state that persist between runs.
package state

import (
	"os"
	"path/filepath"
	"strings"
)

// dir returns the eino-cli state directory (~/.eino-cli).
func dir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".eino-cli")
}

// lastAgentPath returns the file storing the last used agent name.
func lastAgentPath() string {
	d := dir()
	if d == "" {
		return ""
	}
	return filepath.Join(d, "last_agent")
}

// LastAgent returns the agent name used in the previous session, or "".
func LastAgent() string {
	path := lastAgentPath()
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SetLastAgent records the agent used in this session.
func SetLastAgent(name string) error {
	path := lastAgentPath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(name)+"\n"), 0644)
}

// ClearLastAgent removes the stored agent name.
func ClearLastAgent() error {
	path := lastAgentPath()
	if path == "" {
		return nil
	}
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
