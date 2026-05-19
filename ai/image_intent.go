package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"openxhh/config"
	"strings"
	"time"
)

type ImageIntentRequest struct {
	RawComment        string
	NormalizedText    string
	CleanedText       string
	MentionTarget     string
	UsePostContext    bool
	UseCommentContext bool
	UseImageInput     bool
}

type ImageIntentResult struct {
	IsImageRequest      bool   `json:"is_image_request"`
	ImagePrompt         string `json:"image_prompt"`
	MentionTarget       string `json:"mention_target"`
	NeedsPostContext    bool   `json:"needs_post_context"`
	NeedsCommentContext bool   `json:"needs_comment_context"`
	NeedsImageInput     bool   `json:"needs_image_input"`
	WantsSimilarImage   bool   `json:"wants_similar_image"`
	Reason              string `json:"reason"`
}

func UnderstandImageIntent(ctx context.Context, req ImageIntentRequest) (ImageIntentResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	model := config.ConfigStruct.Ai.Model
	if strings.TrimSpace(model) == "" {
		return ImageIntentResult{}, errors.New("ai model is not configured")
	}

	payload := buildImageIntentPrompt(req)
	content, err := sendChatCompletion(ctx, model, []chatCompletionMessage{
		{Role: "system", Content: imageIntentSystemPrompt()},
		{Role: "user", Content: payload},
	})
	if err != nil {
		return ImageIntentResult{}, fmt.Errorf("image intent request failed: %w", err)
	}

	return ParseImageIntentContent(content, req.MentionTarget)
}

func ParseImageIntentContent(content string, fallbackMentionTarget string) (ImageIntentResult, error) {
	var result ImageIntentResult
	if err := json.Unmarshal([]byte(extractJSONText(content)), &result); err != nil {
		return ImageIntentResult{}, err
	}
	result.ImagePrompt = strings.TrimSpace(result.ImagePrompt)
	result.MentionTarget = strings.TrimSpace(result.MentionTarget)
	result.Reason = strings.TrimSpace(result.Reason)
	if result.MentionTarget == "" {
		result.MentionTarget = strings.TrimSpace(fallbackMentionTarget)
	}
	if result.ImagePrompt == "" {
		result.IsImageRequest = false
	}
	return result, nil
}

func imageIntentSystemPrompt() string {
	return `你是 Openxhh 的生图请求理解器，只输出 JSON，不要 Markdown，不要解释。
你的任务是先判断这条评论是不是在请求生成图片，再把意图拆成结构化字段。

规则：
1. 不要因为出现“生图”“生成图片”“画图”就直接判定为图片请求；如果用户只是在讨论生图、询问原理、解释失败原因、比较模型能力，不算生图请求。
2. 如果用户是在请求生成、画、做、来一张、根据正文/帖子/评论区/这层楼/图片生成图片，才算生图请求。
3. 如果用户说“根据正文/文章/帖子/原帖/评论区/这层楼/这张图片”，必须把上下文当成主体来源，而不是忽略。
4. “艾特谁来看、给谁看、让谁看、回复谁、喊谁来看”属于最终目标 mention，不要写进 image_prompt。
5. 机器人 @ 只是唤醒，不是 prompt，也不是最终目标 mention。
6. 如果用户要求“根据这张图片/类似这张图/参考这张图”生成，needs_image_input 必须为 true。
7. image_prompt 必须是适合图片生成模型的画面描述，简洁、具体，不要输出解释句。
8. 输出 JSON 格式：{"is_image_request":true,"image_prompt":"...","mention_target":"","needs_post_context":false,"needs_comment_context":false,"needs_image_input":false,"wants_similar_image":false,"reason":"..."}`
}

func buildImageIntentPrompt(req ImageIntentRequest) string {
	return fmt.Sprintf(`原始评论：%s
归一化文本：%s
清洗后文本：%s
已提取的目标 mention：%s
当前标记：needs_post_context=%v, needs_comment_context=%v, needs_image_input=%v

请判断这是不是生图请求，并输出 JSON。若不是生图请求，请把 is_image_request 设为 false，image_prompt 置空，reason 简述原因。若是生图请求，请直接给出适合图片生成模型的最终 image_prompt。
`,
		limitIntentContext(req.RawComment),
		limitIntentContext(req.NormalizedText),
		limitIntentContext(req.CleanedText),
		req.MentionTarget,
		req.UsePostContext,
		req.UseCommentContext,
		req.UseImageInput,
	)
}

func limitIntentContext(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	runes := []rune(text)
	if len(runes) <= 1200 {
		return text
	}
	return strings.TrimSpace(string(runes[:1200]))
}
