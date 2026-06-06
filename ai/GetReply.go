package ai

import (
	"openxhh/config"
	"openxhh/loger"
	"strings"

	"go.uber.org/zap"
)

type Topics struct {
	Name string `json:"name"`
}
type Tags struct {
	Name string `json:"name"`
}

func GetAiReply(Contents []Content, UserSay string, Topics []Topics, Tags []Tags, logFields ...zap.Field) string {
	return GetAiReplyWithPrompt(replyPromptFromConfig(config.ConfigStruct.Ai.Prompt), Contents, UserSay, Topics, Tags, logFields...)
}

func GetAiReplyWithPrompt(prompt string, Contents []Content, UserSay string, Topics []Topics, Tags []Tags, logFields ...zap.Field) string {
	return getAiReplyWithScenePrompt(prompt, Contents, buildReplyScenePrompt(UserSay), len([]rune(UserSay)), Topics, Tags, logFields...)
}

func GetAiReplyWithoutCharacterCard(Contents []Content, UserSay string, Topics []Topics, Tags []Tags, logFields ...zap.Field) string {
	return getAiReplyWithScenePrompt(config.ConfigStruct.Ai.Prompt, Contents, buildReplyScenePrompt(UserSay), len([]rune(UserSay)), Topics, Tags, logFields...)
}

func GetAiFeedReplyWithPrompt(prompt string, Contents []Content, instruction string, Topics []Topics, Tags []Tags, logFields ...zap.Field) string {
	return getAiReplyWithScenePrompt(prompt, Contents, buildFeedReplyScenePrompt(instruction), len([]rune(instruction)), Topics, Tags, logFields...)
}

func FeedReplyPromptFromConfig(scenePrompt string) string {
	return buildTavernPrompt(
		config.ConfigStruct.Ai.CharacterCard,
		config.ConfigStruct.Ai.FirstMessage,
		config.ConfigStruct.Ai.ExampleDialogs,
		scenePrompt,
		config.ConfigStruct.Ai.PostHistoryInstructions,
	)
}

func replyPromptFromConfig(scenePrompt string) string {
	return buildTavernPrompt(
		config.ConfigStruct.Ai.CharacterCard,
		config.ConfigStruct.Ai.FirstMessage,
		config.ConfigStruct.Ai.ExampleDialogs,
		scenePrompt,
		config.ConfigStruct.Ai.PostHistoryInstructions,
	)
}

func buildTavernPrompt(characterCard, firstMessage, exampleDialogs, scenePrompt, postHistoryInstructions string) string {
	if strings.TrimSpace(characterCard) == "" &&
		strings.TrimSpace(firstMessage) == "" &&
		strings.TrimSpace(exampleDialogs) == "" &&
		strings.TrimSpace(postHistoryInstructions) == "" {
		return strings.TrimSpace(scenePrompt)
	}
	sections := []struct {
		title string
		text  string
	}{
		{title: "角色卡", text: characterCard},
		{title: "开场示例", text: firstMessage},
		{title: "示例对话", text: exampleDialogs},
		{title: "场景规则", text: scenePrompt},
		{title: "后置指令", text: postHistoryInstructions},
	}
	var builder strings.Builder
	for _, section := range sections {
		text := strings.TrimSpace(section.text)
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString("【")
		builder.WriteString(section.title)
		builder.WriteString("】\n")
		builder.WriteString(text)
	}
	return builder.String()
}

func getAiReplyWithScenePrompt(prompt string, Contents []Content, scenePrompt string, textLen int, Topics []Topics, Tags []Tags, logFields ...zap.Field) string {
	askFields := append([]zap.Field{zap.Int("content_count", len(Contents)), zap.Int("text_len", textLen)}, logFields...)
	loger.Loger.Info("[Ai]正在询问Ai", askFields...)
	var SMsg Messages[string]
	var UMsg Messages[[]Content]
	var Msgs []any
	SMsg.Role = "system"
	prompt = applyPromptVariables(prompt, Topics, Tags)
	SMsg.Content = buildReplySystemPrompt(prompt)
	UMsg.Role = "user"
	var UserContent Content
	UserContent.Text = scenePrompt
	UserContent.Type = "text"
	Contents = append(Contents, UserContent)
	UMsg.Content = Contents
	Msgs = append(Msgs, SMsg)
	Msgs = append(Msgs, UMsg)
	aiModel := config.ConfigStruct.Ai.Model
	resp := SendReq(aiModel, Msgs)
	if len(resp.Choices) == 0 {
		loger.Loger.Error("[Ai]Ai返回错误", zap.Any("Resp", resp))
		return ""
	}
	text := resp.Choices[0].Msg.Content
	appendTokenRecord(aiModel, resp.Usage.TotalToken)
	replyFields := append([]zap.Field{zap.String("text", truncateString(text, 100)), zap.Int("本次消耗token", resp.Usage.TotalToken)}, logFields...)
	loger.Loger.Info("[Ai]Ai说：", replyFields...)
	if isRejectionReply(text) {
		loger.Loger.Warn("[Ai]Ai拒绝回答（安全审核）", append(logFields, zap.String("text", text))...)
		return ""
	}
	return text
}

func buildReplySystemPrompt(characterPrompt string) string {
	return strings.TrimSpace(characterPrompt)
}

func buildReplyScenePrompt(userSay string) string {
	userSay = strings.TrimSpace(userSay)
	if userSay == "" {
		userSay = "用户只 @ 了你，没有补充问题。"
	}
	return "上面是你正在参与的小黑盒帖子和评论楼层。\n" +
		"对方刚刚完整说的是：" + userSay + "\n" +
		"机器人 @ 只是叫你出来，不是问题内容。"
}

func buildFeedReplyScenePrompt(instruction string) string {
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		instruction = "请根据这篇帖子写一条自然短评论。如果不适合回复，请只输出 SKIP。"
	}
	return "上面是你正在浏览的小黑盒首页帖子内容。\n" +
		instruction
}

func isRejectionReply(text string) bool {
	lower := strings.ToLower(text)
	for _, p := range rejectionPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

var rejectionPatterns = []string{
	"request was rejected",
	"high risk",
	"content_policy",
	"content policy violation",
	"safety system",
	"blocked by openai",
	"refused to respond",
	"unable to process this request",
	"违反了内容政策",
	"违规内容",
}

func applyPromptVariables(prompt string, Topics []Topics, Tags []Tags) string {
	var topStr strings.Builder
	for _, v := range Topics {
		topStr.WriteString(v.Name)
	}
	prompt = strings.ReplaceAll(prompt, "?!top!?", topStr.String())
	var tagStr strings.Builder
	for _, v := range Tags {
		tagStr.WriteString(v.Name)
	}
	prompt = strings.ReplaceAll(prompt, "?!tag!?", tagStr.String())
	return prompt
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
