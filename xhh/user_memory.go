package xhh

import (
	"openxhh/ai"
	"openxhh/db"
	"strings"
)

const userMemoryPromptMaxChars = 520

func appendUserMemoryContext(contents []ai.Content, userID int, userName string) []ai.Content {
	text := userMemoryContextText(userID, userName)
	if text == "" {
		return contents
	}
	return append(contents, ai.Content{Type: "text", Text: text})
}

func userMemoryContextText(userID int, userName string) string {
	parts := []string{}
	if memory, ok := db.GetUserMemory(int64(userID)); ok {
		name := strings.TrimSpace(userName)
		if name == "" {
			name = memory.UserName
		}
		header := "和当前用户的关系记忆："
		if name != "" {
			header += name + "，"
		}
		header += memory.Summary
		parts = append(parts, header)
	}
	if mood := db.BotMood(); mood != "" {
		parts = append(parts, "你此刻的短期心情："+mood)
	}
	if len(parts) == 0 {
		return ""
	}
	parts = append(parts, "Do not claim real consciousness, perfect memory, or permanent human identity.")
	return limitUserMemoryPrompt(strings.Join(parts, "\n") + "\n使用方式：只把这些当作私下记得的关系背景，不要生硬复述，不要说“根据记忆”。")
}

func rememberSuccessfulReply(v db.CommStruct, questionText, replyText string) {
	db.SaveUserInteraction(int64(v.Uid), v.UserName, questionText, replyText, timeForMemory())
	db.SaveBotMood(nextBotMood(questionText, replyText))
}

func nextBotMood(questionText, replyText string) string {
	text := questionText + " " + replyText
	switch {
	case strings.Contains(text, "可爱") || strings.Contains(text, "喜欢") || strings.Contains(text, "夸"):
		return "刚被夸过，有点得意，但嘴上不会太承认"
	case strings.Contains(text, "转人工") || strings.Contains(text, "转达克尼斯") || strings.Contains(text, "转人妻"):
		return "刚被人拿转接梗逗过，有点炸毛但愿意接梗"
	case strings.Contains(text, "难受") || strings.Contains(text, "怎么办") || strings.Contains(text, "崩溃"):
		return "刚认真接过一个求助或吐槽，语气会稍微放软"
	case strings.Contains(text, "有缘") || strings.Contains(text, "又遇到"):
		return "刚遇到熟人式互动，心情有点新奇和得意"
	default:
		return "正常在线，带一点好奇和红魔族式自信"
	}
}

func limitUserMemoryPrompt(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= userMemoryPromptMaxChars {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:userMemoryPromptMaxChars]))
}

var timeForMemory = func() int64 {
	return 0
}
