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
	return "请基于这篇小黑盒帖子写一条公开评论。如果不适合回复，请只输出 SKIP。标题：" + link.Title + "\n正文摘要：" + link.Description
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
	if issue := basicReplySendIssue(reply, true); issue != "" {
		return issue
	}
	if shouldSkipFeedReply(reply) {
		return ""
	}
	if len([]rune(strings.TrimSpace(reply))) > maxFeedReplyNaturalRunes {
		return "刷帖回复过长"
	}
	if sentenceCount(reply) > maxFeedReplyNaturalSentences {
		return "刷帖回复句子过多"
	}
	for _, sentence := range splitReplySentences(reply) {
		if len([]rune(strings.TrimSpace(sentence))) > maxFeedReplyNaturalSentenceRunes {
			return "刷帖回复单句过长"
		}
	}
	return ""
}

func aiReplyQualityIssue(reply string) string {
	return basicReplySendIssue(reply, false)
}

func aiReplyQualityIssueForQuestion(reply string, questionText string) string {
	return aiReplyQualityIssue(reply)
}

func basicReplySendIssue(reply string, allowSkip bool) string {
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
	return ""
}

func sentenceCount(text string) int {
	return len(splitReplySentences(text))
}

func splitReplySentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	sentences := make([]string, 0, 2)
	start := 0
	for i, r := range text {
		if strings.ContainsRune("。！？!?\n", r) {
			addReplySentence(&sentences, text[start:i])
			start = i + len(string(r))
		}
	}
	addReplySentence(&sentences, text[start:])
	return sentences
}

func addReplySentence(sentences *[]string, sentence string) {
	sentence = strings.TrimSpace(sentence)
	if sentence != "" {
		*sentences = append(*sentences, sentence)
	}
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
