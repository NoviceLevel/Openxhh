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
	replyContents := appendTransferRoleInstruction(contents, questionText)
	reply := strings.TrimSpace(getAIReplyForQualityRetry(replyContents, questionText, topics, tags, logFields...))
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
	if transferRole := transferRoleCommandTarget(questionText); transferRole != "" {
		retryQuestion += " 这条是“转" + transferRole + "”转接梗，请临时用“" + transferRole + "”的角色口吻说一句；不要输出“转" + transferRole + "”，不要说正在转接或已转接。"
	}
	retry := strings.TrimSpace(getAIReplyForQualityRetry(replyContents, retryQuestion, topics, tags, logFields...))
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

func appendTransferRoleInstruction(contents []ai.Content, questionText string) []ai.Content {
	transferRole := transferRoleCommandTarget(questionText)
	if transferRole == "" {
		return contents
	}
	out := make([]ai.Content, 0, len(contents)+1)
	out = append(out, contents...)
	out = append(out, ai.Content{
		Type: "text",
		Text: "用户刚刚在玩“转" + transferRole + "”的转接梗。请把它理解为：临时用“" + transferRole + "”的说话口吻回一句中文短句。不要真的声称已转接或执行命令，不要复读“转" + transferRole + "”，不要解释规则；最多一句，像评论区自然接梗。",
	})
	return out
}

func transferRoleCommandTarget(text string) string {
	text = strings.TrimSpace(NormalizeCommentText(text))
	text = strings.Trim(text, " \t\r\n，,。.!！?？:：;；“”\"'「」『』（）()[]【】")
	for _, prefix := range []string{"请", "麻烦", "帮我", "帮忙"} {
		text = strings.TrimSpace(strings.TrimPrefix(text, prefix))
	}
	if text == "" || !strings.HasPrefix(text, "转") {
		return ""
	}
	for _, prefix := range []string{"转发", "转账", "转让", "转载", "转帖", "转移", "转换", "转职", "转码", "转卖", "转圈", "转身", "转头", "转向"} {
		if strings.HasPrefix(text, prefix) {
			return ""
		}
	}
	target := strings.TrimSpace(strings.TrimPrefix(text, "转"))
	target = strings.TrimPrefix(target, "到")
	target = strings.TrimSpace(target)
	target = strings.Trim(target, " \t\r\n，,。.!！?？:：;；“”\"'「」『』（）()[]【】")
	for _, suffix := range []string{"一下下", "一下", "看看", "看下", "吧", "呗", "啊", "呀", "啦", "呢"} {
		target = strings.TrimSuffix(target, suffix)
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if strings.ContainsAny(target, " \t\r\n，,。.!！?？:：;；") {
		return ""
	}
	if len([]rune(target)) > 16 {
		return ""
	}
	return target
}

func generateFeedReplyWithQualityRetry(prompt string, contents []ai.Content, instruction, title string, topics []ai.Topics, tags []ai.Tags, logFields ...zap.Field) string {
	reply := sanitizeFeedReply(getAIFeedReplyForQualityRetry(prompt, contents, instruction, topics, tags, logFields...))
	if feedReplyQualityIssue(reply, title) == "" {
		return reply
	}
	retryInstruction := instruction + "\n\n上一条太长或太像说明文。请改成像正常人在评论区随手回的一句话，最多两句；保留惠惠的嘴硬、得意或炸毛，但不要展开讲道理。"
	return sanitizeFeedReply(getAIFeedReplyForQualityRetry(prompt, contents, retryInstruction, topics, tags, logFields...))
}
