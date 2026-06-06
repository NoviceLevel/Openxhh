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

func TestApplyDefaultsSetsCharacterPrompt(t *testing.T) {
	oldConfig := ConfigStruct
	ConfigStruct.Ai.Prompt = ""
	t.Cleanup(func() {
		ConfigStruct = oldConfig
	})

	applyDefaults()
	if ConfigStruct.Ai.Prompt != defaultAIPrompt {
		t.Fatalf("Ai.Prompt = %q, want default character prompt", ConfigStruct.Ai.Prompt)
	}
}
