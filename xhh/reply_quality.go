package xhh

import (
	"openxhh/ai"
	"strings"

	"go.uber.org/zap"
)

const (
	maxAIReplyNaturalRunes                = 120
	maxAIReplyNaturalSentenceRunes        = 62
	maxAIReplyNaturalSentences            = 2
	maxSeriousAIReplyNaturalRunes         = 180
	maxSeriousAIReplyNaturalSentenceRunes = 76
	maxSeriousAIReplyNaturalSentences     = 4
	maxFeedReplyNaturalRunes              = 90
	maxFeedReplyNaturalSentenceRunes      = 56
	maxFeedReplyNaturalSentences          = 2
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
	if aiReplyQualityIssueForQuestion(reply, questionText) == "" {
		return reply, false
	}
	retryQuestion := questionText + "\n\n上一条回复太长、太绕或太像角色模板。请重写：像正常人在评论区回复，默认 1-2 句；如果用户是在认真求助或分析，先直接回答问题，再保留一点惠惠的嘴硬或炸毛，不要空转设定。"
	retry := strings.TrimSpace(getAIReplyForQualityRetry(contents, retryQuestion, topics, tags, logFields...))
	if retry == "" {
		return "", false
	}
	if shouldSkipFeedReply(retry) || len([]rune(retry)) > xhhCommentMaxRunes {
		return "", true
	}
	if aiReplyQualityIssueForQuestion(retry, questionText) != "" {
		return "", true
	}
	return retry, false
}

func generateFeedReplyWithQualityRetry(prompt string, contents []ai.Content, instruction, title string, topics []ai.Topics, tags []ai.Tags, logFields ...zap.Field) string {
	reply := sanitizeFeedReply(getAIFeedReplyForQualityRetry(prompt, contents, instruction, topics, tags, logFields...))
	if feedReplyQualityIssue(reply, title) == "" {
		return reply
	}
	retryInstruction := instruction + "\n\n上一条太长或太像说明文。请改成像正常人在评论区随手回的一句话，最多两句；保留惠惠的嘴硬、得意或炸毛，但不要展开讲道理。"
	return sanitizeFeedReply(getAIFeedReplyForQualityRetry(prompt, contents, retryInstruction, topics, tags, logFields...))
}
