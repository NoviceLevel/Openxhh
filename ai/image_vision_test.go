package ai

import "testing"

func TestBuildImageVisionPromptKeepsImages(t *testing.T) {
	var image Content
	image.Type = "image_url"
	image.ImgUrl.Url = "https://example.com/a.png"
	contents := []Content{{Type: "text", Text: "上下文"}, image}

	got := buildImageVisionPrompt("请描述图片", contents)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Type != "text" || got[0].Text != "请描述图片" {
		t.Fatalf("first content = %+v", got[0])
	}
	if got[1].Type != "image_url" || got[1].ImgUrl.Url != image.ImgUrl.Url {
		t.Fatalf("image content = %+v", got[1])
	}
}

func TestDescribeImagesForGenerationNoImageContent(t *testing.T) {
	if _, err := DescribeImagesForGeneration(nil, []Content{{Type: "text", Text: "只有文本"}}, "请描述图片"); err == nil {
		t.Fatal("DescribeImagesForGeneration should fail when no image_url exists")
	}
}
