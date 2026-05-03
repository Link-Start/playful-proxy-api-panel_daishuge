package cliproxy

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestApplyOAuthModelAlias_Rename(t *testing.T) {
	cfg := &config.Config{
		OAuthModelAlias: map[string][]config.OAuthModelAlias{
			"codex": {
				{Name: "gpt-5", Alias: "g5"},
			},
		},
	}
	models := []*ModelInfo{
		{ID: "gpt-5", Name: "models/gpt-5"},
	}

	out := applyOAuthModelAlias(cfg, "codex", "oauth", models)
	if len(out) != 1 {
		t.Fatalf("expected 1 model, got %d", len(out))
	}
	if out[0].ID != "g5" {
		t.Fatalf("expected model id %q, got %q", "g5", out[0].ID)
	}
	if out[0].Name != "models/g5" {
		t.Fatalf("expected model name %q, got %q", "models/g5", out[0].Name)
	}
}

func TestApplyAutomaticThinkingAliases(t *testing.T) {
	models := []*ModelInfo{
		{
			ID:       "gpt-5.3-codex-spark",
			Name:     "models/gpt-5.3-codex-spark",
			Thinking: &registry.ThinkingSupport{Levels: []string{"low", "medium", "high", "xhigh"}},
		},
	}

	out := applyAutomaticThinkingAliases(models, nil)
	ids := make(map[string]*ModelInfo, len(out))
	for _, model := range out {
		ids[model.ID] = model
	}
	for _, id := range []string{
		"gpt-5.3-codex-spark",
		"gpt-5.3-codex-spark-low",
		"gpt-5.3-codex-spark-medium",
		"gpt-5.3-codex-spark-high",
		"gpt-5.3-codex-spark-xhigh",
	} {
		if ids[id] == nil {
			t.Fatalf("missing model alias %q in %#v", id, ids)
		}
	}
	if got := ids["gpt-5.3-codex-spark-high"].ThinkingAliasBase; got != "gpt-5.3-codex-spark" {
		t.Fatalf("ThinkingAliasBase = %q", got)
	}
}

func TestApplyAutomaticThinkingAliases_ExplicitAliasWins(t *testing.T) {
	models := []*ModelInfo{
		{ID: "base", Thinking: &registry.ThinkingSupport{Levels: []string{"high"}}},
		{ID: "base-high"},
	}

	out := applyAutomaticThinkingAliases(models, nil)
	count := 0
	for _, model := range out {
		if model.ID == "base-high" {
			count++
			if model.ThinkingAliasBase != "" {
				t.Fatalf("explicit alias should keep priority, got generated marker %q", model.ThinkingAliasBase)
			}
		}
	}
	if count != 1 {
		t.Fatalf("base-high count = %d, want 1", count)
	}
}

func TestApplyOAuthModelAlias_ForkAddsAlias(t *testing.T) {
	cfg := &config.Config{
		OAuthModelAlias: map[string][]config.OAuthModelAlias{
			"codex": {
				{Name: "gpt-5", Alias: "g5", Fork: true},
			},
		},
	}
	models := []*ModelInfo{
		{ID: "gpt-5", Name: "models/gpt-5"},
	}

	out := applyOAuthModelAlias(cfg, "codex", "oauth", models)
	if len(out) != 2 {
		t.Fatalf("expected 2 models, got %d", len(out))
	}
	if out[0].ID != "gpt-5" {
		t.Fatalf("expected first model id %q, got %q", "gpt-5", out[0].ID)
	}
	if out[1].ID != "g5" {
		t.Fatalf("expected second model id %q, got %q", "g5", out[1].ID)
	}
	if out[1].Name != "models/g5" {
		t.Fatalf("expected forked model name %q, got %q", "models/g5", out[1].Name)
	}
}

func TestApplyOAuthModelAlias_ForkAddsFixedThinkingAlias(t *testing.T) {
	cfg := &config.Config{
		OAuthModelAlias: map[string][]config.OAuthModelAlias{
			"codex": {
				{Name: "gpt-5.3-codex-spark-high", Alias: "spark-fast", Fork: true},
			},
		},
	}
	models := []*ModelInfo{
		{
			ID:       "gpt-5.3-codex-spark",
			Name:     "models/gpt-5.3-codex-spark",
			Thinking: &registry.ThinkingSupport{Levels: []string{"low", "medium", "high", "xhigh"}},
		},
	}

	aliased := applyOAuthModelAlias(cfg, "codex", "oauth", models)
	ids := make(map[string]*ModelInfo, len(aliased))
	for _, model := range aliased {
		ids[model.ID] = model
	}
	if ids["spark-fast"] == nil {
		t.Fatalf("missing fixed thinking alias in %#v", ids)
	}
	if ids["spark-fast"].Thinking != nil {
		t.Fatalf("fixed thinking alias should not generate additional thinking aliases")
	}

	out := applyAutomaticThinkingAliases(aliased, nil)
	ids = make(map[string]*ModelInfo, len(out))
	for _, model := range out {
		ids[model.ID] = model
	}
	if ids["spark-fast"] == nil {
		t.Fatalf("missing fixed thinking alias after automatic aliases in %#v", ids)
	}
	if ids["spark-fast-low"] != nil {
		t.Fatalf("fixed thinking alias generated a misleading level alias")
	}
	if ids["gpt-5.3-codex-spark-high"] == nil {
		t.Fatalf("missing automatic base thinking alias in %#v", ids)
	}
}

func TestApplyOAuthModelAlias_FixedThinkingAliasRequiresSupportedBase(t *testing.T) {
	cfg := &config.Config{
		OAuthModelAlias: map[string][]config.OAuthModelAlias{
			"codex": {
				{Name: "plain-model-high", Alias: "plain-fast", Fork: true},
			},
		},
	}
	models := []*ModelInfo{
		{ID: "plain-model", Name: "models/plain-model"},
	}

	out := applyOAuthModelAlias(cfg, "codex", "oauth", models)
	for _, model := range out {
		if model.ID == "plain-fast" {
			t.Fatalf("non-thinking base model should not get fixed thinking alias")
		}
	}
}

func TestApplyOAuthModelAlias_ForkAddsMultipleAliases(t *testing.T) {
	cfg := &config.Config{
		OAuthModelAlias: map[string][]config.OAuthModelAlias{
			"codex": {
				{Name: "gpt-5", Alias: "g5", Fork: true},
				{Name: "gpt-5", Alias: "g5-2", Fork: true},
			},
		},
	}
	models := []*ModelInfo{
		{ID: "gpt-5", Name: "models/gpt-5"},
	}

	out := applyOAuthModelAlias(cfg, "codex", "oauth", models)
	if len(out) != 3 {
		t.Fatalf("expected 3 models, got %d", len(out))
	}
	if out[0].ID != "gpt-5" {
		t.Fatalf("expected first model id %q, got %q", "gpt-5", out[0].ID)
	}
	if out[1].ID != "g5" {
		t.Fatalf("expected second model id %q, got %q", "g5", out[1].ID)
	}
	if out[1].Name != "models/g5" {
		t.Fatalf("expected forked model name %q, got %q", "models/g5", out[1].Name)
	}
	if out[2].ID != "g5-2" {
		t.Fatalf("expected third model id %q, got %q", "g5-2", out[2].ID)
	}
	if out[2].Name != "models/g5-2" {
		t.Fatalf("expected forked model name %q, got %q", "models/g5-2", out[2].Name)
	}
}
