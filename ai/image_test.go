package ai

import (
	"encoding/json"
	"testing"
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
