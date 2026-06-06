package xhh

import (
	"context"
	"openxhh/ai"
	"openxhh/config"
	"openxhh/db"
	"openxhh/loger"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	replyImageContextMaxChars = 1200
	replyImagePersonaMaxChars = 1200
	replyImageTextMaxChars    = 500
)

func replyWithOptionalImage(v db.CommStruct, replyText, questionText string, contents []ai.Content) bool {
	linkID := strconv.Itoa(v.LinkID)
	replyID := strconv.Itoa(v.CommentID)
	rootID := strconv.Itoa(v.RootID)
	if !config.ConfigStruct.Image.ReplyWithImage {
		return Reply(replyText, linkID, replyID, rootID, "")
	}
	if !ai.HasImageConfig() {
		loger.Loger.Warn("[XHH]普通回复配图已启用，但图片模型配置不完整，回退纯文字", zap.Int("comment_id", v.CommentID), zap.Int("link_id", v.LinkID))
		return Reply(replyText, linkID, replyID, rootID, "")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	prompt := BuildReplyImagePrompt(contents, questionText, replyText)
	imageResult, err := ai.GenerateImage(ctx, prompt)
	if err != nil {
		loger.Loger.Warn("[XHH]普通回复配图生成失败，回退纯文字", zap.Error(err), zap.Int("comment_id", v.CommentID), zap.Int("link_id", v.LinkID))
		return Reply(replyText, linkID, replyID, rootID, "")
	}
	imageURL, uploadPlan, err := resolveXHHImageURL(ctx, imageResult, false)
	if err != nil {
		loger.Loger.Warn("[XHH]普通回复配图上传失败，回退纯文字", zap.Error(err), zap.Int("comment_id", v.CommentID), zap.Int("link_id", v.LinkID))
		return Reply(replyText, linkID, replyID, rootID, "")
	}
	loger.Loger.Info("[XHH]普通回复图片 URL 准备完成", zap.Int("comment_id", v.CommentID), zap.Int("link_id", v.LinkID), zap.String("image_url", imageURL), zap.String("upload_key", uploadPlan.Key), zap.Bool("uploaded", uploadPlan.Uploaded))
	if ReplyImage(replyText, linkID, replyID, rootID, imageURL) {
		return true
	}
	loger.Loger.Warn("[XHH]普通回复带图发送失败，回退纯文字", zap.Int("comment_id", v.CommentID), zap.Int("link_id", v.LinkID))
	return Reply(replyText, linkID, replyID, rootID, "")
}

func BuildReplyImagePrompt(contents []ai.Content, questionText, replyText string) string {
	var builder strings.Builder
	builder.WriteString("Create one image to accompany a Xiaoheihe comment reply.\n")
	builder.WriteString("The image should express the configured character persona and the mood of the final reply.\n")
	builder.WriteString("Do not render the reply as text. Do not add captions, speech bubbles, UI screenshots, watermarks, or logos.\n")
	builder.WriteString("Prefer a vivid character-themed illustration or scene that fits the conversation context.\n")
	if persona := replyImagePersonaPrompt(); persona != "" {
		builder.WriteString("\nCharacter persona:\n")
		builder.WriteString(limitReplyImageText(persona, replyImagePersonaMaxChars))
		builder.WriteByte('\n')
	}
	if contextText := replyImageContextText(contents); contextText != "" {
		builder.WriteString("\nConversation context:\n")
		builder.WriteString(contextText)
		builder.WriteByte('\n')
	}
	if questionText = strings.TrimSpace(questionText); questionText != "" {
		builder.WriteString("\nUser said:\n")
		builder.WriteString(limitReplyImageText(questionText, replyImageTextMaxChars))
		builder.WriteByte('\n')
	}
	if replyText = strings.TrimSpace(replyText); replyText != "" {
		builder.WriteString("\nFinal text reply:\n")
		builder.WriteString(limitReplyImageText(replyText, replyImageTextMaxChars))
		builder.WriteByte('\n')
	}
	builder.WriteString("\nImage prompt: one polished square illustration, no readable text.")
	return builder.String()
}

func replyImagePersonaPrompt() string {
	cfg := config.ConfigStruct.Ai
	parts := []string{
		cfg.ChatName,
		cfg.Description,
		cfg.Personality,
		cfg.Scenario,
		cfg.FirstMessage,
		cfg.ExampleDialogs,
		cfg.PostHistoryInstructions,
	}
	var filtered []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, "\n")
}

func replyImageContextText(contents []ai.Content) string {
	var parts []string
	for _, content := range contents {
		if content.Type != "text" {
			continue
		}
		text := strings.TrimSpace(CleanXHHRichText(content.Text))
		if text != "" {
			parts = append(parts, text)
		}
	}
	return limitReplyImageText(strings.Join(parts, "\n"), replyImageContextMaxChars)
}

func limitReplyImageText(text string, maxChars int) string {
	text = strings.TrimSpace(text)
	if maxChars <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text
	}
	return strings.TrimSpace(string(runes[:maxChars]))
}
