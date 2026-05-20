package xhh

import (
	"encoding/json"
	"testing"
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
