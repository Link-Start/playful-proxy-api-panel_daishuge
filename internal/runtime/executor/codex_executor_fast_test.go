package executor

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/thinking/provider/codex"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	"github.com/tidwall/gjson"
)

func TestCodexExecutorAppliesFastAliasServiceTierAndThinking(t *testing.T) {
	gotBody := executeCodexFastTestRequest(t, "gpt-5.5-high", []byte(`{"model":"gpt-5.5-high-fast","messages":[{"role":"user","content":"hello"}]}`))

	if got := gjson.GetBytes(gotBody, "model").String(); got != "gpt-5.5" {
		t.Fatalf("model = %q, want gpt-5.5. Body: %s", got, string(gotBody))
	}
	if got := gjson.GetBytes(gotBody, "reasoning.effort").String(); got != "high" {
		t.Fatalf("reasoning.effort = %q, want high. Body: %s", got, string(gotBody))
	}
	if got := gjson.GetBytes(gotBody, "service_tier").String(); got != "priority" {
		t.Fatalf("service_tier = %q, want priority. Body: %s", got, string(gotBody))
	}
}

func TestCodexExecutorAppliesSparkFastAliasServiceTierAndThinking(t *testing.T) {
	gotBody := executeCodexFastTestRequest(t, "gpt-5.3-codex-spark-high", []byte(`{"model":"gpt-5.3-codex-spark-high-fast","messages":[{"role":"user","content":"hello"}]}`))

	if got := gjson.GetBytes(gotBody, "model").String(); got != "gpt-5.3-codex-spark" {
		t.Fatalf("model = %q, want gpt-5.3-codex-spark. Body: %s", got, string(gotBody))
	}
	if got := gjson.GetBytes(gotBody, "reasoning.effort").String(); got != "high" {
		t.Fatalf("reasoning.effort = %q, want high. Body: %s", got, string(gotBody))
	}
	if got := gjson.GetBytes(gotBody, "service_tier").String(); got != "priority" {
		t.Fatalf("service_tier = %q, want priority. Body: %s", got, string(gotBody))
	}
}

func TestCodexExecutorAppliesExplicitFastServiceTier(t *testing.T) {
	gotBody := executeCodexFastTestRequest(t, "gpt-5.5-high", []byte(`{"model":"gpt-5.5-high","service_tier":"fast","messages":[{"role":"user","content":"hello"}]}`))

	if got := gjson.GetBytes(gotBody, "model").String(); got != "gpt-5.5" {
		t.Fatalf("model = %q, want gpt-5.5. Body: %s", got, string(gotBody))
	}
	if got := gjson.GetBytes(gotBody, "reasoning.effort").String(); got != "high" {
		t.Fatalf("reasoning.effort = %q, want high. Body: %s", got, string(gotBody))
	}
	if got := gjson.GetBytes(gotBody, "service_tier").String(); got != "priority" {
		t.Fatalf("service_tier = %q, want priority. Body: %s", got, string(gotBody))
	}
}

func executeCodexFastTestRequest(t *testing.T, model string, original []byte) []byte {
	t.Helper()

	var gotPath string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.5","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2},"output":[{"type":"message","content":[{"type":"output_text","text":"OK"}]}]}}` + "\n\n"))
	}))
	defer server.Close()

	executor := NewCodexExecutor(&config.Config{})
	auth := &cliproxyauth.Auth{Attributes: map[string]string{
		"base_url": server.URL,
		"api_key":  "test",
	}}

	_, err := executor.Execute(context.Background(), auth, cliproxyexecutor.Request{
		Model:   model,
		Payload: original,
	}, cliproxyexecutor.Options{
		SourceFormat:    sdktranslator.FromString("openai"),
		OriginalRequest: original,
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if gotPath != "/responses" {
		t.Fatalf("path = %q, want /responses", gotPath)
	}
	return gotBody
}
