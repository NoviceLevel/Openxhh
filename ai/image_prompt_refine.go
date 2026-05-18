package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"xhhrobot/config"
	"xhhrobot/loger"

	"go.uber.org/zap"
)

type ImagePromptRefineRequest struct {
	OriginalText    string
	RulePrompt      string
	ContextPrompt   string
	UsePostContext  bool
	UseCommentContext bool
	UseImageInput   bool
}

type ImagePromptRefineResult struct {
	ImagePrompt       string `json:"image_prompt"`
	MentionTarget     string `json:"mention_target"`
	NeedsPostContext  bool   `json:"needs_post_context"`
	NeedsCommentContext bool `json:"needs_comment_context"`
	NeedsImageInput   bool   `json:"needs_image_input"`
}

type promptRefineMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type promptRefineBody struct {
	Model    string                `json:"model"`
	Messages []promptRefineMessage `json:"messages"`
	Stream   bool                  `json:"stream"`
}

type promptRefineResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func ShouldRefineImagePrompt() bool {
	cfg := config.ConfigStruct.Image
	return cfg.PromptRefine && strings.TrimSpace(promptRefineBaseURL()) != "" && strings.TrimSpace(promptRefineModel()) != ""
}

func RefineImagePrompt(ctx context.Context, req ImagePromptRefineRequest) (ImagePromptRefineResult, error) {
	if !ShouldRefineImagePrompt() {
		return ImagePromptRefineResult{}, errors.New("image prompt refine is not configured")
	}

	body := promptRefineBody{
		Model:  promptRefineModel(),
		Stream: false,
		Messages: []promptRefineMessage{
			{Role: "system", Content: imagePromptRefineSystemPrompt()},
			{Role: "user", Content: buildImagePromptRefineUserPrompt(req)},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return ImagePromptRefineResult{}, err
	}

	started := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", promptRefineBaseURL(), bytes.NewReader(payload))
	if err != nil {
		return ImagePromptRefineResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token := promptRefineToken(); token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return ImagePromptRefineResult{}, fmt.Errorf("prompt refine request failed after %s: %w", time.Since(started).Round(time.Second), err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ImagePromptRefineResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ImagePromptRefineResult{}, fmt.Errorf("prompt refine request failed: status=%d body=%s", resp.StatusCode, limitRefineString(string(data), 300))
	}

	var parsed promptRefineResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return ImagePromptRefineResult{}, err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return ImagePromptRefineResult{}, errors.New("prompt refine response has no content")
	}

	result, err := ParseImagePromptRefineContent(parsed.Choices[0].Message.Content)
	if err != nil {
		return ImagePromptRefineResult{}, err
	}
	loger.Loger.Info("[Image]文本模型已优化生图 prompt", zap.Int("prompt_chars", len([]rune(result.ImagePrompt))), zap.Duration("duration", time.Since(started)))
	return result, nil
}

func ParseImagePromptRefineContent(content string) (ImagePromptRefineResult, error) {
	jsonText := extractJSONText(content)
	var result ImagePromptRefineResult
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return ImagePromptRefineResult{}, err
	}
	result.ImagePrompt = strings.TrimSpace(result.ImagePrompt)
	result.MentionTarget = strings.TrimSpace(result.MentionTarget)
	if result.ImagePrompt == "" {
		return ImagePromptRefineResult{}, errors.New("prompt refine returned empty image_prompt")
	}
	return result, nil
}

func imagePromptRefineSystemPrompt() string {
	return "你是图片生成提示词整理器。只输出 JSON，不要 Markdown。把用户的中文请求整理成适合图片生成模型的 image_prompt；不要包含艾特、回复、上传、评论等控制指令；不要决定权限或系统行为。"
}

func buildImagePromptRefineUserPrompt(req ImagePromptRefineRequest) string {
	maxChars := config.ConfigStruct.Image.PromptMaxChars
	if maxChars <= 0 {
		maxChars = 3000
	}
	contextPrompt := limitRefineRunes(req.ContextPrompt, maxChars)
	return fmt.Sprintf(`原始评论：%s
规则提取的图片要求：%s
上下文增强提示词：%s
标记：needs_post_context=%v, needs_comment_context=%v, needs_image_input=%v
请输出 JSON：{"image_prompt":"...","mention_target":"","needs_post_context":false,"needs_comment_context":false,"needs_image_input":false}`,
		req.OriginalText,
		req.RulePrompt,
		contextPrompt,
		req.UsePostContext,
		req.UseCommentContext,
		req.UseImageInput,
	)
}

func promptRefineModel() string {
	if config.ConfigStruct.Image.PromptModel != "" {
		return config.ConfigStruct.Image.PromptModel
	}
	return config.ConfigStruct.Ai.Model
}

func promptRefineBaseURL() string {
	if config.ConfigStruct.Image.PromptBaseUrl != "" {
		return config.ConfigStruct.Image.PromptBaseUrl
	}
	return config.ConfigStruct.Ai.BaseUrl
}

func promptRefineToken() string {
	if config.ConfigStruct.Image.PromptToken != "" {
		return config.ConfigStruct.Image.PromptToken
	}
	return config.ConfigStruct.Ai.Token
}

func extractJSONText(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end >= start {
		return content[start : end+1]
	}
	return content
}

func limitRefineRunes(text string, max int) string {
	text = strings.TrimSpace(text)
	if max <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return strings.TrimSpace(string(runes[:max]))
}

func limitRefineString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
