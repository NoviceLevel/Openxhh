package xhh

import (
	"database/sql"
	"encoding/json"
	"openxhh/config"
	"openxhh/db"
	"openxhh/loger"
	"openxhh/sqlite"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestMsgUnmarshalAtPost(t *testing.T) {
	data := []byte(`{
		"message_id": 1001,
		"message_type": 16,
		"user_a": {"userid": "89055874", "username": "小猫娘喵喵"},
		"link": {"linkid": 181099114, "description": "@机器人 生成一张机械朋克猫"}
	}`)

	var msg Msg
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if !msg.IsPost {
		t.Fatal("IsPost should be true")
	}
	if msg.CommentID != -1 || msg.RootCommentID != -1 {
		t.Fatalf("comment ids = (%d,%d), want (-1,-1)", msg.CommentID, msg.RootCommentID)
	}
	if msg.LinkID != 181099114 {
		t.Fatalf("LinkID = %d", msg.LinkID)
	}
	if msg.UserID != 89055874 {
		t.Fatalf("UserID = %d", msg.UserID)
	}
	if msg.UserName != "小猫娘喵喵" {
		t.Fatalf("UserName = %q", msg.UserName)
	}
	if msg.CommentText != "@机器人 生成一张机械朋克猫" {
		t.Fatalf("CommentText = %q", msg.CommentText)
	}
}

func TestMsgUnmarshalAtComment(t *testing.T) {
	data := []byte(`{
		"message_id": 1002,
		"message_type": 17,
		"userid_a": 89055874,
		"comment_a_id": 867937626,
		"root_comment_id": 867937000,
		"linkid": 181099114,
		"comment_a_text": "@机器人 生图 一只猫"
	}`)

	var msg Msg
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if msg.IsPost {
		t.Fatal("IsPost should be false")
	}
	if msg.CommentID != 867937626 || msg.RootCommentID != 867937000 {
		t.Fatalf("comment ids = (%d,%d)", msg.CommentID, msg.RootCommentID)
	}
	if msg.LinkID != 181099114 || msg.UserID != 89055874 {
		t.Fatalf("LinkID/UserID = %d/%d", msg.LinkID, msg.UserID)
	}
}

func TestMsgUnmarshalNotificationReplyFields(t *testing.T) {
	data := []byte(`{
		"message_id": 1003,
		"userid_a": 89055874,
		"user_a": {"username": "路人"},
		"comment_id": 867937627,
		"root_comment_id": 867937000,
		"reply_id": "555",
		"userid_b": "999",
		"linkid": 181099114,
		"text": "楼里继续问一下"
	}`)

	var msg Msg
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if msg.CommentID != 867937627 || msg.ReplyCommentID != 555 || msg.ReplyUserID != 999 {
		t.Fatalf("comment/reply ids = (%d,%d,%d), want (867937627,555,999)", msg.CommentID, msg.ReplyCommentID, msg.ReplyUserID)
	}
	if msg.CommentText != "楼里继续问一下" || msg.UserName != "路人" {
		t.Fatalf("text/user = %q/%q", msg.CommentText, msg.UserName)
	}
}

func TestReplyToBotNotificationQueuesReply(t *testing.T) {
	setupXHHMessageQueueTest(t)
	if !db.SaveOutboundMessage(db.OutboundMessage{Source: "ai_reply", LinkID: 181099114, RootCommentID: 867937000, ReplyCommentID: 867936999, CommentID: 555, Text: "机器人刚才的回复", CreatedAt: 100}) {
		t.Fatal("SaveOutboundMessage returned false")
	}
	msg := Msg{
		MsgID:          1004,
		CommentID:      867937628,
		RootCommentID:  0,
		ReplyCommentID: 555,
		LinkID:         181099114,
		UserID:         89055874,
		UserName:       "路人",
		CommentText:    "不是 @，但回复了机器人",
		CreatedAt:      1000,
	}

	if notificationSource(msg) != "reply_to_bot" {
		t.Fatalf("notificationSource = %q, want reply_to_bot", notificationSource(msg))
	}
	if got := queueReplyToBotNotification(msg); got != replyNotificationQueueQueued {
		t.Fatalf("queue status = %v, want queued", got)
	}

	var count int
	var text string
	var reply bool
	var rootID int
	if err := sqlite.Db.QueryRow("SELECT COUNT(*), COALESCE(MAX(comment_text), ''), COALESCE(MAX(reply), false), COALESCE(MAX(comment_root_id), 0) FROM at WHERE msg_id=?", 1004).Scan(&count, &text, &reply, &rootID); err != nil {
		t.Fatalf("query at: %v", err)
	}
	if count != 1 || text != "不是 @，但回复了机器人" || reply || rootID != 867937000 {
		t.Fatalf("queued row = (%d,%q,%v,%d), want (1,不是 @，但回复了机器人,false,867937000)", count, text, reply, rootID)
	}

	if got := queueReplyToBotNotification(msg); got != replyNotificationQueueIgnored {
		t.Fatalf("duplicate queue status = %v, want ignored", got)
	}
}

func TestReplyToCurrentAccountNotificationQueuesReplyWithoutOutboundRecord(t *testing.T) {
	setupXHHMessageQueueTest(t)
	msg := Msg{
		MsgID:          1005,
		CommentID:      867937629,
		RootCommentID:  867937000,
		ReplyCommentID: 555,
		ReplyUserID:    999,
		LinkID:         181099114,
		UserID:         89055874,
		UserName:       "路人",
		CommentText:    "别人帖子里回复了当前账号",
		CreatedAt:      1000,
	}

	if notificationSource(msg) != "reply_to_bot" {
		t.Fatalf("notificationSource = %q, want reply_to_bot", notificationSource(msg))
	}
	if got := queueReplyToBotNotification(msg); got != replyNotificationQueueQueued {
		t.Fatalf("queue status = %v, want queued", got)
	}

	var count int
	var text string
	var reply bool
	var rootID int
	if err := sqlite.Db.QueryRow("SELECT COUNT(*), COALESCE(MAX(comment_text), ''), COALESCE(MAX(reply), false), COALESCE(MAX(comment_root_id), 0) FROM at WHERE msg_id=?", 1005).Scan(&count, &text, &reply, &rootID); err != nil {
		t.Fatalf("query at: %v", err)
	}
	if count != 1 || text != "别人帖子里回复了当前账号" || reply || rootID != 867937000 {
		t.Fatalf("queued row = (%d,%q,%v,%d), want (1,别人帖子里回复了当前账号,false,867937000)", count, text, reply, rootID)
	}
}

func TestOldReplyToCurrentAccountNotificationDoesNotQueue(t *testing.T) {
	setupXHHMessageQueueTest(t)
	msg := Msg{
		MsgID:          1006,
		CommentID:      867937630,
		RootCommentID:  867937000,
		ReplyCommentID: 555,
		ReplyUserID:    999,
		LinkID:         181099114,
		UserID:         89055874,
		UserName:       "路人",
		CommentText:    "很久之前回复了当前账号",
		CreatedAt:      999,
	}

	if notificationSource(msg) != "reply_to_bot" {
		t.Fatalf("notificationSource = %q, want reply_to_bot", notificationSource(msg))
	}
	if got := queueReplyToBotNotification(msg); got != replyNotificationQueueSkippedOld {
		t.Fatalf("queue status = %v, want skipped old", got)
	}

	var count int
	if err := sqlite.Db.QueryRow("SELECT COUNT(*) FROM at WHERE msg_id=?", 1006).Scan(&count); err != nil {
		t.Fatalf("query at: %v", err)
	}
	if count != 0 {
		t.Fatalf("queued old notification count = %d, want 0", count)
	}
}

func setupXHHMessageQueueTest(t *testing.T) {
	t.Helper()
	oldType := config.ConfigStruct.DataBase.Type
	oldDB := sqlite.Db
	oldInfo := Info
	oldOwner := config.ConfigStruct.Xhh.Owner
	oldEnableWhitelist := config.ConfigStruct.Xhh.EnableWhitelist
	oldOwners := append([]int(nil), Owners...)
	oldOwnerIDsLoaded := ownerIDsLoaded
	oldMaxPendingReplies := MaxPendingReplies
	oldMaxPendingRepliesPerUser := MaxPendingRepliesPerUser
	oldNotificationReplyQueueStartUnix := notificationReplyQueueStartUnix
	oldOldReplyNotificationSkipLogged := oldReplyNotificationSkipLogged
	oldLogger := loger.Loger
	loger.Loger = zap.NewNop()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlite.Db = database
	config.ConfigStruct.DataBase.Type = "sqlite"
	config.ConfigStruct.Xhh.EnableWhitelist = false
	config.ConfigStruct.Xhh.Owner = ""
	Owners = nil
	ownerIDsLoaded = false
	Info.HeyBoxId = "999"
	MaxPendingReplies = defaultMaxPendingReplies
	MaxPendingRepliesPerUser = defaultMaxPendingRepliesPerUser
	notificationReplyQueueStartUnix = 1000
	oldReplyNotificationSkipLogged = false
	t.Cleanup(func() {
		database.Close()
		sqlite.Db = oldDB
		config.ConfigStruct.DataBase.Type = oldType
		config.ConfigStruct.Xhh.Owner = oldOwner
		config.ConfigStruct.Xhh.EnableWhitelist = oldEnableWhitelist
		Owners = oldOwners
		ownerIDsLoaded = oldOwnerIDsLoaded
		Info = oldInfo
		MaxPendingReplies = oldMaxPendingReplies
		MaxPendingRepliesPerUser = oldMaxPendingRepliesPerUser
		notificationReplyQueueStartUnix = oldNotificationReplyQueueStartUnix
		oldReplyNotificationSkipLogged = oldOldReplyNotificationSkipLogged
		loger.Loger = oldLogger
	})
	_, err = sqlite.Db.Exec(`CREATE TABLE at (
		msg_id BIGINT PRIMARY KEY,
		comment_a_id BIGINT,
		comment_root_id BIGINT,
		link_id BIGINT,
		user_a_id BIGINT,
		user_a_name TEXT DEFAULT '',
		comment_text TEXT,
		reply boolean
	)`)
	if err != nil {
		t.Fatalf("create at table: %v", err)
	}
	_, err = sqlite.Db.Exec(`CREATE TABLE outbound_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source TEXT DEFAULT '',
		link_id BIGINT DEFAULT 0,
		root_comment_id BIGINT DEFAULT 0,
		reply_comment_id BIGINT DEFAULT 0,
		comment_id BIGINT DEFAULT 0,
		text TEXT DEFAULT '',
		image_url TEXT DEFAULT '',
		created_at BIGINT DEFAULT 0,
		raw_response TEXT DEFAULT '',
		unique_key TEXT UNIQUE
	)`)
	if err != nil {
		t.Fatalf("create outbound_messages table: %v", err)
	}
}

func TestShouldMentionTarget(t *testing.T) {
	if ShouldMentionTarget("告诉我这个是什么意思") {
		t.Fatal("ShouldMentionTarget should not trigger for 告诉我")
	}
	if ShouldMentionTarget("其他方案呢") {
		t.Fatal("ShouldMentionTarget should not trigger for 其他")
	}
	if !ShouldMentionTarget("反驳他") {
		t.Fatal("ShouldMentionTarget should trigger for 反驳他")
	}
	if !ShouldMentionTarget("要艾特她啦") {
		t.Fatal("ShouldMentionTarget should trigger for 艾特她")
	}
}

func TestParseMentionControlReferenceTarget(t *testing.T) {
	got := ParseMentionControl(`<a data-user-id="93872966" href="https://api.xiaoheihe.cn/open_inapp/#heybox://%7B%22protocol_type%22%3A%22openUser%22%2C%22user_id%22%3A%2293872966%22%7D" target="_blank">@小猫娘喵喵</a>要艾特她啦`)
	if got.TargetText != "她" {
		t.Fatalf("TargetText = %q, want 她", got.TargetText)
	}
	if got.CleanedText != "要" {
		t.Fatalf("CleanedText = %q, want 要", got.CleanedText)
	}
	if got.SemanticText != "要艾特她啦" {
		t.Fatalf("SemanticText = %q, want 要艾特她啦", got.SemanticText)
	}
	if mentionQuestionText(got) != "要艾特她啦" {
		t.Fatalf("mentionQuestionText = %q, want 要艾特她啦", mentionQuestionText(got))
	}
}

func TestParseMentionControlWakeOnly(t *testing.T) {
	got := ParseMentionControl(`<a data-user-id="93872966" href="https://api.xiaoheihe.cn/open_inapp/#heybox://%7B%22protocol_type%22%3A%22openUser%22%2C%22user_id%22%3A%2293872966%22%7D" target="_blank">@小猫娘喵喵</a>`)
	if !got.WakeOnly {
		t.Fatal("WakeOnly should be true")
	}
	if got.CleanedText != "" || got.SemanticText != "" {
		t.Fatalf("CleanedText/SemanticText = %q/%q, want empty", got.CleanedText, got.SemanticText)
	}
	if mentionQuestionText(got) == "" {
		t.Fatal("mentionQuestionText should provide an empty-mention prompt")
	}
}

func TestParseCommentCreateResponseCreatedAt(t *testing.T) {
	status, msg, commentID, createdAt := parseCommentCreateResponse([]byte(`{"status":"ok","msg":"","result":{"commentid":123,"created_at":1716350000000}}`))
	if status != "ok" || msg != "" || commentID != 123 || createdAt != 1716350000 {
		t.Fatalf("parseCommentCreateResponse = (%q,%q,%d,%d)", status, msg, commentID, createdAt)
	}
}

func TestReadableXHHResponseBodyDecodesUnicodeEscapes(t *testing.T) {
	got := readableXHHResponseBody(`{"status":"failed","msg":"\u60a8\u6700\u8fd1\u7684\u8bc4\u8bba\u9891\u6b21\u5f02\u5e38\uff0c\u8bf7\u7a0d\u540e\u518d\u8bd5","version":"1.0","result":{}}`)
	if want := "您最近的评论频次异常，请稍后再试"; !strings.Contains(got, want) {
		t.Fatalf("readableXHHResponseBody = %q, want it to contain %q", got, want)
	}
	if strings.Contains(got, `\u60a8`) {
		t.Fatalf("readableXHHResponseBody still contains unicode escape: %q", got)
	}
}

func TestExtractExplicitMentionTargetConversationCommands(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{text: "@机器人 向小明打个招呼", want: "小明"},
		{text: "@机器人 和小明说说", want: "小明"},
		{text: "@机器人 跟小红聊两句", want: "小红"},
		{text: "@机器人 对@阿伟说一下", want: "阿伟"},
		{text: "@机器人 咬小明一口", want: "小明"},
		{text: "@机器人 反驳小明的观点", want: "小明"},
		{text: "@机器人 问问小周怎么看", want: "小周"},
		{text: "@机器人 生成一张小菲的画像，并艾特小菲来看[cube_喜欢]", want: "小菲"},
		{text: "@机器人 生图 一只猫，顺便艾特小明看看[cube_点赞]", want: "小明"},
		{text: "@机器人 并给他看@小猫娘喵喵", want: "小猫娘喵喵"},
		{text: "@机器人 生成一张图，并艾特小明看@小猫娘喵喵", want: "小明"},
		{text: "@机器人 生成一只黑丝冷白皮嫌弃颜奶龙，并艾特麻溜转我五块查看", want: "麻溜转我五块"},
		{text: `<a data-user-id="93872966" href="https://api.xiaoheihe.cn/open_inapp/#heybox://%7B%22protocol_type%22%3A%22openUser%22%2C%22user_id%22%3A%2293872966%22%7D" target="_blank">@小猫娘喵喵</a>要艾特麻溜转我五块查看`, want: "麻溜转我五块"},
		{text: "@机器人 要艾特她啦", want: ""},
		{text: "@机器人 告诉我这个是什么意思", want: ""},
		{text: "@机器人 反驳他", want: ""},
	}

	for _, tt := range cases {
		if got := extractExplicitMentionTarget(tt.text); got != tt.want {
			t.Fatalf("extractExplicitMentionTarget(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}
