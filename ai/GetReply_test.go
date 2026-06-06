package ai

import (
	"openxhh/config"
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

func TestBuildReplySystemPromptDoesNotInjectDefaultCharacter(t *testing.T) {
	got := buildReplySystemPrompt("")
	if got != "" {
		t.Fatalf("buildReplySystemPrompt should not inject default character; got %q", got)
	}
}

func TestBuildTavernPromptSkipsEmptySections(t *testing.T) {
	got := buildTavernPrompt("惠惠", "描述", "个性", "", "第一条", "示例", "场景规则", "")
	for _, want := range []string{"【聊天名称】\n惠惠", "【描述】\n描述", "【个性】\n个性", "【第一条消息】\n第一条", "【示例对话】\n示例", "【场景规则】\n场景规则"} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildTavernPrompt should contain %q; got %q", want, got)
		}
	}
	for _, unwanted := range []string{"【场景】", "【后置指令】"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("buildTavernPrompt should skip %q; got %q", unwanted, got)
		}
	}
}

func TestBuildTavernPromptReturnsSceneOnlyForExistingPrompt(t *testing.T) {
	got := buildTavernPrompt("", "", "", "", "", "", "只用用户 Prompt", "")
	if got != "只用用户 Prompt" {
		t.Fatalf("buildTavernPrompt = %q, want scene prompt only", got)
	}
}

func TestFeedReplyPromptFromConfigUsesFeedSpecificFields(t *testing.T) {
	oldConfig := config.ConfigStruct
	t.Cleanup(func() { config.ConfigStruct = oldConfig })

	config.ConfigStruct.Ai.ChatName = "惠惠"
	config.ConfigStruct.Ai.Description = "公共描述"
	config.ConfigStruct.Ai.Personality = "公共个性"
	config.ConfigStruct.FeedReply.Description = "刷帖描述"
	config.ConfigStruct.FeedReply.Personality = "刷帖个性"

	got := FeedReplyPromptFromConfig("刷帖规则")
	for _, want := range []string{"【聊天名称】\n惠惠", "【描述】\n刷帖描述", "【个性】\n刷帖个性", "【场景规则】\n刷帖规则"} {
		if !strings.Contains(got, want) {
			t.Fatalf("FeedReplyPromptFromConfig should contain %q; got %q", want, got)
		}
	}
	if strings.Contains(got, "公共描述") || strings.Contains(got, "公共个性") {
		t.Fatalf("FeedReplyPromptFromConfig should prefer feed-specific fields; got %q", got)
	}
}

func TestBuildReplyScenePromptFramesCommentReply(t *testing.T) {
	got := buildReplyScenePrompt("@小猫娘 这个配置怎么改")
	for _, want := range []string{"小黑盒帖子和评论楼层", "对方刚刚完整说的是", "机器人 @ 只是叫你出来"} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildReplyScenePrompt should contain %q; got %q", want, got)
		}
	}
	if strings.Contains(got, "像评论区里的这个角色") {
		t.Fatalf("buildReplyScenePrompt should not force a role framing: %q", got)
	}
	if strings.Contains(got, "角色、语气和规则") {
		t.Fatalf("buildReplyScenePrompt should not add style instructions: %q", got)
	}
	if strings.Contains(got, "请结合整条评论理解用户意图") {
		t.Fatalf("buildReplyScenePrompt still uses task-style wording: %q", got)
	}
}

func TestBuildFeedReplyScenePromptFramesPostComment(t *testing.T) {
	got := buildFeedReplyScenePrompt("标题：测试帖子")
	for _, want := range []string{"小黑盒首页帖子内容", "标题：测试帖子"} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildFeedReplyScenePrompt should contain %q; got %q", want, got)
		}
	}
	if strings.Contains(got, "对方刚刚完整说的是") || strings.Contains(got, "机器人 @") {
		t.Fatalf("buildFeedReplyScenePrompt should not use mention-reply framing: %q", got)
	}
	if strings.Contains(got, "角色、语气和规则") {
		t.Fatalf("buildFeedReplyScenePrompt should not add style instructions: %q", got)
	}
}
