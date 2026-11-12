package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSkillFileFrontMatter(t *testing.T) {
	content := `---
description: Review code strictly
tools: [shell, grep]
---
You are a strict reviewer.
Be concise.`

	skill := parseSkillFile("reviewer", content)
	if skill.Description != "Review code strictly" {
		t.Fatalf("description = %q", skill.Description)
	}
	if len(skill.Tools) != 2 || skill.Tools[0] != "shell" {
		t.Fatalf("tools = %v", skill.Tools)
	}
	if skill.System != "You are a strict reviewer.\nBe concise." {
		t.Fatalf("system = %q", skill.System)
	}
}

func TestParseSkillFilePlain(t *testing.T) {
	content := "\n# Translator\n\nTranslate Chinese to English."

	skill := parseSkillFile("translator", content)
	if skill.Description != "Translator" {
		t.Fatalf("description = %q", skill.Description)
	}
	if skill.System != "# Translator\n\nTranslate Chinese to English." {
		t.Fatalf("system = %q", skill.System)
	}
}

func TestLoadConfigMergesSkillFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	einoDir := filepath.Join(home, ".eino-cli", "skills")
	agentsDir := filepath.Join(home, ".agents", "skills")
	for _, dir := range []string{einoDir, agentsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	write := func(path, content string) {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(einoDir, "alpha.md"), "Eino alpha prompt")
	write(filepath.Join(einoDir, "shared.md"), "Eino shared prompt")
	write(filepath.Join(agentsDir, "beta.md"), "Agents beta prompt")
	write(filepath.Join(agentsDir, "shared.md"), "Agents shared prompt")

	cfgPath := filepath.Join(home, "config.yml")
	write(cfgPath, `models:
  m1:
    provider: p
    model: x
skills:
  fromconfig:
    description: from config
    system: config prompt
`)

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	// Both user directories are loaded.
	if cfg.Skills["alpha"].System != "Eino alpha prompt" {
		t.Fatalf("eino-cli skill missing: %+v", cfg.Skills)
	}
	if cfg.Skills["beta"].System != "Agents beta prompt" {
		t.Fatalf("agents skill missing: %+v", cfg.Skills)
	}
	// ~/.eino-cli/skills has priority on name clashes.
	if cfg.Skills["shared"].System != "Eino shared prompt" {
		t.Fatalf("first directory should win: %+v", cfg.Skills)
	}
	// Config-defined skills are still merged.
	if cfg.Skills["fromconfig"].System != "config prompt" {
		t.Fatalf("config skill missing: %+v", cfg.Skills)
	}
}
