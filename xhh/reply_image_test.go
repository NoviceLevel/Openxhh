package xhh

import (
	"context"
	"errors"
	"openxhh/ai"
	"openxhh/config"
	"openxhh/db"
	"openxhh/loger"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestBuildReplyImagePromptIncludesReplyContext(t *testing.T) {
	oldConfig := config.ConfigStruct
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
	})

	config.ConfigStruct.Ai.ChatName = "Megumin"
	config.ConfigStruct.Ai.Description = "Crimson Demon arch wizard"
	config.ConfigStruct.Ai.Personality = "dramatic and proud"
	config.ConfigStruct.Ai.Prompt = "Only output final comment text"

	prompt := BuildReplyImagePrompt(
		[]ai.Content{
			{Type: "text", Text: "<b>Title</b>: hard boss fight"},
			{Type: "image_url"},
		},
		"How do I beat this boss?",
		"Explosion solves everything.",
	)

	for _, want := range []string{
		"Megumin",
		"Crimson Demon arch wizard",
		"hard boss fight",
		"How do I beat this boss?",
		"Explosion solves everything.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("BuildReplyImagePrompt missing %q in %q", want, prompt)
		}
	}
	if strings.Contains(prompt, "<b>") {
		t.Fatalf("BuildReplyImagePrompt should strip HTML tags: %q", prompt)
	}
	if strings.Contains(prompt, "Only output final comment text") {
		t.Fatalf("BuildReplyImagePrompt should not include text-only reply protocol: %q", prompt)
	}
}

func TestLimitReplyImageText(t *testing.T) {
	got := limitReplyImageText("abcdef", 3)
	if got != "abc" {
		t.Fatalf("limitReplyImageText = %q, want abc", got)
	}
}

func TestReplyWithOptionalImageDoesNotFallbackTextWhenImageSendFails(t *testing.T) {
	restoreReplyImageTestState(t)
	config.ConfigStruct.Image.ReplyWithImage = true
	config.ConfigStruct.Image.BaseUrl = "https://example.com/images"
	config.ConfigStruct.Image.Model = "image-model"

	generateReplyImage = func(context.Context, string) (ai.ImageResult, error) {
		return ai.ImageResult{Bytes: []byte{1, 2, 3}}, nil
	}
	resolveReplyImage = func(context.Context, ai.ImageResult, bool) (string, XHHCOSUploadPlan, error) {
		return "https://imgheybox.max-c.com/test.png", XHHCOSUploadPlan{Key: "test.png", CDNURL: "https://imgheybox.max-c.com/test.png", Uploaded: true}, nil
	}
	imageSendCalls := 0
	sendReplyImage = func(text, linkID, replyID, rootID, imageURL string) bool {
		imageSendCalls++
		return false
	}
	textSendCalls := 0
	sendReplyText = func(text, linkID, replyID, rootID, iscy string) bool {
		textSendCalls++
		return true
	}

	ok := replyWithOptionalImage(db.CommStruct{LinkID: 10, CommentID: 20, RootID: 30}, "reply", "question", nil)
	if ok {
		t.Fatal("replyWithOptionalImage returned true, want false")
	}
	if imageSendCalls != 1 {
		t.Fatalf("sendReplyImage calls = %d, want 1", imageSendCalls)
	}
	if textSendCalls != 0 {
		t.Fatalf("sendReplyText calls = %d, want 0", textSendCalls)
	}
}

func TestReplyWithOptionalImageFallsBackTextWhenGenerateFails(t *testing.T) {
	restoreReplyImageTestState(t)
	config.ConfigStruct.Image.ReplyWithImage = true
	config.ConfigStruct.Image.BaseUrl = "https://example.com/images"
	config.ConfigStruct.Image.Model = "image-model"

	generateReplyImage = func(context.Context, string) (ai.ImageResult, error) {
		return ai.ImageResult{}, errors.New("generate failed")
	}
	resolveReplyImage = func(context.Context, ai.ImageResult, bool) (string, XHHCOSUploadPlan, error) {
		t.Fatal("resolveReplyImage should not be called")
		return "", XHHCOSUploadPlan{}, nil
	}
	sendReplyImage = func(text, linkID, replyID, rootID, imageURL string) bool {
		t.Fatal("sendReplyImage should not be called")
		return false
	}
	textSendCalls := 0
	sendReplyText = func(text, linkID, replyID, rootID, iscy string) bool {
		textSendCalls++
		return true
	}

	ok := replyWithOptionalImage(db.CommStruct{LinkID: 10, CommentID: 20, RootID: 30}, "reply", "question", nil)
	if !ok {
		t.Fatal("replyWithOptionalImage returned false, want true")
	}
	if textSendCalls != 1 {
		t.Fatalf("sendReplyText calls = %d, want 1", textSendCalls)
	}
}

func restoreReplyImageTestState(t *testing.T) {
	t.Helper()
	oldConfig := config.ConfigStruct
	oldLogger := loger.Loger
	oldGenerate := generateReplyImage
	oldResolve := resolveReplyImage
	oldSendImage := sendReplyImage
	oldSendText := sendReplyText
	loger.Loger = zap.NewNop()
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
		loger.Loger = oldLogger
		generateReplyImage = oldGenerate
		resolveReplyImage = oldResolve
		sendReplyImage = oldSendImage
		sendReplyText = oldSendText
	})
}
