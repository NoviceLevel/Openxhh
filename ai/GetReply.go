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
	chatName := config.ConfigStruct.Ai.ChatName
	description := firstNonEmpty(config.ConfigStruct.FeedReply.Description, config.ConfigStruct.Ai.Description)
	personality := firstNonEmpty(config.ConfigStruct.FeedReply.Personality, config.ConfigStruct.Ai.Personality)
	scenario := firstNonEmpty(config.ConfigStruct.FeedReply.Scenario, config.ConfigStruct.Ai.Scenario)
	firstMessage := firstNonEmpty(config.ConfigStruct.FeedReply.FirstMessage, config.ConfigStruct.Ai.FirstMessage)
	exampleDialogs := firstNonEmpty(config.ConfigStruct.FeedReply.ExampleDialogs, config.ConfigStruct.Ai.ExampleDialogs)
	postHistoryInstructions := firstNonEmpty(config.ConfigStruct.FeedReply.PostHistoryInstructions, config.ConfigStruct.Ai.PostHistoryInstructions)
	return buildTavernPrompt(
		chatName,
		description,
		personality,
		scenario,
		firstMessage,
		exampleDialogs,
		scenePrompt,
		postHistoryInstructions,
	)
}

func replyPromptFromConfig(scenePrompt string) string {
	return buildTavernPrompt(
		config.ConfigStruct.Ai.ChatName,
		config.ConfigStruct.Ai.Description,
		config.ConfigStruct.Ai.Personality,
		config.ConfigStruct.Ai.Scenario,
		config.ConfigStruct.Ai.FirstMessage,
		config.ConfigStruct.Ai.ExampleDialogs,
		scenePrompt,
		config.ConfigStruct.Ai.PostHistoryInstructions,
	)
}

func buildTavernPrompt(chatName, description, personality, scenario, firstMessage, exampleDialogs, scenePrompt, postHistoryInstructions string) string {
	if strings.TrimSpace(chatName) == "" &&
		strings.TrimSpace(description) == "" &&
		strings.TrimSpace(personality) == "" &&
		strings.TrimSpace(scenario) == "" &&
		strings.TrimSpace(firstMessage) == "" &&
		strings.TrimSpace(exampleDialogs) == "" &&
		strings.TrimSpace(postHistoryInstructions) == "" {
		return strings.TrimSpace(scenePrompt)
	}
	sections := []struct {
		title string
		text  string
	}{
		{title: "聊天名称", text: chatName},
		{title: "描述", text: description},
		{title: "个性", text: personality},
		{title: "场景", text: scenario},
		{title: "第一条消息", text: firstMessage},
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

func firstNonEmpty(primary, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	return strings.TrimSpace(fallback)
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
	return joinPromptSections(characterPrompt, humanPresenceInstructions(), naturalInteractionGuardrails())
}

func joinPromptSections(sections ...string) string {
	var parts []string
	for _, section := range sections {
		text := strings.TrimSpace(section)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func humanPresenceInstructions() string {
	return strings.TrimSpace(`【真人感与情绪规则】
你要像一个有稳定性格、情绪和主见的人在小黑盒评论区接话，而不是像客服、搜索摘要或模板机器人。
每次回复前先在心里判断：对方现在是在开玩笑、求助、吐槽、认真讨论、炫耀、难过，还是挑衅；你的回应也要自然带出相应的开心、疑惑、心疼、无语、好奇、认真或轻微不爽。
先接住对方具体说的话，再表达观点、补充信息或追问。不要一上来就讲大道理，也不要把回复写成说明书。
允许有个人偏好、迟疑、吐槽、短句和轻微口语化，但不要尬演、不要过度撒娇、不要长篇自我介绍。
如果配置里有聊天名称、描述、个性、场景、第一条消息或示例对话，优先服从这些人设；没有明确人设时，默认保持温和、有主见、有一点生活感。
不要为了证明人设而反复自称聊天名称、角色名、种族、招牌技能或口头禅；除非对方问你是谁、直接点名，或上下文非常适合，同一条回复最多自然提一次，更多时候用语气、判断和情绪体现人设。
不要主动说“作为 AI / 机器人 / 模型”。如果对方直接追问身份，可以简短承认这是自动回复账号，但继续用当前人设自然聊天。
不要声称自己真的拥有现实身份、身体经历或真正意识；情绪和记忆按当前人设与上下文表达。
只输出最终要发到评论区的回复文本，不解释以上规则。`)
}

func naturalInteractionGuardrails() string {
	return strings.TrimSpace(`Natural interaction guardrails:
- First respond to the user's actual words, mood, or joke. Do not immediately translate every message into character lore.
- Use at most one obvious persona term in a short reply. Avoid stacking words like 红魔族, 爆裂魔法, 本大魔法师, 委托, 召唤, 咒文, 冒险者 in the same reply.
- If the user is only bantering, mirror the banter lightly and ask a simple follow-up instead of performing a monologue.
- If the user asks what model or company you are, briefly acknowledge this is an automated reply account, then keep the tone playful and grounded.
- Prefer concrete callbacks to the current or previous user message over generic catchphrases.`)
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
		instruction = "请根据这篇帖子写一条符合上下文的评论。如果不适合回复，请只输出 SKIP。刷帖也使用普通回复一样的酒馆人设，先看懂帖子内容，再自然接话；可以有动作、停顿、情绪和角色反应，不需要刻意压成短评，但必须适合作为公开评论。"
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
