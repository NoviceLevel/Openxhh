package xhh

import (
	"openxhh/ai"
	"openxhh/config"
	"strings"
	"testing"
)

func TestBuildReplyImagePromptIncludesReplyContext(t *testing.T) {
	oldConfig := config.ConfigStruct
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
	})

	config.ConfigStruct.Ai.ChatName = "Megumin"
	config.ConfigStruct.Ai.Description = "Crimson Demon arch wizard"
	config.ConfigStruct.Ai.Personality = "dramatic and proud"
	config.ConfigStruct.Ai.Prompt = "Only output final comment text"

	prompt := BuildReplyImagePrompt(
		[]ai.Content{
			{Type: "text", Text: "<b>Title</b>: hard boss fight"},
			{Type: "image_url"},
		},
		"How do I beat this boss?",
		"Explosion solves everything.",
	)

	for _, want := range []string{
		"Megumin",
		"Crimson Demon arch wizard",
		"hard boss fight",
		"How do I beat this boss?",
		"Explosion solves everything.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("BuildReplyImagePrompt missing %q in %q", want, prompt)
		}
	}
	if strings.Contains(prompt, "<b>") {
		t.Fatalf("BuildReplyImagePrompt should strip HTML tags: %q", prompt)
	}
	if strings.Contains(prompt, "Only output final comment text") {
		t.Fatalf("BuildReplyImagePrompt should not include text-only reply protocol: %q", prompt)
	}
}

func TestLimitReplyImageText(t *testing.T) {
	got := limitReplyImageText("abcdef", 3)
	if got != "abc" {
		t.Fatalf("limitReplyImageText = %q, want abc", got)
	}
}
