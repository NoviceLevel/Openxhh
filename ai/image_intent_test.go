package ai

import "testing"

func TestParseImageIntentContent(t *testing.T) {
	content := "```json\n{\"is_image_request\":true,\"image_prompt\":\" 根据楼层内容生成总结图 \",\"mention_target\":\"\",\"needs_post_context\":false,\"needs_comment_context\":true,\"needs_image_input\":false,\"wants_similar_image\":false,\"reason\":\"明确请求\"}\n```"
	got, err := ParseImageIntentContent(content, "张三")
	if err != nil {
		t.Fatalf("ParseImageIntentContent returned error: %v", err)
	}
	if !got.IsImageRequest {
		t.Fatal("IsImageRequest should be true")
	}
	if got.ImagePrompt != "根据楼层内容生成总结图" {
		t.Fatalf("ImagePrompt = %q", got.ImagePrompt)
	}
	if got.MentionTarget != "张三" {
		t.Fatalf("MentionTarget = %q", got.MentionTarget)
	}
	if !got.NeedsCommentContext {
		t.Fatal("NeedsCommentContext should be true")
	}
}

func TestParseImageIntentContentNotImageRequest(t *testing.T) {
	got, err := ParseImageIntentContent(`{"is_image_request":false,"image_prompt":"","reason":"讨论生图功能"}`, "")
	if err != nil {
		t.Fatalf("ParseImageIntentContent returned error: %v", err)
	}
	if got.IsImageRequest {
		t.Fatal("IsImageRequest should be false")
	}
}
