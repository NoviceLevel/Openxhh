package ai

import "testing"

func TestParseImagePromptRefineContent(t *testing.T) {
	content := "```json\n{\"image_prompt\":\" 一只机械朋克猫 \" , \"mention_target\":\" 小菲 \", \"needs_post_context\":true, \"needs_comment_context\":false, \"needs_image_input\":false}\n```"

	got, err := ParseImagePromptRefineContent(content)
	if err != nil {
		t.Fatalf("ParseImagePromptRefineContent returned error: %v", err)
	}
	if got.ImagePrompt != "一只机械朋克猫" {
		t.Fatalf("ImagePrompt = %q", got.ImagePrompt)
	}
	if got.MentionTarget != "小菲" {
		t.Fatalf("MentionTarget = %q", got.MentionTarget)
	}
	if !got.NeedsPostContext {
		t.Fatal("NeedsPostContext should be true")
	}
}

func TestParseImagePromptRefineContentEmptyPrompt(t *testing.T) {
	_, err := ParseImagePromptRefineContent(`{"image_prompt":""}`)
	if err == nil {
		t.Fatal("expected error for empty image_prompt")
	}
}

func TestLimitRefineRunes(t *testing.T) {
	got := limitRefineRunes("猫猫猫", 2)
	if got != "猫猫" {
		t.Fatalf("limitRefineRunes = %q", got)
	}
}
