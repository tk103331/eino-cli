// Package state stores small pieces of CLI state that persist between runs.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Dir returns the eino-cli state directory (~/.eino-cli).
func Dir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".eino-cli")
}

// lastAgentPath returns the file storing the last used agent name.
func lastAgentPath() string {
	d := Dir()
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

// rememberedModelsPath returns the file storing per-agent model choices made
// at runtime (e.g. with the /model command).
func rememberedModelsPath() string {
	d := Dir()
	if d == "" {
		return ""
	}
	return filepath.Join(d, "agent_models.json")
}

// RememberedModel returns the model remembered for an agent, or "".
func RememberedModel(agentName string) string {
	models := rememberedModels()
	return models[strings.TrimSpace(agentName)]
}

// SetRememberedModel records the model chosen for an agent. An empty model
// removes the entry, so the agent falls back to its configured model.
func SetRememberedModel(agentName, model string) error {
	agentName = strings.TrimSpace(agentName)
	if agentName == "" {
		return nil
	}
	model = strings.TrimSpace(model)

	models := rememberedModels()
	if model == "" {
		if _, ok := models[agentName]; !ok {
			return nil
		}
		delete(models, agentName)
	} else {
		if models[agentName] == model {
			return nil
		}
		models[agentName] = model
	}

	path := rememberedModelsPath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(models, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// rememberedModels loads the agent -> model map. A missing or unreadable file
// yields an empty map: remembering a model is best-effort.
func rememberedModels() map[string]string {
	out := make(map[string]string)
	path := rememberedModelsPath()
	if path == "" {
		return out
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return out
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return make(map[string]string)
	}
	return out
}
