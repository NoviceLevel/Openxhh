package ai

import (
	"openxhh/config"
	"strings"
	"testing"
)

func TestBuildReplySystemPromptKeepsCustomPromptAndAddsHumanPresence(t *testing.T) {
	got := buildReplySystemPrompt("你是测试角色。")
	for _, want := range []string{"你是测试角色。", "真人感与情绪规则", "先接住对方具体说的话", "不要主动说“作为 AI / 机器人 / 模型”"} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildReplySystemPrompt should contain %q; got %q", want, got)
		}
	}
	if strings.Contains(got, "回复协议") {
		t.Fatalf("buildReplySystemPrompt should not append old forced protocol wording: %q", got)
	}
}

func TestBuildReplySystemPromptAddsDefaultHumanPresenceWithoutCharacter(t *testing.T) {
	got := buildReplySystemPrompt("")
	for _, want := range []string{"真人感与情绪规则", "默认保持温和、有主见、有一点生活感", "只输出最终要发到评论区的回复文本"} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildReplySystemPrompt should contain %q; got %q", want, got)
		}
	}
}

func TestBuildReplySystemPromptAddsNaturalInteractionGuardrails(t *testing.T) {
	got := buildReplySystemPrompt("persona")
	for _, want := range []string{
		"Natural interaction guardrails",
		"Do not immediately translate every message into character lore",
		"Use at most one obvious persona term",
		`only says things like "喵"`,
		`Do not scold them to "speak human language"`,
		"Stage directions are optional seasoning",
		"Prefer concrete callbacks",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildReplySystemPrompt missing %q in %q", want, got)
		}
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
	for _, unwanted := range []string{"普通路过网友", "角色味只轻轻露出"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("buildFeedReplyScenePrompt should not force old feed style %q: %q", unwanted, got)
		}
	}
}

func TestBuildFeedReplyScenePromptDefaultUsesTavernStyle(t *testing.T) {
	got := buildFeedReplyScenePrompt("")
	for _, want := range []string{"普通回复一样的酒馆人设", "自然接话", "轻微情绪和角色反应", "不要每条都用动作描写开场", "SKIP"} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildFeedReplyScenePrompt default missing %q in %q", want, got)
		}
	}
	for _, unwanted := range []string{"短评论", "普通路过网友", "角色味只轻轻露出"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("buildFeedReplyScenePrompt default should not contain old feed wording %q: %q", unwanted, got)
		}
	}
}
