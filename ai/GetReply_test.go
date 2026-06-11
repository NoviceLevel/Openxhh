package ai

import (
	"openxhh/config"
	"strings"
	"testing"
)

func TestBuildReplySystemPromptUsesOnlyConfiguredPrompt(t *testing.T) {
	got := buildReplySystemPrompt("  persona from config  ")
	if got != "persona from config" {
		t.Fatalf("buildReplySystemPrompt = %q, want configured prompt only", got)
	}
	for _, unwanted := range []string{
		"Natural interaction guardrails",
		"真人感与情绪规则",
		"Every reply needs a Megumin-like reaction",
		"Default to 1-2 sentences",
		"Do not use template lore-shell words",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("buildReplySystemPrompt should not append %q: %q", unwanted, got)
		}
	}
}

func TestBuildReplySystemPromptAllowsEmptyPrompt(t *testing.T) {
	if got := buildReplySystemPrompt("   "); got != "" {
		t.Fatalf("buildReplySystemPrompt empty = %q, want empty", got)
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

func TestFeedReplyPromptFromConfigFallsBackToAIStylePrompt(t *testing.T) {
	oldConfig := config.ConfigStruct
	t.Cleanup(func() { config.ConfigStruct = oldConfig })

	config.ConfigStruct.Ai.ChatName = "惠惠"
	config.ConfigStruct.Ai.Prompt = "普通回复场景"
	config.ConfigStruct.FeedReply.Prompt = ""

	got := FeedReplyPromptFromConfig("")
	if !strings.Contains(got, "【场景规则】\n普通回复场景") {
		t.Fatalf("FeedReplyPromptFromConfig should fall back to AI prompt; got %q", got)
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
	for _, unwanted := range []string{"普通路过网友", "角色味只轻轻露出"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("buildFeedReplyScenePrompt should not force old feed style %q: %q", unwanted, got)
		}
	}
}

func TestBuildFeedReplyScenePromptDefaultOnlyFramesTask(t *testing.T) {
	got := buildFeedReplyScenePrompt("")
	for _, want := range []string{"小黑盒首页帖子内容", "请基于这篇小黑盒帖子写一条公开评论", "SKIP"} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildFeedReplyScenePrompt default missing %q in %q", want, got)
		}
	}
	for _, unwanted := range []string{"惠惠", "普通评论员", "惠惠式反应", "红魔族式夸张", "专席", "报委托", "传送阵", "卷轴", "普通短评默认1-2句"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("buildFeedReplyScenePrompt default should not add style constraint %q: %q", unwanted, got)
		}
	}
}
