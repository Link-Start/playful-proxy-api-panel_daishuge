package handlers

import (
	"encoding/json"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const presetPromptSeparator = "\n\n"

// SetPresetPromptConfig updates the request-time preset prompt snapshot.
func (h *BaseAPIHandler) SetPresetPromptConfig(cfg config.PresetPromptConfig) {
	if h == nil {
		return
	}
	cfg.Normalize()
	h.presetPromptMu.Lock()
	h.presetPromptConfig = cfg
	h.presetPromptMu.Unlock()
}

func (h *BaseAPIHandler) activePresetPrompt() (string, bool) {
	if h == nil {
		return "", false
	}
	h.presetPromptMu.RLock()
	cfg := h.presetPromptConfig
	h.presetPromptMu.RUnlock()
	if !cfg.Enabled || strings.TrimSpace(cfg.Prompt) == "" {
		return "", false
	}
	maxBytes := cfg.MaxBytes
	if maxBytes <= 0 {
		maxBytes = config.DefaultPresetPromptMaxBytes
	}
	if maxBytes > config.PresetPromptHardMaxBytes {
		maxBytes = config.PresetPromptHardMaxBytes
	}
	if len([]byte(cfg.Prompt)) > maxBytes {
		return "", false
	}
	return cfg.Prompt, true
}

func (h *BaseAPIHandler) applyPresetPromptToPayload(handlerType string, rawJSON []byte) []byte {
	prompt, ok := h.activePresetPrompt()
	if !ok || len(rawJSON) == 0 || !json.Valid(rawJSON) {
		return rawJSON
	}

	switch strings.ToLower(strings.TrimSpace(handlerType)) {
	case "openai":
		return injectPresetPromptIntoOpenAIChat(rawJSON, prompt)
	case "openai-response":
		return injectPresetPromptIntoOpenAIResponses(rawJSON, prompt)
	case "claude":
		return injectPresetPromptIntoClaude(rawJSON, prompt)
	case "gemini", "gemini-cli":
		return injectPresetPromptIntoGemini(rawJSON, prompt)
	default:
		return rawJSON
	}
}

func injectPresetPromptIntoOpenAIChat(rawJSON []byte, prompt string) []byte {
	messages := gjson.GetBytes(rawJSON, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return rawJSON
	}
	item, err := json.Marshal(map[string]any{
		"role":    "system",
		"content": prompt,
	})
	if err != nil {
		return rawJSON
	}
	mutatedMessages, ok := prependJSONRawArray(messages.Raw, item)
	if !ok {
		return rawJSON
	}
	out, err := sjson.SetRawBytes(rawJSON, "messages", mutatedMessages)
	if err != nil {
		return rawJSON
	}
	return out
}

func injectPresetPromptIntoOpenAIResponses(rawJSON []byte, prompt string) []byte {
	if openAIResponsesHasImageGenerationTool(rawJSON) {
		return rawJSON
	}
	instructions := gjson.GetBytes(rawJSON, "instructions")
	input := gjson.GetBytes(rawJSON, "input")
	if !instructions.Exists() && !input.Exists() {
		return rawJSON
	}
	if instructions.Exists() {
		if instructions.Type != gjson.String {
			return rawJSON
		}
		out, err := sjson.SetBytes(rawJSON, "instructions", prompt+presetPromptSeparator+instructions.String())
		if err != nil {
			return rawJSON
		}
		return out
	}
	out, err := sjson.SetBytes(rawJSON, "instructions", prompt)
	if err != nil {
		return rawJSON
	}
	return out
}

func openAIResponsesHasImageGenerationTool(rawJSON []byte) bool {
	tools := gjson.GetBytes(rawJSON, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return false
	}
	for _, tool := range tools.Array() {
		if strings.EqualFold(strings.TrimSpace(tool.Get("type").String()), "image_generation") {
			return true
		}
	}
	return false
}

func injectPresetPromptIntoClaude(rawJSON []byte, prompt string) []byte {
	messages := gjson.GetBytes(rawJSON, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return rawJSON
	}

	system := gjson.GetBytes(rawJSON, "system")
	if !system.Exists() {
		out, err := sjson.SetBytes(rawJSON, "system", prompt)
		if err != nil {
			return rawJSON
		}
		return out
	}
	switch {
	case system.Type == gjson.String:
		out, err := sjson.SetBytes(rawJSON, "system", prompt+presetPromptSeparator+system.String())
		if err != nil {
			return rawJSON
		}
		return out
	case system.IsArray():
		item, err := json.Marshal(map[string]any{
			"type": "text",
			"text": prompt,
		})
		if err != nil {
			return rawJSON
		}
		mutatedSystem, ok := prependJSONRawArray(system.Raw, item)
		if !ok {
			return rawJSON
		}
		out, err := sjson.SetRawBytes(rawJSON, "system", mutatedSystem)
		if err != nil {
			return rawJSON
		}
		return out
	default:
		return rawJSON
	}
}

func injectPresetPromptIntoGemini(rawJSON []byte, prompt string) []byte {
	contents := gjson.GetBytes(rawJSON, "contents")
	if !contents.Exists() || !contents.IsArray() {
		return rawJSON
	}

	part, err := json.Marshal(map[string]any{"text": prompt})
	if err != nil {
		return rawJSON
	}
	systemInstruction := gjson.GetBytes(rawJSON, "systemInstruction")
	if !systemInstruction.Exists() {
		item, err := json.Marshal(map[string]any{
			"parts": []map[string]any{{"text": prompt}},
		})
		if err != nil {
			return rawJSON
		}
		out, err := sjson.SetRawBytes(rawJSON, "systemInstruction", item)
		if err != nil {
			return rawJSON
		}
		return out
	}
	if !systemInstruction.IsObject() {
		return rawJSON
	}

	parts := gjson.GetBytes(rawJSON, "systemInstruction.parts")
	if !parts.Exists() {
		out, err := sjson.SetRawBytes(rawJSON, "systemInstruction.parts", []byte("["+string(part)+"]"))
		if err != nil {
			return rawJSON
		}
		return out
	}
	if !parts.IsArray() {
		return rawJSON
	}
	mutatedParts, ok := prependJSONRawArray(parts.Raw, part)
	if !ok {
		return rawJSON
	}
	out, err := sjson.SetRawBytes(rawJSON, "systemInstruction.parts", mutatedParts)
	if err != nil {
		return rawJSON
	}
	return out
}

func prependJSONRawArray(arrayRaw string, item []byte) ([]byte, bool) {
	trimmed := strings.TrimSpace(arrayRaw)
	if trimmed == "" || !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return nil, false
	}
	if strings.TrimSpace(trimmed[1:len(trimmed)-1]) == "" {
		out := make([]byte, 0, len(item)+2)
		out = append(out, '[')
		out = append(out, item...)
		out = append(out, ']')
		return out, true
	}
	out := make([]byte, 0, len(trimmed)+len(item)+1)
	out = append(out, '[')
	out = append(out, item...)
	out = append(out, ',')
	out = append(out, trimmed[1:]...)
	return out, true
}
