package ai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"openxhh/config"
	"openxhh/loger"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

type ImageResult struct {
	Path  string
	Bytes []byte
	URL   string
}

type imageRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	Size           string `json:"size,omitempty"`
	N              int    `json:"n,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
}

type imageChatRequest struct {
	Model    string                  `json:"model"`
	Messages []chatCompletionMessage `json:"messages"`
	Size     string                  `json:"size,omitempty"`
	Stream   bool                    `json:"stream"`
}

type imageResponse struct {
	Data  []imageDataItem `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type imageDataItem struct {
	URL      string `json:"url"`
	B64JSON  string `json:"b64_json"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url"`
}

func HasImageConfig() bool {
	cfg := config.ConfigStruct.Image
	return strings.TrimSpace(cfg.BaseUrl) != "" && strings.TrimSpace(cfg.Model) != ""
}

func GenerateImage(ctx context.Context, prompt string) (ImageResult, error) {
	cfg := config.ConfigStruct.Image
	started := time.Now()
	loger.Loger.Info("[Image]开始请求生图", zap.String("endpoint", safeURLForLog(cfg.BaseUrl)), zap.String("model", cfg.Model), zap.String("size", cfg.Size), zap.String("response_format", cfg.ResponseFormat))
	if strings.TrimSpace(cfg.BaseUrl) == "" || strings.TrimSpace(cfg.Model) == "" {
		return ImageResult{}, errors.New("missing image generation config: image.baseUrl and image.model are required")
	}

	reqBody := imageRequest{
		Model:          cfg.Model,
		Prompt:         prompt,
		Size:           cfg.Size,
		N:              1,
		ResponseFormat: imageResponseFormatForRequest(cfg.ResponseFormat),
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return ImageResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.BaseUrl, bytes.NewReader(payload))
	if err != nil {
		return ImageResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ImageResult{}, fmt.Errorf("image generation http request failed after %s: %w", time.Since(started).Round(time.Second), err)
	}
	defer resp.Body.Close()
	loger.Loger.Info("[Image]生图接口已响应", zap.Int("status", resp.StatusCode), zap.Duration("duration", time.Since(started)))

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ImageResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body := limitString(string(data), 300)
		if shouldRetryImageWithMessages(resp.StatusCode, body) {
			loger.Loger.Warn("[Image]生图接口要求 messages 字段，尝试 messages 请求格式", zap.Int("status", resp.StatusCode), zap.String("body", body))
			return generateImageWithMessages(ctx, prompt, started)
		}
		return ImageResult{}, fmt.Errorf("image generation request failed: status=%d body=%s", resp.StatusCode, body)
	}

	item, err := parseImageDataItem(data)
	if err != nil {
		return ImageResult{}, err
	}
	result := ImageResult{URL: imageItemURL(item)}
	if item.B64JSON != "" {
		loger.Loger.Info("[Image]生图返回 base64", zap.Int("base64_len", len(item.B64JSON)))
		result.Bytes, err = base64.StdEncoding.DecodeString(item.B64JSON)
		if err != nil {
			return ImageResult{}, fmt.Errorf("decode image base64 failed: %w", err)
		}
	} else if result.URL != "" {
		loger.Loger.Info("[Image]生图返回 URL，开始下载", zap.String("image_url", safeURLForLog(result.URL)))
		result.Bytes, err = downloadImage(ctx, result.URL)
		if err != nil {
			return ImageResult{}, err
		}
	} else {
		return ImageResult{}, errors.New("image generation response has neither b64_json nor url")
	}

	path, err := writeGeneratedImage(prompt, result.Bytes, cfg.OutputDir)
	if err != nil {
		return ImageResult{}, fmt.Errorf("write generated image failed: %w", err)
	}
	result.Path = path
	loger.Loger.Info("[Image]生图完成", zap.String("path", path), zap.Int("bytes", len(result.Bytes)), zap.Duration("duration", time.Since(started)))
	return result, nil
}

func DryRunImage(prompt string) ImageResult {
	data, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lUTnWQAAAABJRU5ErkJggg==")
	return ImageResult{Path: "dry-run-placeholder.png", Bytes: data}
}

func generateImageWithMessages(ctx context.Context, prompt string, started time.Time) (ImageResult, error) {
	cfg := config.ConfigStruct.Image
	reqBody := imageChatRequest{
		Model: cfg.Model,
		Messages: []chatCompletionMessage{{
			Role:    "user",
			Content: prompt,
		}},
		Size:   cfg.Size,
		Stream: false,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return ImageResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.BaseUrl, bytes.NewReader(payload))
	if err != nil {
		return ImageResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ImageResult{}, fmt.Errorf("image generation messages request failed after %s: %w", time.Since(started).Round(time.Second), err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ImageResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ImageResult{}, fmt.Errorf("image generation messages request failed: status=%d body=%s", resp.StatusCode, limitString(string(data), 300))
	}

	item, err := parseImageDataItem(data)
	if err != nil {
		return ImageResult{}, err
	}
	result, err := imageResultFromItem(ctx, prompt, cfg.OutputDir, item)
	if err != nil {
		return ImageResult{}, err
	}
	loger.Loger.Info("[Image]messages 生图完成", zap.String("path", result.Path), zap.Int("bytes", len(result.Bytes)), zap.Duration("duration", time.Since(started)))
	return result, nil
}

func shouldRetryImageWithMessages(status int, body string) bool {
	if status < 400 || status >= 600 {
		return false
	}
	body = strings.ToLower(body)
	return strings.Contains(body, "messages is required") ||
		strings.Contains(body, "field messages is required") ||
		strings.Contains(body, "missing required parameter: 'messages'")
}

func parseImageDataItem(data []byte) (imageDataItem, error) {
	var parsed imageResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return imageDataItem{}, err
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return imageDataItem{}, errors.New(parsed.Error.Message)
	}
	if len(parsed.Data) > 0 {
		return parsed.Data[0], nil
	}
	if item, ok := parseChatImageDataItem(data); ok {
		return item, nil
	}
	return imageDataItem{}, errors.New("image generation response has no data")
}

func parseChatImageDataItem(data []byte) (imageDataItem, bool) {
	var parsed struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return imageDataItem{}, false
	}
	for _, choice := range parsed.Choices {
		if item, ok := imageItemFromRawContent(choice.Message.Content); ok {
			return item, true
		}
	}
	return imageDataItem{}, false
}

func imageItemFromRawContent(raw json.RawMessage) (imageDataItem, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return imageDataItem{}, false
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return imageItemFromText(text)
	}

	var contents []struct {
		Text     string `json:"text"`
		URL      string `json:"url"`
		ImageURL struct {
			URL string `json:"url"`
		} `json:"image_url"`
	}
	if err := json.Unmarshal(raw, &contents); err != nil {
		return imageDataItem{}, false
	}
	for _, content := range contents {
		if content.URL != "" {
			return imageDataItem{URL: content.URL}, true
		}
		if content.ImageURL.URL != "" {
			var item imageDataItem
			item.ImageURL.URL = content.ImageURL.URL
			return item, true
		}
		if item, ok := imageItemFromText(content.Text); ok {
			return item, true
		}
	}
	return imageDataItem{}, false
}

func imageItemFromText(text string) (imageDataItem, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return imageDataItem{}, false
	}
	if strings.HasPrefix(text, "{") {
		var parsed imageResponse
		if err := json.Unmarshal([]byte(text), &parsed); err == nil && len(parsed.Data) > 0 {
			return parsed.Data[0], true
		}
	}
	if url := firstHTTPURL(text); url != "" {
		return imageDataItem{URL: url}, true
	}
	return imageDataItem{}, false
}

func imageItemURL(item imageDataItem) string {
	if strings.TrimSpace(item.URL) != "" {
		return strings.TrimSpace(item.URL)
	}
	return strings.TrimSpace(item.ImageURL.URL)
}

func imageResultFromItem(ctx context.Context, prompt, outputDir string, item imageDataItem) (ImageResult, error) {
	var err error
	result := ImageResult{URL: imageItemURL(item)}
	if item.B64JSON != "" {
		loger.Loger.Info("[Image]生图返回 base64", zap.Int("base64_len", len(item.B64JSON)))
		result.Bytes, err = base64.StdEncoding.DecodeString(item.B64JSON)
		if err != nil {
			return ImageResult{}, fmt.Errorf("decode image base64 failed: %w", err)
		}
	} else if result.URL != "" {
		loger.Loger.Info("[Image]生图返回 URL，开始下载", zap.String("image_url", safeURLForLog(result.URL)))
		result.Bytes, err = downloadImage(ctx, result.URL)
		if err != nil {
			return ImageResult{}, err
		}
	} else {
		return ImageResult{}, errors.New("image generation response has neither b64_json nor url")
	}

	path, err := writeGeneratedImage(prompt, result.Bytes, outputDir)
	if err != nil {
		return ImageResult{}, fmt.Errorf("write generated image failed: %w", err)
	}
	result.Path = path
	return result, nil
}

func firstHTTPURL(text string) string {
	for _, marker := range []string{"https://", "http://"} {
		if idx := strings.Index(text, marker); idx >= 0 {
			rest := text[idx:]
			end := len(rest)
			for i, r := range rest {
				if i == 0 {
					continue
				}
				if strings.ContainsRune(" \t\r\n\"'<>[]{}(),，。；;）)", r) {
					end = i
					break
				}
			}
			return strings.TrimRight(rest[:end], ".!?。！？")
		}
	}
	return ""
}

func downloadImage(ctx context.Context, imageURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download generated image failed: status=%d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func writeGeneratedImage(prompt string, imageBytes []byte, outputDir string) (string, error) {
	if outputDir == "" {
		outputDir = "images"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", prompt, time.Now().UnixNano())))
	filename := hex.EncodeToString(sum[:])[:24] + imageExtension(imageBytes)
	path := filepath.Join(outputDir, filename)
	return path, os.WriteFile(path, imageBytes, 0644)
}

func imageExtension(imageBytes []byte) string {
	contentType := http.DetectContentType(imageBytes)
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}

func imageResponseFormatForRequest(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "omit", "none", "default", "auto", "b64_json":
		return ""
	default:
		return strings.TrimSpace(value)
	}
}

func safeURLForLog(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "invalid-url"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func limitString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
