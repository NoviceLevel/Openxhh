package xhh

import (
	"context"
	"errors"
	"openxhh/ai"
	"openxhh/config"
	"openxhh/db"
	"openxhh/loger"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestAppendUniqueMention(t *testing.T) {
	trigger := buildMention(1001, "触发者")
	target := buildMention(2002, "目标用户")

	mentions := appendUniqueMention(nil, trigger)
	mentions = appendUniqueMention(mentions, target)
	if got := len(mentions); got != 2 {
		t.Fatalf("len(mentions) = %d, want 2", got)
	}

	mentions = appendUniqueMention(mentions, buildMention(1001, "触发者新名字"))
	if got := len(mentions); got != 2 {
		t.Fatalf("len(mentions) after duplicate = %d, want 2", got)
	}
}

func TestExtractMentionUserID(t *testing.T) {
	if got := extractMentionUserID(buildMention(1001, "触发者")); got != "1001" {
		t.Fatalf("extractMentionUserID = %q, want 1001", got)
	}
}

func TestNormalizeImageReplyText(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{name: "normal reply", text: " 图来啦，快看看喵 ", want: "图来啦，快看看喵"},
		{name: "remove quotes and newlines", text: "“图片来啦\n快看看”", want: "图片来啦 快看看"},
		{name: "hide generated prompt", text: "已生成：一只猫", want: defaultImageReplyText},
		{name: "hide prompt word", text: "prompt 已经处理好了", want: defaultImageReplyText},
		{name: "empty reply", text: "   ", want: defaultImageReplyText},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeImageReplyText(tt.text); got != tt.want {
				t.Fatalf("normalizeImageReplyText(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestProcessRoutedImageCommentFallsBackToTextWhenImageGenerationFails(t *testing.T) {
	oldConfig := config.ConfigStruct
	oldLogger := loger.Loger
	oldGenerateImage := generateImageForComment
	oldAIReply := getAIReplyForQualityRetry
	oldSendText := sendReplyText
	oldCooldown := xhhCaptchaCooldownUntil.Load()
	loger.Loger = zap.NewNop()
	config.ConfigStruct.Xhh.EnableWhitelist = false
	config.ConfigStruct.Image.ReplyWithImage = false
	xhhCaptchaCooldownUntil.Store(time.Now().Add(time.Minute).Unix())
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
		loger.Loger = oldLogger
		generateImageForComment = oldGenerateImage
		getAIReplyForQualityRetry = oldAIReply
		sendReplyText = oldSendText
		xhhCaptchaCooldownUntil.Store(oldCooldown)
	})

	generateImageForComment = func(context.Context, string, ImageCommentOptions) (ai.ImageResult, error) {
		return ai.ImageResult{}, errors.New("invalid image token")
	}
	getAIReplyForQualityRetry = func([]ai.Content, string, []ai.Topics, []ai.Tags, ...zap.Field) string {
		return "转什么猫娘啊！本大魔法师只给你喵一小下。"
	}
	textSends := 0
	sendReplyText = func(text, linkID, replyID, rootID, iscy string) bool {
		textSends++
		if replyID != "20" {
			t.Fatalf("replyID = %q, want 20", replyID)
		}
		return true
	}

	ok := processRoutedImageComment(
		db.CommStruct{LinkID: 10, CommentID: 20, RootID: 30, Uid: 40, Text: "转猫娘", UserName: "user"},
		ParseMentionControl("转猫娘"),
		&ai.CommentRouteResult{Action: ai.CommentRouteActionImage, ImagePrompt: "猫娘"},
	)
	if !ok {
		t.Fatal("processRoutedImageComment returned false, want text fallback success")
	}
	if textSends != 1 {
		t.Fatalf("text sends = %d, want 1", textSends)
	}
}
