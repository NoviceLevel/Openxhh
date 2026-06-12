package xhh

import (
	"openxhh/ai"
	"strings"

	"go.uber.org/zap"
)

const (
	maxFeedReplyNaturalRunes         = 90
	maxFeedReplyNaturalSentenceRunes = 56
	maxFeedReplyNaturalSentences     = 2
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
	reply := sanitizeFeedReply(getAIFeedReplyForQualityRetry(prompt, contents, instruction, topics, tags, logFields...))
	if feedReplyQualityIssue(reply, title) == "" {
		return reply
	}
	retryInstruction := instruction + "\n\n上一条太长或太像说明文。请改成像正常人在评论区随手回的一句话，最多两句；保留惠惠的嘴硬、得意或炸毛，但不要展开讲道理。"
	return sanitizeFeedReply(getAIFeedReplyForQualityRetry(prompt, contents, retryInstruction, topics, tags, logFields...))
}
