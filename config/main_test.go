package config

import (
	"reflect"
	"testing"
)

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

func TestApplyDefaultsDoesNotSetImageResponseFormat(t *testing.T) {
	oldConfig := ConfigStruct
	t.Cleanup(func() {
		ConfigStruct = oldConfig
	})
	resetConfigStructForTest()

	applyDefaults()
	if ConfigStruct.Image.ResponseFormat != "" {
		t.Fatalf("Image.ResponseFormat = %q, want empty", ConfigStruct.Image.ResponseFormat)
	}
	if ConfigStruct.Image.ReplyWithImage {
		t.Fatal("Image.ReplyWithImage = true, want false by default")
	}
}

func TestApplyDefaultsMigratesCharacterCard(t *testing.T) {
	oldConfig := ConfigStruct
	t.Cleanup(func() {
		ConfigStruct = oldConfig
	})
	resetConfigStructForTest()

	ConfigStruct.Ai.CharacterCard = "legacy persona"

	if !applyDefaults() {
		t.Fatal("applyDefaults returned false, want migration change")
	}
	if ConfigStruct.Ai.Description != "legacy persona" {
		t.Fatalf("Ai.Description = %q, want migrated character card", ConfigStruct.Ai.Description)
	}
	if ConfigStruct.Ai.CharacterCard != "" {
		t.Fatalf("Ai.CharacterCard = %q, want cleared legacy field", ConfigStruct.Ai.CharacterCard)
	}
}

func TestApplyDefaultsDoesNotOverwriteDescriptionWithCharacterCard(t *testing.T) {
	oldConfig := ConfigStruct
	t.Cleanup(func() {
		ConfigStruct = oldConfig
	})
	resetConfigStructForTest()

	ConfigStruct.Ai.Description = "new persona"
	ConfigStruct.Ai.CharacterCard = "legacy persona"

	applyDefaults()
	if ConfigStruct.Ai.Description != "new persona" {
		t.Fatalf("Ai.Description = %q, want existing description", ConfigStruct.Ai.Description)
	}
	if ConfigStruct.Ai.CharacterCard != "" {
		t.Fatalf("Ai.CharacterCard = %q, want cleared legacy field", ConfigStruct.Ai.CharacterCard)
	}
}

func resetConfigStructForTest() {
	reflect.ValueOf(&ConfigStruct).Elem().Set(reflect.Zero(reflect.TypeOf(ConfigStruct)))
}
