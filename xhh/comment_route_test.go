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

func TestResolveRouteMentionTargetKeepsRuleTarget(t *testing.T) {
	got := resolveRouteMentionTarget("小猫娘喵喵", "麻溜转我五块", "@小猫娘喵喵 生成一只猫，并艾特麻溜转我五块查看")
	if got != "麻溜转我五块" {
		t.Fatalf("resolveRouteMentionTarget = %q, want 麻溜转我五块", got)
	}
}

func TestResolveRouteMentionTargetDropsWakeMention(t *testing.T) {
	text := `<a data-user-id="93872966" href="https://api.xiaoheihe.cn/open_inapp/#heybox://%7B%22protocol_type%22%3A%22openUser%22%2C%22user_id%22%3A%2293872966%22%7D" target="_blank">@小猫娘喵喵</a>要艾特麻溜转我五块查看`
	got := resolveRouteMentionTarget("小猫娘喵喵", "", text)
	if got != "" {
		t.Fatalf("resolveRouteMentionTarget = %q, want empty", got)
	}
}
