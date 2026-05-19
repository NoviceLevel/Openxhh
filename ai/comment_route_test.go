package ai

import (
	"strings"
	"testing"
)

func TestParseCommentRouteContentImage(t *testing.T) {
	content := "```json\n{\"action\":\"image\",\"image_prompt\":\" 根据帖子内容生成赛博猫娘海报 \",\"mention_target\":\"\",\"needs_post_context\":true,\"needs_comment_context\":false,\"needs_image_input\":false,\"wants_similar_image\":false,\"reason\":\"明确生图\"}\n```"
	got, err := ParseCommentRouteContent(content, "小菲")
	if err != nil {
		t.Fatalf("ParseCommentRouteContent returned error: %v", err)
	}
	if got.Action != CommentRouteActionImage {
		t.Fatalf("Action = %q, want image", got.Action)
	}
	if got.ImagePrompt != "根据帖子内容生成赛博猫娘海报" {
		t.Fatalf("ImagePrompt = %q", got.ImagePrompt)
	}
	if got.MentionTarget != "小菲" {
		t.Fatalf("MentionTarget = %q", got.MentionTarget)
	}
	if !got.NeedsPostContext {
		t.Fatal("NeedsPostContext should be true")
	}
}

func TestParseCommentRouteContentDefaultsUnknownToReply(t *testing.T) {
	got, err := ParseCommentRouteContent(`{"action":"unknown","image_prompt":"","reason":"无法判断"}`, "")
	if err != nil {
		t.Fatalf("ParseCommentRouteContent returned error: %v", err)
	}
	if got.Action != CommentRouteActionReply {
		t.Fatalf("Action = %q, want reply", got.Action)
	}
}

func TestParseCommentRouteContentNormalizesChineseAction(t *testing.T) {
	got, err := ParseCommentRouteContent(`{"action":"看图生图","image_prompt":"参考图片生成头像","needs_image_input":true}`, "")
	if err != nil {
		t.Fatalf("ParseCommentRouteContent returned error: %v", err)
	}
	if got.Action != CommentRouteActionImage {
		t.Fatalf("Action = %q, want image", got.Action)
	}
	if !got.NeedsImageInput {
		t.Fatal("NeedsImageInput should be true")
	}
}

func TestApplyCommentRouteRuleHintsFillsContextPrompt(t *testing.T) {
	got := applyCommentRouteRuleHints(CommentRouteResult{Action: CommentRouteActionImage}, CommentRouteRequest{RuleNeedsPostContext: true})
	if got.ImagePrompt != "根据帖子内容生成图片" {
		t.Fatalf("ImagePrompt = %q", got.ImagePrompt)
	}
	if !got.NeedsPostContext {
		t.Fatal("NeedsPostContext should be true")
	}
}

func TestCommentRouteSystemPromptPreservesContextSubject(t *testing.T) {
	prompt := commentRouteSystemPrompt()
	for _, want := range []string{"不要凭空编造主体", "不要覆盖上下文主体", "后续 prompt refine 会填入细节"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("commentRouteSystemPrompt should contain %q", want)
		}
	}
}
