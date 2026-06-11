package xhh

import (
	"openxhh/ai"
	"strings"

	"go.uber.org/zap"
)

var getAIReplyForQualityRetry = ai.GetAiReply
var getAIFeedReplyForQualityRetry = ai.GetAiFeedReplyWithPrompt

func generateAIReplyWithQualityRetry(contents []ai.Content, questionText string, topics []ai.Topics, tags []ai.Tags, logFields ...zap.Field) (string, bool) {
	reply := strings.TrimSpace(getAIReplyForQualityRetry(contents, questionText, topics, tags, logFields...))
	if reply == "" {
		return "", false
	}
	if shouldSkipFeedReply(reply) || len([]rune(reply)) > xhhCommentMaxRunes {
		return "", true
	}
	return reply, false
}

func generateFeedReplyWithQualityRetry(prompt string, contents []ai.Content, instruction, title string, topics []ai.Topics, tags []ai.Tags, logFields ...zap.Field) string {
	return sanitizeFeedReply(getAIFeedReplyForQualityRetry(prompt, contents, instruction, topics, tags, logFields...))
}
