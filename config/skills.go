package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// skillFrontMatter is the optional YAML header of a skill markdown file.
type skillFrontMatter struct {
	Name        string   `yaml:"name,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Tools       []string `yaml:"tools,omitempty"`
}

// SkillDirectories returns the directories scanned for skill markdown files
// in priority order: on name clashes the first directory wins.
func SkillDirectories() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".eino-cli", "skills"),
		filepath.Join(home, ".agents", "skills"),
	}
}

// parseSkillFile parses one skill markdown document. The document may start
// with a YAML front matter block delimited by "---" lines:
//
//	---
//	description: Code reviewer
//	tools: [shell]
//	---
//	You are a strict code reviewer...
//
// Without front matter the whole body is the system prompt and the
// description falls back to the first non-empty line.
func parseSkillFile(defaultName, content string) Skill {
	content = strings.ReplaceAll(content, "\r\n", "\n")

	var meta skillFrontMatter
	body := content

	lines := strings.Split(content, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				front := strings.Join(lines[1:i], "\n")
				_ = yaml.Unmarshal([]byte(front), &meta)
				body = strings.Join(lines[i+1:], "\n")
				break
			}
		}
	}

	name := strings.TrimSpace(meta.Name)
	if name == "" {
		name = defaultName
	}

	system := strings.TrimSpace(body)
	desc := strings.TrimSpace(meta.Description)
	if desc == "" {
		desc = firstLine(system)
	}

	return Skill{
		Description: desc,
		System:      system,
		Tools:       meta.Tools,
	}
}

// firstLine returns the first non-empty line, truncated to a readable length.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "#>*- "))
		if line != "" {
			runes := []rune(line)
			if len(runes) > 80 {
				return string(runes[:79]) + "…"
			}
			return line
		}
	}
	return ""
}

// loadSkillsFromDirs merges skill files found in SkillDirectories into cfg.
// Files override config-defined skills with the same name; among directories
// the first entry of SkillDirectories wins. Directories are therefore loaded
// from lowest to highest priority.
func loadSkillsFromDirs(cfg *Config) {
	if cfg.Skills == nil {
		cfg.Skills = make(map[string]Skill)
	}
	dirs := SkillDirectories()
	for i := len(dirs) - 1; i >= 0; i-- {
		dir := dirs[i]
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			cfg.Skills[name] = parseSkillFile(name, string(data))
		}
	}
}
