package xhh

import (
	"openxhh/ai"
	"openxhh/loger"
	"strings"

	"go.uber.org/zap"
)

const maxReplyQualityAttempts = 8

var getAIReplyForQualityRetry = ai.GetAiReply
var getAIFeedReplyForQualityRetry = ai.GetAiFeedReplyWithPrompt

func generateAIReplyWithQualityRetry(contents []ai.Content, questionText string, topics []ai.Topics, tags []ai.Tags, logFields ...zap.Field) (string, bool) {
	lastIssue := ""
	for attempt := 1; attempt <= maxReplyQualityAttempts; attempt++ {
		currentQuestion := questionText
		fields := logFields
		if attempt > 1 {
			currentQuestion = aiReplyRetryInstruction(questionText, lastIssue)
			fields = appendZapFields(logFields, zap.Bool("retry", true), zap.Int("quality_attempt", attempt), zap.String("quality_issue", lastIssue), zap.String("retry_question", currentQuestion))
		}
		reply := strings.TrimSpace(getAIReplyForQualityRetry(contents, currentQuestion, topics, tags, fields...))
		if reply == "" {
			return "", false
		}
		issue := aiReplyQualityIssue(reply)
		if issue == "" {
			return reply, false
		}
		lastIssue = issue
		if attempt < maxReplyQualityAttempts {
			loger.Loger.Warn("[XHH]AI回复质量检查未通过，继续重试", appendZapFields(logFields, zap.Int("quality_attempt", attempt), zap.Int("max_attempts", maxReplyQualityAttempts), zap.String("issue", issue), zap.String("reply", reply))...)
			continue
		}
		loger.Loger.Warn("[XHH]跳过低质量AI回复，已达到质量重试上限", appendZapFields(logFields, zap.Int("quality_attempts", attempt), zap.String("issue", issue), zap.String("reply", reply))...)
		return "", true
	}
	return "", true
}

func generateFeedReplyWithQualityRetry(prompt string, contents []ai.Content, instruction, title string, topics []ai.Topics, tags []ai.Tags, logFields ...zap.Field) string {
	lastIssue := ""
	for attempt := 1; attempt <= maxReplyQualityAttempts; attempt++ {
		currentInstruction := instruction
		fields := logFields
		if attempt > 1 {
			currentInstruction = feedReplyRetryInstruction(instruction, lastIssue)
			fields = appendZapFields(logFields, zap.Bool("retry", true), zap.Int("quality_attempt", attempt), zap.String("quality_issue", lastIssue), zap.String("retry_question", currentInstruction))
		}
		reply := sanitizeFeedReply(getAIFeedReplyForQualityRetry(prompt, contents, currentInstruction, topics, tags, fields...))
		if reply == "" {
			return ""
		}
		issue := feedReplyQualityIssue(reply, title)
		if issue == "" {
			return reply
		}
		lastIssue = issue
		if attempt < maxReplyQualityAttempts {
			loger.Loger.Warn("[FeedReply]回复质量检查未通过，继续重试", appendZapFields(logFields, zap.Int("quality_attempt", attempt), zap.Int("max_attempts", maxReplyQualityAttempts), zap.String("issue", issue), zap.String("reply", reply))...)
			continue
		}
		loger.Loger.Warn("[FeedReply]跳过低质量回复，已达到质量重试上限", appendZapFields(logFields, zap.Int("quality_attempts", attempt), zap.String("issue", issue), zap.String("reply", reply))...)
		return reply
	}
	return ""
}

func aiReplyRetryInstruction(questionText, issue string) string {
	questionText = strings.TrimSpace(questionText)
	if questionText == "" {
		questionText = "对方只是 @ 你，没有补充问题。"
	}
	var builder strings.Builder
	builder.WriteString(questionText)
	builder.WriteString("\n\n上一次回复质量不合格，原因：")
	builder.WriteString(issue)
	builder.WriteString("。请重新生成。要求：像当前配置的人设本人在小黑盒评论区自然接话；先回应对方说的话；用态度、情绪和判断体现人设，不要靠反复自称名字、种族、招牌技能或口头禅证明人设；不要客服腔；不要输出 SKIP；默认1-3句。")
	builder.WriteString("\nNatural rewrite note: answer the user's actual words first; do not stack persona terms such as 红魔族、爆裂魔法、本大魔法师、委托、召唤、咒文 in one reply.")
	return builder.String()
}

func appendZapFields(fields []zap.Field, extra ...zap.Field) []zap.Field {
	out := make([]zap.Field, 0, len(fields)+len(extra))
	out = append(out, fields...)
	out = append(out, extra...)
	return out
}
