package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRepositoryExampleConfigsParseAndPreservePPAPDefaults(t *testing.T) {
	repoRoot := repositoryRoot(t)

	tests := []struct {
		name               string
		file               string
		wantWebsocketAuth  bool
		wantPanelRepo      string
		wantUsageStatsPath string
	}{
		{
			name:              "default example",
			file:              "config.example.yaml",
			wantWebsocketAuth: true,
			wantPanelRepo:     DefaultPanelGitHubRepository,
		},
		{
			name:               "docker example",
			file:               "config.docker.example.yaml",
			wantWebsocketAuth:  true,
			wantPanelRepo:      DefaultPanelGitHubRepository,
			wantUsageStatsPath: "/CLIProxyAPI/data/usage-statistics.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := copyExampleConfigToTemp(t, filepath.Join(repoRoot, tt.file))
			cfg, err := LoadConfigOptional(configPath, false)
			if err != nil {
				t.Fatalf("LoadConfigOptional(%s) error: %v", tt.file, err)
			}
			if cfg.WebsocketAuth != tt.wantWebsocketAuth {
				t.Fatalf("ws-auth = %v, want %v", cfg.WebsocketAuth, tt.wantWebsocketAuth)
			}
			if cfg.RemoteManagement.PanelGitHubRepository != tt.wantPanelRepo {
				t.Fatalf("panel repo = %q, want %q", cfg.RemoteManagement.PanelGitHubRepository, tt.wantPanelRepo)
			}
			if tt.wantUsageStatsPath != "" && cfg.UsageStatisticsPath != tt.wantUsageStatsPath {
				t.Fatalf("usage statistics path = %q, want %q", cfg.UsageStatisticsPath, tt.wantUsageStatsPath)
			}
		})
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func copyExampleConfigToTemp(t *testing.T, source string) string {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read example config: %v", err)
	}
	target := filepath.Join(t.TempDir(), filepath.Base(source))
	if err := os.WriteFile(target, data, 0o600); err != nil {
		t.Fatalf("write temp example config: %v", err)
	}
	return target
}
