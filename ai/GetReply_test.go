package ai

import (
	"strings"
	"testing"
)

func TestBuildReplySystemPromptUsesCustomPromptVerbatim(t *testing.T) {
	got := buildReplySystemPrompt("你是测试角色。")
	if got != "你是测试角色。" {
		t.Fatalf("buildReplySystemPrompt = %q, want custom prompt verbatim", got)
	}
	if strings.Contains(got, "回复协议") {
		t.Fatalf("buildReplySystemPrompt should not append forced protocol to custom prompt: %q", got)
	}
}

func TestBuildReplySystemPromptUsesDefaultCharacter(t *testing.T) {
	got := buildReplySystemPrompt("")
	if !strings.Contains(got, "小猫娘喵喵") || !strings.Contains(got, "回复协议") {
		t.Fatalf("buildReplySystemPrompt default = %q", got)
	}
}

func TestBuildReplyScenePromptFramesCommentReply(t *testing.T) {
	got := buildReplyScenePrompt("@小猫娘 这个配置怎么改")
	for _, want := range []string{"小黑盒帖子和评论楼层", "对方刚刚完整说的是", "按系统提示中的角色、语气和规则"} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildReplyScenePrompt should contain %q; got %q", want, got)
		}
	}
	if strings.Contains(got, "像评论区里的这个角色") {
		t.Fatalf("buildReplyScenePrompt should not force a role framing: %q", got)
	}
	if strings.Contains(got, "请结合整条评论理解用户意图") {
		t.Fatalf("buildReplyScenePrompt still uses task-style wording: %q", got)
	}
}
