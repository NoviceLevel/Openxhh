package xhh

import (
	"openxhh/ai"
	"openxhh/config"
	"openxhh/loger"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestReplyQualityOnlyKeepsSendLevelChecks(t *testing.T) {
	restoreReplyQualityTestState(t)
	config.ConfigStruct.Ai.ChatName = "惠惠"

	styleReplies := []string{
		"建议你先看看预算和需求。",
		"这里是惠惠专席，要么领成就，要么报委托。",
		"*惠惠压低帽檐。*\n\n这事确实要先看清楚。\n\n*她又把法杖往地上一杵。*",
		"哼，这次算你说得不错。",
		"惠惠觉得这事可以先缓一下，惠惠不是不管你。",
		"转什么deepseek！本大魔法师又不是传送阵客服！",
	}
	for _, reply := range styleReplies {
		if got := aiReplyQualityIssue(reply); got != "" {
			t.Fatalf("aiReplyQualityIssue(%q) = %q, want no style rejection", reply, got)
		}
	}
}

func TestReplyQualityStillRejectsSkipAndOverLengthForSendSafety(t *testing.T) {
	restoreReplyQualityTestState(t)

	if got := aiReplyQualityIssue("SKIP"); got == "" {
		t.Fatal("aiReplyQualityIssue(SKIP) = empty, want send-level issue")
	}
	if got := feedReplyQualityIssue("SKIP", ""); got != "" {
		t.Fatalf("feedReplyQualityIssue(SKIP) = %q, want allowed feed skip", got)
	}

	tooLong := strings.Repeat("测", xhhCommentMaxRunes+1)
	if got := aiReplyQualityIssue(tooLong); got != "回复过长" {
		t.Fatalf("aiReplyQualityIssue over limit = %q, want 回复过长", got)
	}
	if got := feedReplyQualityIssue(tooLong, ""); got != "回复过长" {
		t.Fatalf("feedReplyQualityIssue over limit = %q, want 回复过长", got)
	}
}

func TestGenerateAIReplyDoesNotRetryStyleReplies(t *testing.T) {
	restoreReplyQualityTestState(t)

	calls := 0
	getAIReplyForQualityRetry = func([]ai.Content, string, []ai.Topics, []ai.Tags, ...zap.Field) string {
		calls++
		return "建议你先看看预算和需求。"
	}

	got, skipped := generateAIReplyWithQualityRetry(nil, "hello", nil, nil)
	if skipped {
		t.Fatal("generateAIReplyWithQualityRetry skipped style reply, want sent")
	}
	if got != "建议你先看看预算和需求。" {
		t.Fatalf("reply = %q", got)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestGenerateAIReplyAddsTransferRoleInstruction(t *testing.T) {
	restoreReplyQualityTestState(t)

	var capturedQuestion string
	var capturedContents []ai.Content
	getAIReplyForQualityRetry = func(contents []ai.Content, question string, topics []ai.Topics, tags []ai.Tags, fields ...zap.Field) string {
		capturedQuestion = question
		capturedContents = append([]ai.Content(nil), contents...)
		return "哎呀，当然要交给本女神啦！"
	}

	got, skipped := generateAIReplyWithQualityRetry([]ai.Content{{Type: "text", Text: "原上下文"}}, "转阿库娅", nil, nil)
	if skipped {
		t.Fatal("generateAIReplyWithQualityRetry skipped transfer role reply")
	}
	if got != "哎呀，当然要交给本女神啦！" {
		t.Fatalf("reply = %q", got)
	}
	if capturedQuestion != "转阿库娅" {
		t.Fatalf("question passed to AI = %q, want original user text", capturedQuestion)
	}
	foundInstruction := false
	for _, content := range capturedContents {
		if strings.Contains(content.Text, "阿库娅") && strings.Contains(content.Text, "口吻") && strings.Contains(content.Text, "不要复读") {
			foundInstruction = true
			break
		}
	}
	if !foundInstruction {
		t.Fatalf("transfer role instruction was not appended: %+v", capturedContents)
	}
}

func TestAIReplyQualityRejectsTransferCommandEcho(t *testing.T) {
	restoreReplyQualityTestState(t)

	badReply := "没问题，在那位废柴女神掉链子之前，先听好了：转阿库娅。"
	if got := aiReplyQualityIssueForQuestion(badReply, "转阿库娅"); got == "" {
		t.Fatal("aiReplyQualityIssueForQuestion returned empty for transfer command echo")
	}

	goodReply := "哎呀，当然要交给本女神啦！"
	if got := aiReplyQualityIssueForQuestion(goodReply, "转阿库娅"); got != "" {
		t.Fatalf("aiReplyQualityIssueForQuestion role reply = %q, want empty", got)
	}
}

func TestGenerateAIReplyRetriesTransferRoleEcho(t *testing.T) {
	restoreReplyQualityTestState(t)

	replies := []string{
		"没问题，在那位废柴女神掉链子之前，先听好了：转阿库娅。",
		"哎呀，当然要交给本女神啦！",
	}
	calls := 0
	getAIReplyForQualityRetry = func(contents []ai.Content, question string, topics []ai.Topics, tags []ai.Tags, fields ...zap.Field) string {
		calls++
		if calls == 2 && (!strings.Contains(question, "阿库娅") || !strings.Contains(question, "不要输出")) {
			t.Fatalf("retry question missing transfer role guidance: %q", question)
		}
		return replies[calls-1]
	}

	got, skipped := generateAIReplyWithQualityRetry(nil, "转阿库娅", nil, nil)
	if skipped {
		t.Fatal("generateAIReplyWithQualityRetry skipped valid transfer retry reply")
	}
	if got != replies[1] {
		t.Fatalf("reply = %q, want retry reply %q", got, replies[1])
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestAIReplyQualityAllowsSeriousQuestionsButRejectsLongTemplate(t *testing.T) {
	restoreReplyQualityTestState(t)

	longReply := "哼，叫我出来就是为了这种扫盲工作吗？我可是精通爆裂魔法的大魔法师，才不是什么图鉴检索器。\n\n不过看在图里有那么多能被爆裂魔法一次性解决的元素，我就勉强看看吧。那分明是大乱炖，谁还分得那么清啊！"
	if got := aiReplyQualityIssueForQuestion(longReply, "帮忙分析下图9都有哪些动漫元素"); got == "" {
		t.Fatal("aiReplyQualityIssueForQuestion returned empty for long template reply")
	}

	shortAnswer := "图9像是动漫梗大乱炖，我能看出几处角色发型和校服元素。哼，发原图清楚点我再给你逐个炸出来。"
	if got := aiReplyQualityIssueForQuestion(shortAnswer, "帮忙分析下图9都有哪些动漫元素"); got != "" {
		t.Fatalf("aiReplyQualityIssueForQuestion short serious reply = %q, want empty", got)
	}
}

func TestGenerateAIReplyRetriesLongTemplateReplyOnce(t *testing.T) {
	restoreReplyQualityTestState(t)

	replies := []string{
		"哼，叫我出来就是为了这种扫盲工作吗？我可是精通爆裂魔法的大魔法师，才不是什么图鉴检索器。\n\n不过看在图里有那么多能被爆裂魔法一次性解决的元素，我就勉强看看吧。那分明是大乱炖，谁还分得那么清啊！",
		"图9像是动漫梗大乱炖，我能看出几处角色发型和校服元素。哼，发原图清楚点我再给你逐个炸出来。",
	}
	calls := 0
	getAIReplyForQualityRetry = func(contents []ai.Content, question string, topics []ai.Topics, tags []ai.Tags, fields ...zap.Field) string {
		calls++
		if calls == 2 && !strings.Contains(question, "上一条回复太长") {
			t.Fatalf("retry question missing rewrite guidance: %q", question)
		}
		return replies[calls-1]
	}

	got, skipped := generateAIReplyWithQualityRetry(nil, "帮忙分析下图9都有哪些动漫元素", nil, nil)
	if skipped {
		t.Fatal("generateAIReplyWithQualityRetry skipped valid retry reply")
	}
	if got != replies[1] {
		t.Fatalf("reply = %q, want retry reply %q", got, replies[1])
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestGenerateAIReplySendsRetryWhenOnlyNaturalStyleIssueRemains(t *testing.T) {
	restoreReplyQualityTestState(t)

	replies := []string{
		"哼，叫我出来就是为了这种跑腿找兑换码的事吗？我可是大魔法师，才不是什么随叫随到的寻宝机器人！\n\n不过，既然大家都想要，那我就勉为其难帮你留意一下吧。要是让我发现剩下的代码，我会大发慈悲地告诉你，记得时刻感谢我。",
		"哼，这么多人把我也叫出来，结果就为了这几个兑换码？看来大家都在蹲守呢。可惜本大魔法师对这些花哨的时装没兴趣，还没我那身披风帅气。",
	}
	calls := 0
	getAIReplyForQualityRetry = func(contents []ai.Content, question string, topics []ai.Topics, tags []ai.Tags, fields ...zap.Field) string {
		calls++
		return replies[calls-1]
	}

	got, skipped := generateAIReplyWithQualityRetry(nil, "看盒千日，用盒一时，请列出失落城堡2的八个时装皮肤兑换码", nil, nil)
	if skipped {
		t.Fatal("generateAIReplyWithQualityRetry skipped retry with only natural-style issue")
	}
	if got != replies[1] {
		t.Fatalf("reply = %q, want retry reply %q", got, replies[1])
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestAIReplyQualityTreatsExchangeCodeQuestionsAsSerious(t *testing.T) {
	restoreReplyQualityTestState(t)

	reply := "可用兑换码建议先试这些：LC2FASHION01、LC2FASHION02、LC2FASHION03、LC2FASHION04、LC2FASHION05、LC2FASHION06、LC2FASHION07、LC2FASHION08。哼，过期了可别赖我。"
	if got := aiReplyQualityIssueForQuestion(reply, "请列出失落城堡2的八个时装皮肤兑换码"); got != "" {
		t.Fatalf("aiReplyQualityIssueForQuestion exchange-code reply = %q, want empty", got)
	}
}

func TestGenerateFeedReplyDoesNotRetryShortStyleReplies(t *testing.T) {
	restoreReplyQualityTestState(t)

	calls := 0
	getAIFeedReplyForQualityRetry = func(string, []ai.Content, string, []ai.Topics, []ai.Tags, ...zap.Field) string {
		calls++
		return "这价格看着还行，火力也不错。"
	}

	got := generateFeedReplyWithQualityRetry("prompt", nil, "instruction", "", nil, nil)
	if got != "这价格看着还行，火力也不错。" {
		t.Fatalf("reply = %q", got)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestFeedReplyQualityRejectsLongNaturalReplies(t *testing.T) {
	restoreReplyQualityTestState(t)

	longReply := "喂，别在那儿哭丧着脸了！那种一看就是圈子自闭的群，退了就退了，这叫及时止损！要是这种群里没几个能听懂我爆裂魔法真谛的家伙，我早就一个魔法炸过去让服务器原地退休了。"
	if got := feedReplyQualityIssue(longReply, ""); got == "" {
		t.Fatal("feedReplyQualityIssue returned empty for long feed reply")
	}

	shortReply := "退了就退了，别为了没人接话的群难过。哼，是他们没眼光。"
	if got := feedReplyQualityIssue(shortReply, ""); got != "" {
		t.Fatalf("feedReplyQualityIssue short reply = %q, want empty", got)
	}
}

func TestGenerateFeedReplyRetriesLongReplyOnce(t *testing.T) {
	restoreReplyQualityTestState(t)

	replies := []string{
		"喂，别在那儿哭丧着脸了！那种一看就是圈子自闭的群，退了就退了，这叫及时止损！要是这种群里没几个能听懂我爆裂魔法真谛的家伙，我早就一个魔法炸过去让服务器原地退休了。",
		"退了就退了，别为了没人接话的群难过。哼，是他们没眼光。",
	}
	calls := 0
	getAIFeedReplyForQualityRetry = func(prompt string, contents []ai.Content, instruction string, topics []ai.Topics, tags []ai.Tags, fields ...zap.Field) string {
		calls++
		if calls == 2 && !strings.Contains(instruction, "上一条太长") {
			t.Fatalf("retry instruction missing short-rewrite guidance: %q", instruction)
		}
		return replies[calls-1]
	}

	got := generateFeedReplyWithQualityRetry("prompt", nil, "instruction", "", nil, nil)
	if got != replies[1] {
		t.Fatalf("reply = %q, want retry reply %q", got, replies[1])
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
