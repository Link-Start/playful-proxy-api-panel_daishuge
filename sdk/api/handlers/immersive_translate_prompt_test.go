package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	coreexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
	"github.com/tidwall/gjson"
)

const immersiveTranslateTestSystem = `你是一个专业的简体中文母语译者，需将文本流畅地翻译为简体中文。
## 翻译规则
1. 仅输出译文内容，禁止解释或添加任何额外内容
2. 返回的译文必须和原文保持完全相同的段落数量和格式

Document Metadata:
Title: 《Example - YouTube》
Type: Subtitle
Summary: This video explains a puzzle.`

func TestImmersiveTranslateSubtitlePromptInjectsOpenAIChat(t *testing.T) {
	executor := &presetPromptCaptureExecutor{provider: "immersive-translate-subtitle"}
	handler, model := newPresetPromptTestHandler(t, executor, sdkconfig.PresetPromptConfig{})
	body := immersiveTranslateTestBody(model, immersiveTranslateTestSystem, "翻译为简体中文：\n\nfirst line\n\n%%\n\nsecond line")

	_, _, errMsg := handler.ExecuteWithAuthManager(context.Background(), "openai", model, body, "")
	if errMsg != nil {
		t.Fatalf("ExecuteWithAuthManager returned error: %+v", errMsg)
	}

	payloads := executor.ExecutePayloads()
	if len(payloads) != 1 {
		t.Fatalf("captured upstream payloads = %d, want 1", len(payloads))
	}
	forwarded := payloads[0]
	if got := gjson.GetBytes(forwarded, "messages.0.role").String(); got != "system" {
		t.Fatalf("messages.0.role = %q, want system; payload=%s", got, forwarded)
	}
	if got := gjson.GetBytes(forwarded, "messages.0.content").String(); got != immersiveTranslateSubtitlePrompt {
		t.Fatalf("subtitle prompt was not prepended; got %q", got)
	}
	if got := gjson.GetBytes(forwarded, "messages.1.content").String(); got != immersiveTranslateTestSystem {
		t.Fatalf("original system prompt moved incorrectly; got %q", got)
	}

	originals := executor.ExecuteOriginalRequests()
	if len(originals) != 1 || !bytes.Equal(originals[0], body) {
		t.Fatalf("OriginalRequest was mutated: got %q want %q", originals, body)
	}
}

func TestImmersiveTranslateSubtitlePromptDoesNotTriggerForInlinePercentOnly(t *testing.T) {
	executor := &presetPromptCaptureExecutor{provider: "immersive-translate-inline-percent"}
	handler, model := newPresetPromptTestHandler(t, executor, sdkconfig.PresetPromptConfig{})
	body := immersiveTranslateTestBody(model, immersiveTranslateTestSystem, "翻译为简体中文：\n\nopen http://a/%%30%30 now")

	_, _, errMsg := handler.ExecuteWithAuthManager(context.Background(), "openai", model, body, "")
	if errMsg != nil {
		t.Fatalf("ExecuteWithAuthManager returned error: %+v", errMsg)
	}
	payloads := executor.ExecutePayloads()
	if len(payloads) != 1 {
		t.Fatalf("captured upstream payloads = %d, want 1", len(payloads))
	}
	if got := gjson.GetBytes(payloads[0], "messages.0.content").String(); got == immersiveTranslateSubtitlePrompt {
		t.Fatalf("inline %% in URL triggered subtitle prompt unexpectedly; payload=%s", payloads[0])
	}
	if !bytes.Equal(payloads[0], body) {
		t.Fatalf("payload changed without a clean delimiter line: got %s want %s", payloads[0], body)
	}
}

func TestImmersiveTranslateSubtitlePromptDoesNotTriggerForNonYouTubeTranslation(t *testing.T) {
	executor := &presetPromptCaptureExecutor{provider: "immersive-translate-generic"}
	handler, model := newPresetPromptTestHandler(t, executor, sdkconfig.PresetPromptConfig{})
	system := strings.ReplaceAll(immersiveTranslateTestSystem, "Title: 《Example - YouTube》", "Title: 《Example Blog》")
	body := immersiveTranslateTestBody(model, system, "翻译为简体中文：\n\nfirst line\n\n%%\n\nsecond line")

	_, _, errMsg := handler.ExecuteWithAuthManager(context.Background(), "openai", model, body, "")
	if errMsg != nil {
		t.Fatalf("ExecuteWithAuthManager returned error: %+v", errMsg)
	}
	payloads := executor.ExecutePayloads()
	if len(payloads) != 1 {
		t.Fatalf("captured upstream payloads = %d, want 1", len(payloads))
	}
	if got := gjson.GetBytes(payloads[0], "messages.0.content").String(); got == immersiveTranslateSubtitlePrompt {
		t.Fatalf("non-YouTube translation triggered subtitle prompt unexpectedly; payload=%s", payloads[0])
	}
}

func TestImmersiveTranslateSubtitlePromptStacksWithPresetPromptAndRedactsLeaks(t *testing.T) {
	leaked := "prefix " + presetPromptTestMarker + " middle " + immersiveTranslateSubtitlePrompt + " suffix"
	executor := &presetPromptCaptureExecutor{
		provider:        "immersive-translate-redact",
		responsePayload: []byte(fmt.Sprintf(`{"choices":[{"message":{"content":%q}}]}`, leaked)),
	}
	handler, model := newPresetPromptTestHandler(t, executor, sdkconfig.PresetPromptConfig{
		Enabled: true,
		Prompt:  presetPromptTestMarker,
	})
	body := immersiveTranslateTestBody(model, immersiveTranslateTestSystem, "翻译为简体中文：\n\nfirst line\n\n%%\n\nsecond line")

	payload, _, errMsg := handler.ExecuteWithAuthManager(context.Background(), "openai", model, body, "")
	if errMsg != nil {
		t.Fatalf("ExecuteWithAuthManager returned error: %+v", errMsg)
	}
	if bytes.Contains(payload, []byte(presetPromptTestMarker)) || bytes.Contains(payload, []byte(immersiveTranslateSubtitlePrompt)) {
		t.Fatalf("response leaked injected prompt: %s", payload)
	}
	if got := bytes.Count(payload, []byte(presetPromptRedactionText)); got != 2 {
		t.Fatalf("redaction count = %d, want 2; payload=%s", got, payload)
	}

	payloads := executor.ExecutePayloads()
	if got := gjson.GetBytes(payloads[0], "messages.0.content").String(); got != immersiveTranslateSubtitlePrompt {
		t.Fatalf("messages.0.content = %q, want subtitle prompt", got)
	}
	if got := gjson.GetBytes(payloads[0], "messages.1.content").String(); got != presetPromptTestMarker {
		t.Fatalf("messages.1.content = %q, want preset prompt", got)
	}
}

func TestImmersiveTranslateSubtitlePromptStreamingRedactsLeakAcrossChunks(t *testing.T) {
	leaked := "prefix " + immersiveTranslateSubtitlePrompt + " suffix"
	encodedLeak, err := json.Marshal(leaked)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	frame := []byte("data: {\"delta\":" + string(encodedLeak) + "}\n\n")
	mid := len(frame) / 2
	executor := &presetPromptCaptureExecutor{
		provider: "immersive-translate-stream-redact",
		streamChunks: []coreexecutor.StreamChunk{
			{Payload: bytes.Clone(frame[:mid])},
			{Payload: bytes.Clone(frame[mid:])},
		},
	}
	handler, model := newPresetPromptTestHandler(t, executor, sdkconfig.PresetPromptConfig{})
	body := immersiveTranslateTestBody(model, immersiveTranslateTestSystem, "翻译为简体中文：\n\nfirst line\n\n%%\n\nsecond line")

	data, _, errs := handler.ExecuteStreamWithAuthManager(context.Background(), "openai", model, body, "")
	var got bytes.Buffer
	for chunk := range data {
		got.Write(chunk)
	}
	for errMsg := range errs {
		if errMsg != nil {
			t.Fatalf("unexpected stream error: %+v", errMsg)
		}
	}
	if strings.Contains(got.String(), "当前请求来自沉浸式翻译") {
		t.Fatalf("stream response leaked subtitle prompt: %s", got.String())
	}
	if !strings.Contains(got.String(), presetPromptRedactionText) {
		t.Fatalf("stream response missing redaction marker: %s", got.String())
	}
}

func immersiveTranslateTestBody(model, system, user string) []byte {
	return []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"system","content":%q},{"role":"user","content":%q}]}`, model, system, user))
}
