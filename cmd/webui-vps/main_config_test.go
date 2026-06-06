package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestApplyConfigDefaultsMigratesCharacterCard(t *testing.T) {
	var cfg appConfig
	cfg.AI.CharacterCard = "legacy persona"

	if !applyConfigDefaults(&cfg) {
		t.Fatal("applyConfigDefaults returned false, want migration change")
	}
	if cfg.AI.Description != "legacy persona" {
		t.Fatalf("AI.Description = %q, want migrated character card", cfg.AI.Description)
	}
	if cfg.AI.CharacterCard != "" {
		t.Fatalf("AI.CharacterCard = %q, want cleared legacy field", cfg.AI.CharacterCard)
	}
}

func TestApplyConfigDefaultsDoesNotOverwriteDescriptionWithCharacterCard(t *testing.T) {
	var cfg appConfig
	cfg.AI.Description = "new persona"
	cfg.AI.CharacterCard = "legacy persona"

	applyConfigDefaults(&cfg)
	if cfg.AI.Description != "new persona" {
		t.Fatalf("AI.Description = %q, want existing description", cfg.AI.Description)
	}
	if cfg.AI.CharacterCard != "" {
		t.Fatalf("AI.CharacterCard = %q, want cleared legacy field", cfg.AI.CharacterCard)
	}
}

func TestApplyConfigDefaultsDoesNotSetImageResponseFormat(t *testing.T) {
	var cfg appConfig

	applyConfigDefaults(&cfg)
	if cfg.Image.ResponseFormat != "" {
		t.Fatalf("Image.ResponseFormat = %q, want empty", cfg.Image.ResponseFormat)
	}
}

func TestBuildConfigTestAIBodyResponsesIncludesSearchTool(t *testing.T) {
	var cfg appConfig
	cfg.AI.Model = "test-model"
	cfg.AI.ChatName = "惠惠"
	cfg.AI.Description = "红魔族大魔法师"
	cfg.AI.WebSearch = boolPtr(true)
	cfg.AI.ForceWebSearch = boolPtr(true)
	cfg.AI.SearchContextSize = "high"

	body, err := buildConfigTestAIBody(cfg, "测试一下", true)
	if err != nil {
		t.Fatalf("buildConfigTestAIBody returned error: %v", err)
	}
	var got responsesTestBody
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Model != "test-model" {
		t.Fatalf("Model = %q", got.Model)
	}
	if got.Instructions == "" {
		t.Fatal("Instructions should include test system prompt")
	}
	if len(got.Tools) != 1 || got.Tools[0].Type != "web_search_preview" || got.Tools[0].SearchContextSize != "high" {
		t.Fatalf("Tools = %+v", got.Tools)
	}
	if got.ToolChoice != "required" {
		t.Fatalf("ToolChoice = %q, want required", got.ToolChoice)
	}
}

func TestConfigTestOutputDirUsesRootForRelativePath(t *testing.T) {
	root := filepath.Join("C:", "Openxhh")
	got := configTestOutputDir("images", root)
	want := filepath.Join(root, "images")
	if got != want {
		t.Fatalf("configTestOutputDir = %q, want %q", got, want)
	}
}

func TestParseConfigTestAIResponseAcceptsSSE(t *testing.T) {
	body := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"model\"}}],\"usage\":{\"total_tokens\":1}}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" ok\"}}],\"usage\":{\"total_tokens\":2}}\n\n" +
		"data: [DONE]\n\n")
	text, tokens, err := parseConfigTestAIResponse(body, false)
	if err != nil {
		t.Fatalf("parseConfigTestAIResponse returned error: %v", err)
	}
	if text != "model ok" {
		t.Fatalf("text = %q, want model ok", text)
	}
	if tokens != 2 {
		t.Fatalf("tokens = %d, want 2", tokens)
	}
}
