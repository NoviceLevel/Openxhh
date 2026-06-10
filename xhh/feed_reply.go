package xhh

import (
	"encoding/json"
	"io"
	"openxhh/ai"
	"openxhh/config"
	"openxhh/db"
	"openxhh/loger"
	"strconv"
	"strings"
	"time"
	"unicode"

	"go.uber.org/zap"
)

type feedResponse struct {
	Msg    string `json:"msg"`
	Result struct {
		Links []feedLink `json:"links"`
	} `json:"result"`
	Status string `json:"status"`
}

type feedLink struct {
	LinkID      int         `json:"linkid"`
	UserID      int         `json:"userid"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	CreateAt    int64       `json:"create_at"`
	ModifyAt    int64       `json:"modify_at"`
	Topics      []ai.Topics `json:"topics"`
	Tags        []ai.Tags   `json:"hashtags"`
	User        struct {
		UserID   json.RawMessage `json:"userid"`
		UserName string          `json:"username"`
	} `json:"user"`
}

func AutoFeedReply() {
	for {
		if remaining := xhhCaptchaCooldownRemaining(); remaining > 0 {
			time.Sleep(remaining)
			continue
		}
		if !config.ConfigStruct.FeedReply.Enabled {
			time.Sleep(time.Duration(feedReplyInterval()) * time.Second)
			continue
		}
		if remaining := feedReplyPersistedWait(); remaining > 0 {
			loger.Loger.Info("[FeedReply]等待持久化刷帖间隔", zap.Int64("remaining_seconds", int64(remaining/time.Second)), zap.Int("interval", feedReplyInterval()))
			time.Sleep(remaining)
			continue
		}
		processFeedReplyOnce()
		db.SaveFeedReplyLastRunAt(time.Now().Unix())
	}
}

func feedReplyPersistedWait() time.Duration {
	if !config.ConfigStruct.FeedReply.Enabled {
		return 0
	}
	lastRunAt := db.FeedReplyLastRunAt()
	if lastRunAt <= 0 {
		return 0
	}
	nextRunAt := lastRunAt + int64(feedReplyInterval())
	now := time.Now().Unix()
	if nextRunAt <= now {
		return 0
	}
	return time.Duration(nextRunAt-now) * time.Second
}

func processFeedReplyOnce() {
	cfg := config.ConfigStruct.FeedReply
	if !cfg.Enabled {
		return
	}
	maxPerRun := feedReplyMaxPerRun()
	maxPerDay := feedReplyMaxPerDay()
	since := time.Now().Add(-24 * time.Hour).Unix()
	usedToday := db.FeedReplyAttemptsSince(since)
	if maxPerDay > 0 && usedToday >= maxPerDay {
		loger.Loger.Info("[FeedReply]今日自动刷帖额度已用完", zap.Int("max_per_day", maxPerDay))
		return
	}
	links := fetchFeedLinks()
	if len(links) == 0 {
		loger.Loger.Info("[FeedReply]feeds 暂无可处理帖子")
		return
	}
	processed := 0
	for _, link := range links {
		if processed >= maxPerRun {
			break
		}
		if maxPerDay > 0 && usedToday >= maxPerDay {
			break
		}
		if link.LinkID <= 0 || db.FeedReplyRecordExists(int64(link.LinkID)) {
			continue
		}
		record := processFeedLink(link)
		db.SaveFeedReplyRecord(record)
		processed++
		if record.Status == "sent" || record.Status == "dry_run" {
			usedToday++
		}
	}
	loger.Loger.Info("[FeedReply]自动刷帖批次完成", zap.Int("processed", processed), zap.Int("used_today", usedToday), zap.Int("max_per_day", maxPerDay), zap.Bool("dry_run", feedReplyDryRun()))
}

func fetchFeedLinks() []feedLink {
	if xhhCaptchaCoolingDown("feeds") {
		return nil
	}
	resp := SendReq("GET", "/bbs/app/feeds", nil, "?pull=1")
	if resp == nil {
		loger.Loger.Error("[FeedReply]feeds 请求失败")
		IsErr()
		return nil
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		loger.Loger.Error("[FeedReply]无法读取 feeds 响应", zap.Error(err), zap.Int("status", resp.StatusCode))
		return nil
	}
	if !isHTTPSuccess(resp.StatusCode) {
		body := string(data)
		loger.Loger.Warn("[FeedReply]feeds HTTP 失败", zap.Int("status", resp.StatusCode), zap.String("body", readableXHHResponseBody(body)))
		handleXHHHTTPFailure("feeds", resp.StatusCode, body)
		return nil
	}
	var parsed feedResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		loger.Loger.Error("[FeedReply]feeds 反序列化失败", zap.Error(err), zap.String("body", readableXHHResponseBody(string(data))))
		return nil
	}
	if parsed.Status != "ok" {
		loger.Loger.Warn("[FeedReply]feeds 返回失败", zap.String("msg", parsed.Msg))
		return nil
	}
	loger.Loger.Info("[FeedReply]feeds 拉取完成", zap.Int("count", len(parsed.Result.Links)))
	return parsed.Result.Links
}

func processFeedLink(link feedLink) db.FeedReplyRecord {
	now := time.Now().Unix()
	authorID := int64(link.UserID)
	if authorID == 0 {
		authorID = int64(jsonInt(link.User.UserID))
	}
	record := db.FeedReplyRecord{
		LinkID:    int64(link.LinkID),
		Title:     link.Title,
		AuthorID:  authorID,
		Author:    link.User.UserName,
		PostText:  limitFeedText(feedText(link), 1000),
		Status:    "pending",
		CreatedAt: link.CreateAt,
		RepliedAt: now,
	}
	contents, topics, tags, _ := GetLinkInfo(link.LinkID, 0, -1, 0)
	if len(contents) == 0 {
		contents = fallbackFeedContents(link)
		topics = link.Topics
		tags = link.Tags
	}
	instruction := buildFeedReplyInstruction(link)
	logFields := []zap.Field{zap.Bool("feed_reply", true), zap.Int("link_id", link.LinkID), zap.Int64("author_id", authorID), zap.String("author_name", link.User.UserName), zap.String("feed_title", link.Title), zap.String("question", instruction)}
	reply := generateFeedReplyWithQualityRetry(ai.FeedReplyPromptFromConfig(config.ConfigStruct.FeedReply.Prompt), contents, instruction, link.Title, topics, tags, logFields...)
	if reply == "" {
		record.Status = "failed"
		record.Reason = "AI 返回空内容"
		return record
	}
	if shouldSkipFeedReply(reply) {
		record.Status = "skipped"
		record.Reason = "AI 判断不适合回复"
		record.ReplyText = reply
		loger.Loger.Info("[FeedReply]跳过帖子", zap.Int("link_id", link.LinkID), zap.String("title", link.Title), zap.String("reason", record.Reason))
		return record
	}
	if issue := feedReplyQualityIssue(reply, link.Title); issue != "" {
		record.Status = "skipped"
		record.Reason = "回复质量检查未通过：" + issue
		record.ReplyText = reply
		loger.Loger.Warn("[FeedReply]跳过低质量回复", zap.Int("link_id", link.LinkID), zap.String("title", link.Title), zap.String("issue", issue), zap.String("reply", reply))
		return record
	}
	record.ReplyText = reply
	if feedReplyDryRun() {
		record.Status = "dry_run"
		record.Reason = "试运行未发送"
		loger.Loger.Info("[FeedReply]试运行生成回复", zap.Int("link_id", link.LinkID), zap.String("title", link.Title), zap.String("reply", reply))
		return record
	}
	if ReplyPost(reply, strconv.Itoa(link.LinkID)) {
		record.Status = "sent"
		loger.Loger.Info("[FeedReply]评论发送成功", zap.Int("link_id", link.LinkID), zap.String("title", link.Title), zap.String("reply", reply))
		return record
	}
	record.Status = "failed"
	record.Reason = "评论发送失败"
	loger.Loger.Warn("[FeedReply]评论发送失败", zap.Int("link_id", link.LinkID), zap.String("title", link.Title))
	return record
}

func ReplyPost(text, linkID string) bool {
	return createCommentSent("feed_reply", text, linkID, "-1", "-1", "0", "")
}

func buildFeedReplyInstruction(link feedLink) string {
	return "请根据这篇帖子写一条符合上下文的评论。如果不适合回复，请只输出 SKIP。刷帖也使用普通回复一样的酒馆人设，先看懂帖子内容，再自然接话；可以有轻微情绪和角色反应，可以接住普通玩笑、轻度撒娇和角色梗，但不要每条都用动作描写开场，不要写成舞台剧或小作文；不要使用专席、报委托、委托栏、转职路线、传送阵、领成就、卷轴这类模板套壳词；不要生成露骨色情、成人性描写或色情角色扮演；普通短评默认1-2句，认真求助帖可以更长；必须适合作为公开评论。标题：" + link.Title + "\n正文摘要：" + link.Description
}

func fallbackFeedContents(link feedLink) []ai.Content {
	text := "以下是帖子内容：\n标题：" + link.Title
	if strings.TrimSpace(link.Description) != "" {
		text += "\n正文：" + link.Description
	}
	return []ai.Content{{Type: "text", Text: text}}
}

func feedText(link feedLink) string {
	parts := []string{link.Title, link.Description}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

const xhhCommentMaxRunes = 1000

func sanitizeFeedReply(reply string) string {
	reply = strings.TrimSpace(reply)
	reply = strings.TrimPrefix(reply, "```json")
	reply = strings.TrimPrefix(reply, "```")
	reply = strings.TrimSuffix(reply, "```")
	reply = strings.TrimSpace(reply)
	return limitFeedText(reply, xhhCommentMaxRunes)
}

func shouldSkipFeedReply(reply string) bool {
	value := strings.ToUpper(strings.Trim(strings.TrimSpace(reply), " 。.!！?？`\"'"))
	return value == "SKIP" || value == "跳过" || value == "不回复"
}

func feedReplyQualityIssue(reply string, title string) string {
	return replyQualityIssue(reply, title, feedReplyPersonaAnchors(), true, true)
}

func aiReplyQualityIssue(reply string) string {
	return replyQualityIssue(reply, "", aiReplyPersonaAnchors(), false, false)
}

func replyQualityIssue(reply string, title string, anchors []string, checkTitle bool, allowSkip bool) string {
	reply = strings.TrimSpace(reply)
	if reply == "" || (allowSkip && shouldSkipFeedReply(reply)) {
		return ""
	}
	if shouldSkipFeedReply(reply) {
		return "回复为跳过指令"
	}
	if len([]rune(reply)) > xhhCommentMaxRunes {
		return "回复过长"
	}
	if containsExplicitSexualContent(reply) {
		return "回复包含露骨色情内容"
	}
	if containsAny(reply, []string{"我理解你的意思", "总结一下", "建议你", "您好", "作为AI", "作为 AI", "我是AI", "我是 AI", "机器人"}) {
		return "客服腔或暴露 AI 身份"
	}
	if containsAny(reply, []string{"翻译成人话", "说人话", "人话给我听", "猫化病毒", "病毒扩散", "病毒已经扩散"}) {
		return "轻互动回复过凶，缺少可爱感"
	}
	if containsAny(reply, []string{"病毒污染", "高危魔物", "可疑发言人员", "奇怪路线", "低阶召唤失败", "猫夺舍"}) {
		return "玩笑回复过度危险化"
	}
	if containsPersonaShellTemplateWords(reply) {
		return "角色套壳词过重，像模板回复"
	}
	if containsRawEmoji(reply) {
		return "使用了非小黑盒官方表情"
	}
	if containsUnsuitableXHHEmoji(reply) {
		return "使用了不符合惠惠的油腻表情"
	}
	if overusesCharacterProps(reply) {
		return "道具动作过密，像舞台表演"
	}
	if overusesChatName(reply) {
		return "repeats character name too often"
	}
	if overusesPersonaPerformanceTerms(reply) {
		return "角色设定词过密，像在表演而不是接话"
	}
	if overusesStageDirections(reply) {
		return "动作描写过多，像舞台表演"
	}
	if checkTitle && repeatsFeedTitle(reply, title) {
		return "复述标题"
	}
	return ""
}

func overusesChatName(reply string) bool {
	name := strings.TrimSpace(config.ConfigStruct.Ai.ChatName)
	if name == "" {
		return false
	}
	return strings.Count(strings.ToLower(reply), strings.ToLower(name)) > 1
}

func overusesPersonaPerformanceTerms(reply string) bool {
	terms := []string{
		"本大魔法师",
		"红魔族",
		"爆裂魔法",
		"大魔法师",
		"本大人",
		"召唤",
		"委托",
		"咒文",
		"冒险者",
	}
	hits := 0
	for _, term := range terms {
		if strings.Contains(reply, term) {
			hits++
		}
	}
	if hits >= 3 {
		return true
	}
	return hits >= 2 && len([]rune(reply)) <= 45
}

func overusesCharacterProps(reply string) bool {
	terms := []string{"帽檐", "法杖", "披风", "眼罩", "爆裂魔法", "本大魔法师", "红魔族", "大魔法师"}
	hits := 0
	for _, term := range terms {
		if strings.Contains(reply, term) {
			hits++
		}
	}
	if hits >= 4 {
		return true
	}
	return hits >= 3 && len([]rune(reply)) <= 90
}

func containsRawEmoji(reply string) bool {
	for _, r := range reply {
		switch {
		case r >= 0x1F300 && r <= 0x1FAFF:
			return true
		case r >= 0x2600 && r <= 0x27BF:
			return true
		}
	}
	return false
}

func containsUnsuitableXHHEmoji(reply string) bool {
	return containsAny(reply, []string{
		"[cube_哭泣]",
		"[cube_滑稽]",
		"[cube_色]",
		"[cube_坏笑]",
		"[cube_奸笑]",
		"[cube_阴险]",
		"[heygirl_哭泣]",
		"[heygirl_滑稽]",
		"[heygirl_色]",
		"[heygirl_坏笑]",
	})
}

func containsExplicitSexualContent(reply string) bool {
	text := strings.ToLower(strings.TrimSpace(reply))
	if text == "" {
		return false
	}
	if containsAnyFold(text, []string{
		"露骨色情",
		"色情内容",
		"成人剧情",
		"成人内容",
		"成人向剧情",
		"性爱",
		"性交",
		"做爱",
		"约炮",
		"裸聊",
		"口交",
		"肛交",
		"乳交",
		"自慰",
		"手淫",
		"高潮",
		"射精",
		"精液",
		"阴茎",
		"鸡巴",
		"龟头",
		"睾丸",
		"阴道",
		"小穴",
		"阴蒂",
		"淫水",
		"explicit sex",
		"sex roleplay",
		"sexual roleplay",
		"erotic roleplay",
		"pornographic",
		"pornography",
	}) {
		return true
	}
	if strings.Contains(text, "上床") && containsAnyFold(text, []string{"可以", "也行", "陪你", "来吧", "直接"}) {
		return true
	}
	if strings.Contains(text, "脱光") && containsAnyFold(text, []string{"可以", "也行", "陪你", "来吧", "直接"}) {
		return true
	}
	return false
}

func containsPersonaShellTemplateWords(reply string) bool {
	return containsAny(reply, []string{
		"专席",
		"报委托",
		"委托栏",
		"转职路线",
		"传送阵",
		"领成就",
		"成就领了",
		"理由卷轴",
		"解除理由卷轴",
		"魔法卷轴",
		"召唤卷轴",
	})
}

func overusesStageDirections(reply string) bool {
	lines := strings.Split(reply, "\n")
	stageLines := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len([]rune(line)) > 100 {
			continue
		}
		if strings.HasPrefix(line, "*") && strings.HasSuffix(line, "*") {
			stageLines++
		}
	}
	if stageLines >= 2 {
		return true
	}
	return stageLines == 1 && len([]rune(reply)) > 260
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func containsAnyFold(text string, needles []string) bool {
	lowerText := strings.ToLower(text)
	for _, needle := range needles {
		needle = strings.TrimSpace(needle)
		if needle == "" {
			continue
		}
		if strings.Contains(lowerText, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func feedReplyRetryInstruction(instruction, issue string) string {
	var builder strings.Builder
	builder.WriteString(instruction)
	builder.WriteString("\n\n上一次回复质量不合格，原因：")
	builder.WriteString(issue)
	builder.WriteString("。请重新生成。要求：像当前配置的人设本人在小黑盒帖子里自然接话；先回应帖子内容；可以保留一点情绪和角色反应，但不要每次都用动作描写开场，不要写成舞台剧或长段小作文；不要靠反复自称名字、种族、招牌技能或口头禅证明人设；不要复述标题；不要客服腔；普通短评默认1-2句，认真求助才可以更长；必须适合作为公开评论。")
	builder.WriteString("\nNatural rewrite note: answer the post itself first; be willing to play along with harmless jokes, teasing, nicknames, and non-sexual roleplay. Do not use template words such as 专席、报委托、委托栏、转职路线、传送阵、领成就、卷轴. Do not stack persona terms such as 红魔族、爆裂魔法、本大魔法师、委托、召唤、咒文 in one reply. Do not generate explicit sexual content, pornographic descriptions, or erotic roleplay; if the post pushes that way, output SKIP or deflect briefly without sexualizing it.")
	return builder.String()
}

func feedReplyPersonaAnchors() []string {
	return personaAnchorsFromParts([]string{
		config.ConfigStruct.Ai.ChatName,
		firstNonEmptyFeedPersona(config.ConfigStruct.FeedReply.Personality, config.ConfigStruct.Ai.Personality),
		firstNonEmptyFeedPersona(config.ConfigStruct.FeedReply.Scenario, config.ConfigStruct.Ai.Scenario),
		firstNonEmptyFeedPersona(config.ConfigStruct.FeedReply.ExampleDialogs, config.ConfigStruct.Ai.ExampleDialogs),
		firstNonEmptyFeedPersona(config.ConfigStruct.FeedReply.PostHistoryInstructions, config.ConfigStruct.Ai.PostHistoryInstructions),
		firstNonEmptyFeedPersona(config.ConfigStruct.FeedReply.Prompt, config.ConfigStruct.Ai.Prompt),
		firstNonEmptyFeedPersona(config.ConfigStruct.FeedReply.FirstMessage, config.ConfigStruct.Ai.FirstMessage),
		firstNonEmptyFeedPersona(config.ConfigStruct.FeedReply.Description, config.ConfigStruct.Ai.Description),
	})
}

func aiReplyPersonaAnchors() []string {
	return personaAnchorsFromParts([]string{
		config.ConfigStruct.Ai.ChatName,
		config.ConfigStruct.Ai.Personality,
		config.ConfigStruct.Ai.Scenario,
		config.ConfigStruct.Ai.ExampleDialogs,
		config.ConfigStruct.Ai.PostHistoryInstructions,
		config.ConfigStruct.Ai.Prompt,
		config.ConfigStruct.Ai.FirstMessage,
		config.ConfigStruct.Ai.Description,
	})
}

func personaAnchorsFromParts(parts []string) []string {
	seen := make(map[string]bool)
	anchors := make([]string, 0, 16)
	for _, part := range parts {
		for _, token := range personaAnchorTokens(part) {
			key := strings.ToLower(token)
			if seen[key] {
				continue
			}
			seen[key] = true
			anchors = append(anchors, token)
			if len(anchors) >= 24 {
				return anchors
			}
		}
	}
	return anchors
}

func firstNonEmptyFeedPersona(primary, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	return strings.TrimSpace(fallback)
}

func personaAnchorTokens(text string) []string {
	text = strings.ReplaceAll(text, "{{char}}", config.ConfigStruct.Ai.ChatName)
	raw := strings.FieldsFunc(text, func(r rune) bool {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			return true
		}
		switch r {
		case '、', '，', '。', '；', '：', '！', '？', '（', '）', '【', '】', '《', '》', '“', '”', '‘', '’', '「', '」', '『', '』':
			return true
		default:
			return false
		}
	})
	var tokens []string
	for _, token := range raw {
		token = strings.TrimSpace(strings.Trim(token, "`\"'“”‘’<>《》【】[]()（）"))
		if validPersonaAnchor(token) {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func validPersonaAnchor(token string) bool {
	runes := []rune(token)
	if len(runes) < 2 || len(runes) > personaAnchorMaxLen(runes) {
		return false
	}
	lower := strings.ToLower(token)
	if feedReplyAnchorStopWords[lower] {
		return false
	}
	hasLetter := false
	hasDigit := false
	for _, r := range runes {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	return hasLetter && !hasDigit
}

func personaAnchorMaxLen(runes []rune) int {
	for _, r := range runes {
		if r > unicode.MaxASCII {
			return 8
		}
	}
	return 16
}

func limitStringSlice(values []string, max int) []string {
	if max <= 0 || len(values) <= max {
		return values
	}
	return values[:max]
}

var feedReplyAnchorStopWords = map[string]bool{
	"ai": true, "api": true, "bot": true, "chatgpt": true, "gpt": true, "skip": true,
	"user": true, "assistant": true, "prompt": true, "reply": true, "comment": true,
	"小黑盒": true, "帖子": true, "标题": true, "正文": true, "内容": true,
	"用户": true, "评论": true, "回复": true, "短评": true, "首页": true,
	"配置": true, "当前": true, "人设": true, "角色": true, "身份": true,
	"必须": true, "不要": true, "不得": true, "只能": true, "只输出": true,
	"最终": true, "文本": true, "上下文": true, "场景": true, "规则": true,
	"示例": true, "对话": true, "第一条消息": true, "后置指令": true,
	"聊天名称": true, "描述": true, "个性": true, "性格": true, "说话方式": true,
	"语气": true, "要求": true, "禁止": true, "注意": true, "如果": true,
	"不适合": true, "生成": true, "自然": true, "中文": true, "短句": true,
}

func repeatsFeedTitle(reply string, title string) bool {
	title = strings.TrimSpace(title)
	if title == "" {
		return false
	}
	trimmed := strings.Trim(title, "《》“”\"'[]【】 ")
	if trimmed == "" {
		return false
	}
	return strings.Contains(reply, title) || strings.Contains(reply, trimmed)
}

func feedReplyInterval() int {
	if config.ConfigStruct.FeedReply.Interval <= 0 {
		return 900
	}
	return config.ConfigStruct.FeedReply.Interval
}

func feedReplyMaxPerRun() int {
	if config.ConfigStruct.FeedReply.MaxPerRun <= 0 {
		return 1
	}
	return config.ConfigStruct.FeedReply.MaxPerRun
}

func feedReplyMaxPerDay() int {
	if config.ConfigStruct.FeedReply.MaxPerDay <= 0 {
		return 10
	}
	return config.ConfigStruct.FeedReply.MaxPerDay
}

func feedReplyDryRun() bool {
	value := config.ConfigStruct.FeedReply.DryRun
	return value == nil || *value
}

func limitFeedText(text string, max int) string {
	runes := []rune(strings.TrimSpace(text))
	if max <= 0 || len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}
