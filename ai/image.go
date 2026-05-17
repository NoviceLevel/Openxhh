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
	"os"
	"path/filepath"
	"strings"
	"time"
	"xhhrobot/config"
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

type imageResponse struct {
	Data []struct {
		URL     string `json:"url"`
		B64JSON string `json:"b64_json"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func HasImageConfig() bool {
	cfg := config.ConfigStruct.Image
	return strings.TrimSpace(cfg.BaseUrl) != "" && strings.TrimSpace(cfg.Model) != ""
}

func GenerateImage(ctx context.Context, prompt string) (ImageResult, error) {
	cfg := config.ConfigStruct.Image
	if strings.TrimSpace(cfg.BaseUrl) == "" || strings.TrimSpace(cfg.Model) == "" {
		return ImageResult{}, errors.New("missing image generation config: image.baseUrl and image.model are required")
	}

	reqBody := imageRequest{
		Model:          cfg.Model,
		Prompt:         prompt,
		Size:           cfg.Size,
		N:              1,
		ResponseFormat: cfg.ResponseFormat,
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
		return ImageResult{}, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ImageResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ImageResult{}, fmt.Errorf("image generation request failed: status=%d body=%s", resp.StatusCode, limitString(string(data), 300))
	}

	var parsed imageResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return ImageResult{}, err
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return ImageResult{}, errors.New(parsed.Error.Message)
	}
	if len(parsed.Data) == 0 {
		return ImageResult{}, errors.New("image generation response has no data")
	}

	item := parsed.Data[0]
	result := ImageResult{URL: item.URL}
	if item.B64JSON != "" {
		result.Bytes, err = base64.StdEncoding.DecodeString(item.B64JSON)
		if err != nil {
			return ImageResult{}, err
		}
	} else if item.URL != "" {
		result.Bytes, err = downloadImage(ctx, item.URL)
		if err != nil {
			return ImageResult{}, err
		}
	} else {
		return ImageResult{}, errors.New("image generation response has neither b64_json nor url")
	}

	path, err := writeGeneratedImage(prompt, result.Bytes, cfg.OutputDir)
	if err != nil {
		return ImageResult{}, err
	}
	result.Path = path
	return result, nil
}

func DryRunImage(prompt string) ImageResult {
	data, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lUTnWQAAAABJRU5ErkJggg==")
	return ImageResult{Path: "dry-run-placeholder.png", Bytes: data}
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

func limitString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
