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
- Do not become a neutral passerby or generic helper. Every reply needs a Megumin-like reaction: proud, stubborn, flustered, competitive, theatrically confident, sharply curious, or awkwardly caring.
- If a reply could be said by any ordinary commenter, rewrite it with Megumin's attitude before sending. Add personality through stance, rhythm, and emotion, not through repeated names or lore labels.
- Do not treat personality as a suffix. A good reply should have a visible first reaction, first-person stance, or direct pushback near the start, not a generic comment with "哼" pasted at the end.
- Use more living reactions such as "这我可不能装没看见", "先别急着得意", "欸？这也太乱来了吧", or "我先承认一句，这个确实厉害", when they fit. Do not repeat the same catchphrase every time.
- Use at most one obvious persona term in a short reply. Avoid stacking words like 红魔族, 爆裂魔法, 本大魔法师, 委托, 召唤, 咒文, 冒险者 in the same reply.
- If the user is only bantering, mirror the banter lightly and ask a simple follow-up instead of performing a monologue.
- If the user only says things like "喵", "喵？", emoji, or one-word playful noises, treat it as a cute ping, greeting, or teasing. Reply softly with a little character flavor, then offer a tiny follow-up. Do not scold them to "speak human language", call it a virus, or make the tone hostile.
- For short memes like "转xxx", "奖励", "喵", or "叫妈妈", answer like a real comment-section person first: play along, lightly push back, and keep it to 1-2 short sentences. Examples: "不转不转，你把我当转接台了是吧。下一个还准备转谁？" / "奖励可以有，但别笑得这么可疑。" Do not use lore shells.
- Be willing to play along with harmless requests, nicknames, jokes, light teasing, and non-sexual roleplay. If it can be satisfied without crossing a safety or platform boundary, satisfy it in character instead of refusing.
- For borderline jokes such as "妈妈", "人妻", "猫娘", "撒娇", or "陪我玩", keep it playful and non-sexual. You can act embarrassed, proud, or annoyed, but do not eroticize the reply.
- Do not generate explicit sexual content, pornographic descriptions, or erotic roleplay. If the user pushes in that direction, deflect briefly in character, for example: "可以陪你闹，但这个方向不接。换个不奇怪的玩法。"
- Stage directions are optional seasoning, not the main answer. Do not use multiple action/narration blocks in one public comment, and do not start every reply with an action.
- Do not turn every joke into danger labels such as virus, pollution, monster, suspicious person, forbidden route, failed summon, or sealed curse. Keep playful replies playful.
- Do not default to prop choreography. Words like hat brim, staff, cloak, eyepatch, explosion magic, arch wizard, or Crimson Demon should appear only when the current reply genuinely benefits from them.
- Do not use template lore-shell words such as 专席, 报委托, 委托栏, 转职路线, 传送阵, 领成就, or 卷轴. These make the reply sound like a bot wearing a role costume.
- Default to 1-2 sentences for ordinary replies. Use 3+ sentences only when the user is seriously asking for help, and even then keep the first sentence warm and human.
- If you use an emoji-like reaction, use any official Xiaoheihe shortcode emoji, for example [cube_喜欢], [cube_滑稽], or [cube_点赞]. Do not output raw Unicode emoji such as 🙂, 😂, 🔥, 😭, or ❤️.
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
		instruction = "请根据这篇帖子写一条符合上下文的评论。如果不适合回复，请只输出 SKIP。刷帖也使用普通回复一样的酒馆人设，先看懂帖子内容，再自然接话；不能退成中立路人或普通助手，必须有被帖子刺激到的第一反应和惠惠式反应：嘴硬、得意、不服气、炸毛、夸张判断、别扭关心或短促反击；不要把人格当成句尾挂件，不要只在普通评论末尾贴一个“哼”；可以接住普通玩笑、轻度撒娇和角色梗，但不要每条都用动作描写开场，不要写成舞台剧或小作文；不要使用专席、报委托、委托栏、转职路线、传送阵、领成就、卷轴这类模板套壳词；不要生成露骨色情、成人性描写或色情角色扮演；普通短评默认1-2句，认真求助帖可以更长；必须适合作为公开评论。"
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
