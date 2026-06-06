package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"openxhh/config"
	"openxhh/loger"
	"testing"

	"go.uber.org/zap"
)

func TestImageResponseFormatForRequestOmitsLegacyDefault(t *testing.T) {
	for _, value := range []string{"", "b64_json", "omit", "none", "default", "auto"} {
		if got := imageResponseFormatForRequest(value); got != "" {
			t.Fatalf("imageResponseFormatForRequest(%q) = %q, want empty", value, got)
		}
	}
}

func TestImageRequestOmitsEmptyResponseFormat(t *testing.T) {
	body, err := json.Marshal(imageRequest{
		Model:          "image-model",
		Prompt:         "test prompt",
		ResponseFormat: imageResponseFormatForRequest("b64_json"),
	})
	if err != nil {
		t.Fatalf("marshal image request: %v", err)
	}
	if string(body) != `{"model":"image-model","prompt":"test prompt"}` {
		t.Fatalf("body = %s", string(body))
	}
}

func TestImageResponseFormatForRequestKeepsExplicitURL(t *testing.T) {
	if got := imageResponseFormatForRequest("url"); got != "url" {
		t.Fatalf("imageResponseFormatForRequest(url) = %q", got)
	}
}

func TestGenerateImageRetriesWithMessagesWhenRequired(t *testing.T) {
	oldImage := config.ConfigStruct.Image
	oldLogger := loger.Loger
	t.Cleanup(func() {
		config.ConfigStruct.Image = oldImage
		loger.Loger = oldLogger
	})
	loger.Loger = zap.NewNop()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		defer r.Body.Close()
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if attempts == 1 {
			if _, ok := body["prompt"]; !ok {
				t.Fatalf("first request should use prompt body: %s", string(body["messages"]))
			}
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"field messages is required"}}`))
			return
		}
		if _, ok := body["messages"]; !ok {
			t.Fatalf("second request should use messages body: %+v", body)
		}
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lUTnWQAAAABJRU5ErkJggg=="}]}`))
	}))
	defer server.Close()

	config.ConfigStruct.Image.BaseUrl = server.URL
	config.ConfigStruct.Image.Model = "image-model"
	config.ConfigStruct.Image.Size = "1024x1024"
	config.ConfigStruct.Image.OutputDir = t.TempDir()

	result, err := GenerateImage(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if len(result.Bytes) == 0 || result.Path == "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestParseImageDataItemAcceptsChatImageURL(t *testing.T) {
	item, err := parseImageDataItem([]byte(`{"choices":[{"message":{"content":"generated: https://example.com/image.png"}}]}`))
	if err != nil {
		t.Fatalf("parseImageDataItem returned error: %v", err)
	}
	if got := imageItemURL(item); got != "https://example.com/image.png" {
		t.Fatalf("image url = %q", got)
	}
}
