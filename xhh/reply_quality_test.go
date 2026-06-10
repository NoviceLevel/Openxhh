package xhh

import (
	"openxhh/ai"
	"openxhh/config"
	"openxhh/loger"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestAIReplyQualityIssueAllowsNaturalReplyWithoutExplicitPersonaAnchor(t *testing.T) {
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
	if got := aiReplyQualityIssue("That looks pretty reasonable."); got != "" {
		t.Fatalf("aiReplyQualityIssue natural reply = %q, want empty", got)
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

func TestAIReplyQualityIssueRejectsRepeatedChatName(t *testing.T) {
	oldConfig := config.ConfigStruct
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
	})
	config.ConfigStruct.Ai.ChatName = "惠惠"

	if got := aiReplyQualityIssue("惠惠觉得这事可以先缓一下，惠惠不是不管你。"); got == "" {
		t.Fatal("aiReplyQualityIssue repeated chat name = empty, want issue")
	}
	if got := aiReplyQualityIssue("这事可以先缓一下，我会看着点。"); got != "" {
		t.Fatalf("aiReplyQualityIssue natural reply = %q, want empty", got)
	}
}

func TestAIReplyQualityIssueRejectsOveractedPersonaTerms(t *testing.T) {
	oldConfig := config.ConfigStruct
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
	})
	config.ConfigStruct.Ai.ChatName = "惠惠"
	config.ConfigStruct.Ai.Description = "红魔族大魔法师，会爆裂魔法"

	reply := "哼，想召唤本大魔法师来接委托？红魔族的爆裂魔法已经准备好了！"
	if got := aiReplyQualityIssue(reply); got == "" {
		t.Fatal("aiReplyQualityIssue overacted persona reply = empty, want issue")
	}
	if got := aiReplyQualityIssue("你这句像是在试探我底牌啊。可以聊，但别真把我拆开研究。"); got != "" {
		t.Fatalf("aiReplyQualityIssue natural reply = %q, want empty", got)
	}
}

func TestAIReplyQualityIssueRejectsHarshCutePingReplies(t *testing.T) {
	restoreReplyQualityTestState(t)
	config.ConfigStruct.Ai.ChatName = "惠惠"

	reply := "又喵？！这到底是暗号，还是猫化病毒已经扩散了？翻译成人话给我听！"
	if got := aiReplyQualityIssue(reply); got == "" {
		t.Fatal("aiReplyQualityIssue harsh cute-ping reply = empty, want issue")
	}
	if got := aiReplyQualityIssue("喵什么喵……哼，既然都把我叫出来了，就陪你一下。想说什么？"); got != "" {
		t.Fatalf("aiReplyQualityIssue soft cute-ping reply = %q, want empty", got)
	}
}

func TestAIReplyQualityIssueRejectsDangerLabelsForBanter(t *testing.T) {
	restoreReplyQualityTestState(t)

	reply := "你这输入法已经被病毒污染了吧，先登记成高危魔物！"
	if got := aiReplyQualityIssue(reply); got == "" {
		t.Fatal("aiReplyQualityIssue danger-label banter reply = empty, want issue")
	}
	if got := aiReplyQualityIssue("你这输入法也太会拐弯了吧。先说正事，我听一句。"); got != "" {
		t.Fatalf("aiReplyQualityIssue playful banter reply = %q, want empty", got)
	}
}

func TestAIReplyQualityIssueRejectsOverusedCharacterProps(t *testing.T) {
	restoreReplyQualityTestState(t)

	reply := "*惠惠按住帽檐，举起法杖，披风一甩，眼罩都差点歪掉。* 这是本大魔法师的爆裂魔法警告！"
	if got := aiReplyQualityIssue(reply); got == "" {
		t.Fatal("aiReplyQualityIssue prop-heavy reply = empty, want issue")
	}
	if got := aiReplyQualityIssue("不转不转，职业栏已经够挤了。你要是真有事，就好好说一句。"); got != "" {
		t.Fatalf("aiReplyQualityIssue natural banter reply = %q, want empty", got)
	}
}

func TestAIReplyRetryInstructionAvoidsForcingPersonaAnchors(t *testing.T) {
	oldConfig := config.ConfigStruct
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
	})
	config.ConfigStruct.Ai.ChatName = "Megumin"
	config.ConfigStruct.Ai.Personality = "explosion magic"

	got := aiReplyRetryInstruction("hello", "missing persona")
	for _, want := range []string{"hello", "missing persona", "不要靠反复自称名字", "用态度、情绪和判断体现人设", "动作描写只能少量点到"} {
		if !strings.Contains(got, want) {
			t.Fatalf("aiReplyRetryInstruction missing %q in %q", want, got)
		}
	}
	for _, unwanted := range []string{"Megumin", "explosion", "人设锚点"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("aiReplyRetryInstruction should not force anchor %q in %q", unwanted, got)
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
		if calls == 1 {
			return "建议你先别急着买。"
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
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestGenerateAIReplyWithQualityRetryAcceptsNaturalReplyWithoutExplicitAnchor(t *testing.T) {
	restoreReplyQualityTestState(t)
	config.ConfigStruct.Ai.ChatName = "Megumin"
	config.ConfigStruct.Ai.Personality = "explosion magic"

	calls := 0
	getAIReplyForQualityRetry = func([]ai.Content, string, []ai.Topics, []ai.Tags, ...zap.Field) string {
		calls++
		return "That looks reasonable."
	}

	got, skipped := generateAIReplyWithQualityRetry(nil, "hello", nil, nil)
	if skipped {
		t.Fatal("generateAIReplyWithQualityRetry skipped = true, want false")
	}
	if got != "That looks reasonable." {
		t.Fatalf("reply = %q", got)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestGenerateFeedReplyWithQualityRetryUntilValid(t *testing.T) {
	restoreReplyQualityTestState(t)
	config.ConfigStruct.Ai.ChatName = "Megumin"
	config.ConfigStruct.Ai.Personality = "explosion magic"

	calls := 0
	getAIFeedReplyForQualityRetry = func(string, []ai.Content, string, []ai.Topics, []ai.Tags, ...zap.Field) string {
		calls++
		if calls == 1 {
			return "建议你先看看预算和需求。"
		}
		return "Megumin says explosion belongs in this post."
	}

	got := generateFeedReplyWithQualityRetry("prompt", nil, "instruction", "", nil, nil)
	if got != "Megumin says explosion belongs in this post." {
		t.Fatalf("reply = %q", got)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
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
