package xhh

import (
	"openxhh/db"
	"testing"
)

func TestBuildCommentRouteRequestKeepsPostContextHint(t *testing.T) {
	v := db.CommStruct{Text: "@机器人 根据文章内容生成一张图，祝楼主发财"}
	userText := NormalizeCommentText(v.Text)
	mention := ParseMentionControl(userText)

	got := buildCommentRouteRequest(v, userText, mention)
	if !got.RuleImageCandidate {
		t.Fatal("RuleImageCandidate should be true")
	}
	if !got.RuleNeedsPostContext {
		t.Fatal("RuleNeedsPostContext should be true")
	}
	if got.RuleImagePrompt != "根据帖子内容生成图片" {
		t.Fatalf("RuleImagePrompt = %q", got.RuleImagePrompt)
	}
}

func TestBuildCommentRouteRequestDoesNotBiasImageDiscussion(t *testing.T) {
	v := db.CommStruct{Text: "@机器人 生图为什么会失败"}
	userText := NormalizeCommentText(v.Text)
	mention := ParseMentionControl(userText)

	got := buildCommentRouteRequest(v, userText, mention)
	if got.RuleImageCandidate {
		t.Fatal("RuleImageCandidate should be false for image discussion")
	}
}
