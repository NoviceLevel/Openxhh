package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"openxhh/config"
	"strings"
)

type chatCompletionMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func sendChatCompletion(ctx context.Context, model string, messages []chatCompletionMessage) (string, error) {
	if strings.TrimSpace(model) == "" {
		return "", errors.New("model is empty")
	}
	body := struct {
		Model    string                  `json:"model"`
		Messages []chatCompletionMessage `json:"messages"`
		Stream   bool                    `json:"stream"`
	}{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", config.ConfigStruct.Ai.BaseUrl, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+config.ConfigStruct.Ai.Token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("chat completion request failed: status=%d body=%s", resp.StatusCode, limitRefineString(string(data), 300))
	}

	var parsed promptRefineResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", errors.New("chat completion response has no content")
	}
	return parsed.Choices[0].Message.Content, nil
}
