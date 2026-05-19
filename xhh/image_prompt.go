package xhh

import (
	"openxhh/ai"
	"openxhh/loger"
	"strings"

	"go.uber.org/zap"
)

const imageContextMaxChars = 1200

func BuildContextualImagePrompt(basePrompt string, command ImageCommand, contents []ai.Content) string {
	if !command.UsePostContext && !command.UseCommentContext {
		return basePrompt
	}

	contextText := selectImageContextText(contents, command)
	if contextText == "" {
		return basePrompt
	}

	prompt := "请根据以下" + imageContextLabel(command) + "生成图片。\n" +
		"参考内容：" + contextText + "\n" +
		"图片要求：" + basePrompt
	loger.Loger.Info("[XHH]已构造上下文生图 prompt", zap.Int("context_chars", len(contextText)), zap.Bool("post_context", command.UsePostContext), zap.Bool("comment_context", command.UseCommentContext))
	return prompt
}

func selectImageContextText(contents []ai.Content, command ImageCommand) string {
	var parts []string
	seenCommentContext := false
	for _, content := range contents {
		if content.Type != "text" || strings.TrimSpace(content.Text) == "" {
			continue
		}
		text := strings.TrimSpace(content.Text)
		if isCommentContextText(text) {
			seenCommentContext = true
		}
		if command.UseCommentContext && !command.UsePostContext && !seenCommentContext {
			continue
		}
		if !command.UseCommentContext && isCommentContextText(text) {
			continue
		}
		parts = append(parts, text)
	}
	return limitImageContext(strings.Join(parts, "\n"), imageContextMaxChars)
}

func imageContextLabel(command ImageCommand) string {
	if command.UsePostContext && command.UseCommentContext {
		return "帖子正文和评论区内容"
	}
	if command.UseCommentContext {
		return "评论区内容"
	}
	return "帖子正文内容"
}

func isCommentContextText(text string) bool {
	prefixes := []string{"以下是评论区", "以下是当前评论楼层上下文", "以下是当前评论楼层", "以下是评论楼层上下文", "以下是楼层上下文", "下面这张图片来自评论用户"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func limitImageContext(text string, maxChars int) string {
	text = strings.TrimSpace(text)
	if len([]rune(text)) <= maxChars {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:maxChars]))
}
