package handlers

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

const immersiveTranslateSegmentPromptTemplate = `当前请求来自沉浸式翻译插件。你正在返回机器会继续拆分的批量译文，不是普通整段翻译。用户体验优先级高于中文自然连贯性：必须先保证第 1 段原文对应第 1 段译文，第 2 段原文对应第 2 段译文，依次类推。

本次输入已经被独立分隔符行切成 %[2]d 个片段，分隔符行数量是 %[1]d。你的输出必须满足这个精确形状：
译文片段数量 = %[2]d
分隔符行数量 = %[1]d

分隔符协议：
1. 每个分隔符行必须只包含两个 ASCII 百分号：%%%%。
2. 分隔符行前后只使用普通换行 U+000A。不要输出回车 U+000D，不要输出 CRLF，不要在 %%%% 前后加空格或 Tab。
3. 不要使用全角百分号，不要把 %%%% 包进引号、代码块、Markdown、项目符号或编号。

分段协议：
1. 一个输入片段只能产生一个译文片段。禁止合并相邻片段，禁止把一个片段拆成多个译文片段。
2. 可以参考上下文理解含义，但不得把相邻片段的意思搬进当前片段，不得重排顺序。
3. 片段即使是半句话、短语、标题、帖子碎片、歌词碎片、日文混合文本、已有中文或无需翻译内容，也必须保留它自己的输出位置。
4. 如果片段无需翻译，保留原文或只做必要本地化，但仍按原位置输出。
5. 不输出解释、标签、编号、项目符号、Markdown 加粗、额外标题或额外空段。

输出前只检查三件事：分隔符行必须等于 %[1]d；译文片段必须等于 %[2]d；每个分隔符行必须精确等于 %%%%。`

func buildImmersiveTranslateSegmentPrompt(delimiterCount int) string {
	if delimiterCount < 1 {
		delimiterCount = 1
	}
	return fmt.Sprintf(immersiveTranslateSegmentPromptTemplate, delimiterCount, delimiterCount+1)
}

var immersiveTranslateSubtitlePrompt = buildImmersiveTranslateSegmentPrompt(1)

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
	delimiterCount := cleanPercentDelimiterLineCount(user)
	if !looksLikeImmersiveTranslateSegmentSystem(system) {
		return "", false
	}
	if !looksLikeImmersiveTranslateSegmentUser(user, delimiterCount) {
		return "", false
	}
	return buildImmersiveTranslateSegmentPrompt(delimiterCount), true
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

func looksLikeImmersiveTranslateSegmentUser(text string, delimiterCount int) bool {
	if strings.TrimSpace(text) == "" || delimiterCount == 0 {
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
