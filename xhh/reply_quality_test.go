package xhh

import (
	"openxhh/config"
	"strings"
	"testing"
)

func TestAIReplyQualityIssueUsesAIPersonaAnchors(t *testing.T) {
	oldConfig := config.ConfigStruct
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
	})
	config.ConfigStruct.Ai.ChatName = "Megumin"
	config.ConfigStruct.Ai.Description = ""
	config.ConfigStruct.Ai.Personality = "proud arch wizard explosion"
	config.ConfigStruct.Ai.Scenario = ""
	config.ConfigStruct.Ai.FirstMessage = ""
	config.ConfigStruct.Ai.ExampleDialogs = ""
	config.ConfigStruct.Ai.PostHistoryInstructions = ""
	config.ConfigStruct.Ai.Prompt = ""

	if got := aiReplyQualityIssue("Megumin says explosion solves this."); got != "" {
		t.Fatalf("aiReplyQualityIssue valid reply = %q, want empty", got)
	}
	if got := aiReplyQualityIssue("That looks pretty reasonable."); got == "" {
		t.Fatal("aiReplyQualityIssue generic reply = empty, want issue")
	}
}

func TestAIReplyQualityIssueDoesNotTreatSkipAsValid(t *testing.T) {
	oldConfig := config.ConfigStruct
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
	})
	config.ConfigStruct.Ai.ChatName = "Megumin"

	if got := aiReplyQualityIssue("SKIP"); got == "" {
		t.Fatal("aiReplyQualityIssue(SKIP) = empty, want issue")
	}
}

func TestAIReplyRetryInstructionIncludesAnchors(t *testing.T) {
	oldConfig := config.ConfigStruct
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
	})
	config.ConfigStruct.Ai.ChatName = "Megumin"
	config.ConfigStruct.Ai.Personality = "explosion magic"

	got := aiReplyRetryInstruction("hello", "missing persona")
	for _, want := range []string{"hello", "missing persona", "Megumin", "explosion"} {
		if !strings.Contains(got, want) {
			t.Fatalf("aiReplyRetryInstruction missing %q in %q", want, got)
		}
	}
}
