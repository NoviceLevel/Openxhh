package main

import "testing"

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
