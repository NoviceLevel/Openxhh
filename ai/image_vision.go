package ai

import (
	"context"
	"errors"
	"fmt"
	"openxhh/config"
	"strings"
)

func DescribeImagesForGeneration(ctx context.Context, contents []Content, instruction string) (string, error) {
	imageCount := 0
	for _, content := range contents {
		if content.Type == "image_url" && strings.TrimSpace(content.ImgUrl.Url) != "" {
			imageCount++
		}
	}
	if imageCount == 0 {
		return "", errors.New("no image content available")
	}
	model := config.ConfigStruct.Ai.Model
	if strings.TrimSpace(model) == "" {
		return "", errors.New("ai model is not configured")
	}

	messages := []chatCompletionMessage{
		{Role: "system", Content: imageVisionSystemPrompt()},
		{Role: "user", Content: buildImageVisionPrompt(instruction, contents)},
	}
	content, err := sendChatCompletion(ctx, model, messages)
	if err != nil {
		return "", fmt.Errorf("image vision request failed: %w", err)
	}
	return strings.TrimSpace(content), nil
}

func imageVisionSystemPrompt() string {
	return `你是图片理解助手。请阅读用户提供的图片和少量文本上下文，把图片内容转写成适合文生图模型使用的中文描述。
只输出中文画面描述，不要解释，不要提到你看到了图片，不要输出“我不能看图”之类的话。
如果用户说“类似这张图”，请保留主体、构图、风格、色彩和氛围，但不要提及“原图”。`
}

func buildImageVisionPrompt(instruction string, contents []Content) []Content {
	prompt := []Content{{Type: "text", Text: strings.TrimSpace(instruction)}}
	for _, content := range contents {
		if content.Type == "image_url" && strings.TrimSpace(content.ImgUrl.Url) != "" {
			prompt = append(prompt, content)
		}
	}
	return prompt
}
