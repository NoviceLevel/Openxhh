package xhh

import (
	"openxhh/ai"
	"openxhh/config"
	"openxhh/loger"
	"strings"
	"testing"

	"go.uber.org/zap"
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

func TestGenerateAIReplyWithQualityRetryUntilValid(t *testing.T) {
	restoreReplyQualityTestState(t)
	config.ConfigStruct.Ai.ChatName = "Megumin"
	config.ConfigStruct.Ai.Personality = "explosion magic"

	calls := 0
	getAIReplyForQualityRetry = func([]ai.Content, string, []ai.Topics, []ai.Tags, ...zap.Field) string {
		calls++
		if calls < 4 {
			return "That looks reasonable."
		}
		return "Megumin says explosion is the answer."
	}

	got, skipped := generateAIReplyWithQualityRetry(nil, "hello", nil, nil)
	if skipped {
		t.Fatal("generateAIReplyWithQualityRetry skipped, want valid reply")
	}
	if got != "Megumin says explosion is the answer." {
		t.Fatalf("reply = %q", got)
	}
	if calls != 4 {
		t.Fatalf("calls = %d, want 4", calls)
	}
}

func TestGenerateAIReplyWithQualityRetryStopsAtLimit(t *testing.T) {
	restoreReplyQualityTestState(t)
	config.ConfigStruct.Ai.ChatName = "Megumin"
	config.ConfigStruct.Ai.Personality = "explosion magic"

	calls := 0
	getAIReplyForQualityRetry = func([]ai.Content, string, []ai.Topics, []ai.Tags, ...zap.Field) string {
		calls++
		return "That looks reasonable."
	}

	got, skipped := generateAIReplyWithQualityRetry(nil, "hello", nil, nil)
	if !skipped {
		t.Fatal("generateAIReplyWithQualityRetry skipped = false, want true")
	}
	if got != "" {
		t.Fatalf("reply = %q, want empty", got)
	}
	if calls != maxReplyQualityAttempts {
		t.Fatalf("calls = %d, want %d", calls, maxReplyQualityAttempts)
	}
}

func TestGenerateFeedReplyWithQualityRetryUntilValid(t *testing.T) {
	restoreReplyQualityTestState(t)
	config.ConfigStruct.Ai.ChatName = "Megumin"
	config.ConfigStruct.Ai.Personality = "explosion magic"

	calls := 0
	getAIFeedReplyForQualityRetry = func(string, []ai.Content, string, []ai.Topics, []ai.Tags, ...zap.Field) string {
		calls++
		if calls < 3 {
			return "That looks reasonable."
		}
		return "Megumin says explosion belongs in this post."
	}

	got := generateFeedReplyWithQualityRetry("prompt", nil, "instruction", "", nil, nil)
	if got != "Megumin says explosion belongs in this post." {
		t.Fatalf("reply = %q", got)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}

func restoreReplyQualityTestState(t *testing.T) {
	t.Helper()
	oldConfig := config.ConfigStruct
	oldLogger := loger.Loger
	oldAIReply := getAIReplyForQualityRetry
	oldFeedReply := getAIFeedReplyForQualityRetry
	loger.Loger = zap.NewNop()
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
		loger.Loger = oldLogger
		getAIReplyForQualityRetry = oldAIReply
		getAIFeedReplyForQualityRetry = oldFeedReply
	})
}
