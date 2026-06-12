package handlers

import (
	"strings"

	"github.com/tidwall/gjson"
)

const immersiveTranslateSegmentPrompt = `当前请求来自沉浸式翻译的分段批量翻译。请把分段对齐视为最高优先级，优先保证用户看到的每一段原文一一对应到同位置译文，中文语法和上下文连贯性只能在单个片段内部优化。

硬性输出协议：
1. 用户正文中只有“单独一行且去掉空白后等于 %%”的内容是分隔符；URL 或普通文本里的内联 %% 不是分隔符，必须留在原片段内。
2. 按这些分隔符切分后，输入有 N 个片段，输出必须也有 N 个译文片段，顺序完全一致。
3. 输出的分隔符行数量必须和用户正文完全相同；每个分隔符行只能是两个百分号：%%。
4. 每个输入片段只能对应一个输出片段。禁止合并相邻片段，禁止把一个片段拆成多个片段，禁止把前后片段的含义搬进当前片段。
5. 如果一个片段只是半句话、短语、标题、帖子碎片、歌词碎片或语法不完整，也只翻译这个片段本身，不为了中文通顺补上相邻片段内容。
6. 翻译时可参考上下文理解含义，但不得改变片段边界、数量、顺序或分隔符。
7. 不输出解释、标签、编号、Markdown、引号或额外空段。

自检后再输出：分隔符数量必须一致，片段数量必须一致，每个位置只放对应原片段的译文。`

const immersiveTranslateSubtitlePrompt = immersiveTranslateSegmentPrompt

func immersiveTranslateSubtitlePromptForPayload(handlerType string, rawJSON []byte) (string, bool) {
	if !strings.EqualFold(strings.TrimSpace(handlerType), "openai") || len(rawJSON) == 0 || !gjson.ValidBytes(rawJSON) {
		return "", false
	}
	messages := gjson.GetBytes(rawJSON, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return "", false
	}

	var systemText strings.Builder
	var userText strings.Builder
	for _, message := range messages.Array() {
		role := strings.ToLower(strings.TrimSpace(message.Get("role").String()))
		content := openAIChatContentText(message.Get("content"))
		if strings.TrimSpace(content) == "" {
			continue
		}
		switch role {
		case "system":
			systemText.WriteString(content)
			systemText.WriteByte('\n')
		case "user":
			userText.WriteString(content)
			userText.WriteByte('\n')
		}
	}

	system := systemText.String()
	user := userText.String()
	if !looksLikeImmersiveTranslateSegmentSystem(system) {
		return "", false
	}
	if !looksLikeImmersiveTranslateSegmentUser(user) {
		return "", false
	}
	return immersiveTranslateSegmentPrompt, true
}

func looksLikeImmersiveTranslateSegmentSystem(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	if !strings.Contains(text, "你是一个专业的简体中文母语译者") {
		return false
	}
	if !strings.Contains(text, "返回的译文必须和原文保持完全相同的段落数量和格式") {
		return false
	}
	return strings.Contains(text, "Document Metadata:") || strings.Contains(text, "输入输出格式示例")
}

func looksLikeImmersiveTranslateSegmentUser(text string) bool {
	if strings.TrimSpace(text) == "" || cleanPercentDelimiterLineCount(text) == 0 {
		return false
	}
	return strings.Contains(text, "翻译为")
}

func cleanPercentDelimiterLineCount(text string) int {
	if text == "" {
		return 0
	}
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "%%" {
			count++
		}
	}
	return count
}

func openAIChatContentText(content gjson.Result) string {
	switch {
	case content.Type == gjson.String:
		return content.String()
	case content.IsArray():
		var builder strings.Builder
		for _, part := range content.Array() {
			if text := part.Get("text"); text.Exists() && text.Type == gjson.String {
				builder.WriteString(text.String())
				builder.WriteByte('\n')
			}
		}
		return builder.String()
	default:
		return ""
	}
}
