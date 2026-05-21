package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigOptionalPresetPromptDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("port: 8317\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	assertDefaultPresetPromptConfig(t, cfg.PresetPrompt)
}

func TestLoadConfigOptionalPresetPromptEnabled(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`preset-prompt:
  enabled: true
  prompt: "  operator prompt  "
  max-bytes: 64
`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if !cfg.PresetPrompt.Enabled {
		t.Fatal("preset-prompt.enabled = false, want true")
	}
	if cfg.PresetPrompt.Prompt != "  operator prompt  " {
		t.Fatalf("preset-prompt.prompt = %q, want exact configured value", cfg.PresetPrompt.Prompt)
	}
	if cfg.PresetPrompt.MaxBytes != 64 {
		t.Fatalf("preset-prompt.max-bytes = %d, want 64", cfg.PresetPrompt.MaxBytes)
	}
}

func TestLoadConfigOptionalPresetPromptExactPromptFileSmoke(t *testing.T) {
	promptPath := strings.TrimSpace(os.Getenv("PPAP_T15_EXACT_PROMPT_FILE"))
	if promptPath == "" {
		t.Skip("set PPAP_T15_EXACT_PROMPT_FILE to run the exact production prompt config smoke")
	}
	rawPrompt, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read exact prompt file: %v", err)
	}
	prompt := strings.TrimRight(string(rawPrompt), "\r\n")
	if strings.TrimSpace(prompt) == "" {
		t.Fatal("exact prompt file is empty")
	}

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(exactPresetPromptConfigYAML(prompt)), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}
	if !cfg.PresetPrompt.Enabled {
		t.Fatal("preset-prompt.enabled = false, want true")
	}
	if cfg.PresetPrompt.Prompt != prompt {
		t.Fatal("preset-prompt.prompt did not round-trip from exact prompt config")
	}
	if cfg.PresetPrompt.MaxBytes != DefaultPresetPromptMaxBytes {
		t.Fatalf("preset-prompt.max-bytes = %d, want default %d", cfg.PresetPrompt.MaxBytes, DefaultPresetPromptMaxBytes)
	}
}

func TestLoadConfigOptionalPresetPromptEnabledRequiresPrompt(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`preset-prompt:
  enabled: true
  prompt: "   "
`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadConfigOptional(configPath, false)
	if err == nil {
		t.Fatal("LoadConfigOptional() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "preset-prompt.prompt must be set") {
		t.Fatalf("LoadConfigOptional() error = %v, want preset prompt validation", err)
	}
}

func TestLoadConfigOptionalPresetPromptSizeLimit(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`preset-prompt:
  enabled: true
  prompt: "too large"
  max-bytes: 4
`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadConfigOptional(configPath, false)
	if err == nil {
		t.Fatal("LoadConfigOptional() error = nil, want size validation error")
	}
	if !strings.Contains(err.Error(), "preset-prompt.prompt is too large") {
		t.Fatalf("LoadConfigOptional() error = %v, want size validation", err)
	}
}

func TestLoadConfigOptionalPresetPromptLimitNormalization(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`preset-prompt:
  enabled: false
  max-bytes: -1
`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}
	if cfg.PresetPrompt.MaxBytes != DefaultPresetPromptMaxBytes {
		t.Fatalf("preset-prompt.max-bytes = %d, want default %d", cfg.PresetPrompt.MaxBytes, DefaultPresetPromptMaxBytes)
	}

	configPath = filepath.Join(t.TempDir(), "config.yaml")
	data = []byte(`preset-prompt:
  enabled: false
  max-bytes: 999999
`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err = LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}
	if cfg.PresetPrompt.MaxBytes != PresetPromptHardMaxBytes {
		t.Fatalf("preset-prompt.max-bytes = %d, want max %d", cfg.PresetPrompt.MaxBytes, PresetPromptHardMaxBytes)
	}
}

func TestLoadConfigOptionalPresetPromptRejectsPromptAboveHardCap(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("preset-prompt:\n  enabled: true\n  prompt: " + strings.Repeat("a", PresetPromptHardMaxBytes+1) + "\n  max-bytes: 999999\n")
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadConfigOptional(configPath, false)
	if err == nil {
		t.Fatal("LoadConfigOptional() error = nil, want hard cap validation error")
	}
	if !strings.Contains(err.Error(), "preset-prompt.prompt is too large") {
		t.Fatalf("LoadConfigOptional() error = %v, want size validation", err)
	}
}

func exactPresetPromptConfigYAML(prompt string) string {
	var builder strings.Builder
	builder.WriteString("preset-prompt:\n")
	builder.WriteString("  enabled: true\n")
	builder.WriteString("  prompt: |-\n")
	normalized := strings.ReplaceAll(prompt, "\r\n", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		builder.WriteString("    ")
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func assertDefaultPresetPromptConfig(t *testing.T, cfg PresetPromptConfig) {
	t.Helper()
	if cfg.Enabled {
		t.Fatal("preset-prompt.enabled = true, want false")
	}
	if cfg.Prompt != "" {
		t.Fatalf("preset-prompt.prompt = %q, want empty", cfg.Prompt)
	}
	if cfg.MaxBytes != DefaultPresetPromptMaxBytes {
		t.Fatalf("preset-prompt.max-bytes = %d, want %d", cfg.MaxBytes, DefaultPresetPromptMaxBytes)
	}
}
