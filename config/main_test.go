package config

import "testing"

func TestCreateDefaultConfigReturnsWriteError(t *testing.T) {
	oldConfig := ConfigStruct
	t.Cleanup(func() {
		ConfigStruct = oldConfig
	})

	if err := createDefaultConfig(t.TempDir()); err == nil {
		t.Fatal("createDefaultConfig returned nil error for directory path")
	}
}

func TestApplyDefaultsDoesNotSetPrompt(t *testing.T) {
	oldConfig := ConfigStruct
	ConfigStruct.Ai.Prompt = ""
	ConfigStruct.FeedReply.Prompt = ""
	t.Cleanup(func() {
		ConfigStruct = oldConfig
	})

	applyDefaults()
	if ConfigStruct.Ai.Prompt != "" {
		t.Fatalf("Ai.Prompt = %q, want empty prompt", ConfigStruct.Ai.Prompt)
	}
	if ConfigStruct.FeedReply.Prompt != "" {
		t.Fatalf("FeedReply.Prompt = %q, want empty prompt", ConfigStruct.FeedReply.Prompt)
	}
}
