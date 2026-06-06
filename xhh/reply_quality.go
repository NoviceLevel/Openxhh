package xhh

import (
	"openxhh/ai"
	"openxhh/loger"
	"strings"

	"go.uber.org/zap"
)

func generateAIReplyWithQualityRetry(contents []ai.Content, questionText string, topics []ai.Topics, tags []ai.Tags, logFields ...zap.Field) (string, bool) {
	reply := strings.TrimSpace(ai.GetAiReply(contents, questionText, topics, tags, logFields...))
	if reply == "" {
		return "", false
	}
	if issue := aiReplyQualityIssue(reply); issue != "" {
		loger.Loger.Warn("[XHH]AI回复质量检查未通过，重试一次", appendZapFields(logFields, zap.String("issue", issue), zap.String("reply", reply))...)
		retryQuestion := aiReplyRetryInstruction(questionText, issue)
		reply = strings.TrimSpace(ai.GetAiReply(contents, retryQuestion, topics, tags, appendZapFields(logFields, zap.Bool("retry", true), zap.String("quality_issue", issue), zap.String("retry_question", retryQuestion))...))
	}
	if reply == "" {
		return "", false
	}
	if issue := aiReplyQualityIssue(reply); issue != "" {
		loger.Loger.Warn("[XHH]跳过低质量AI回复", appendZapFields(logFields, zap.String("issue", issue), zap.String("reply", reply))...)
		return "", true
	}
	return reply, false
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
	builder.WriteString("。请重新生成。要求：必须像当前配置的人设本人在小黑盒评论区回复；先回应对方说的话；体现当前人设的身份感和说话习惯")
	if anchors := aiReplyPersonaAnchors(); len(anchors) > 0 {
		builder.WriteString("，可自然使用这些人设锚点：")
		builder.WriteString(strings.Join(limitStringSlice(anchors, 8), "、"))
	}
	builder.WriteString("；不要客服腔；不要输出 SKIP；默认1-3句。")
	return builder.String()
}

func appendZapFields(fields []zap.Field, extra ...zap.Field) []zap.Field {
	out := make([]zap.Field, 0, len(fields)+len(extra))
	out = append(out, fields...)
	out = append(out, extra...)
	return out
}
