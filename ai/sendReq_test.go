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

func TestBuildReqBodyAddsChatCompletionsWebSearchByDefault(t *testing.T) {
	restoreAIConfig(t)
	config.ConfigStruct.Ai.BaseUrl = "https://example.com/v1/chat/completions"
	config.ConfigStruct.Ai.WebSearch = nil
	config.ConfigStruct.Ai.SearchContextSize = ""

	body, err := buildReqBody("test-model", []any{Messages[string]{Role: "system", Content: "system prompt"}})
	if err != nil {
		t.Fatalf("buildReqBody returned error: %v", err)
	}

	var got struct {
		Model            string `json:"model"`
		WebSearchOptions struct {
			SearchContextSize string `json:"search_context_size"`
		} `json:"web_search_options"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("request body json = %s, error: %v", string(body), err)
	}
	if got.Model != "test-model" {
		t.Fatalf("model = %q", got.Model)
	}
	if got.WebSearchOptions.SearchContextSize != "medium" {
		t.Fatalf("search_context_size = %q, want medium", got.WebSearchOptions.SearchContextSize)
	}
}

func TestBuildReqBodyOmitsChatCompletionsWebSearchWhenDisabled(t *testing.T) {
	restoreAIConfig(t)
	config.ConfigStruct.Ai.BaseUrl = "https://example.com/v1/chat/completions"
	config.ConfigStruct.Ai.WebSearch = testBoolPtr(false)

	body, err := buildReqBody("test-model", []any{Messages[string]{Role: "system", Content: "system prompt"}})
	if err != nil {
		t.Fatalf("buildReqBody returned error: %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("request body json = %s, error: %v", string(body), err)
	}
	if _, ok := got["web_search_options"]; ok {
		t.Fatalf("web_search_options should be omitted when webSearch is false: %s", string(body))
	}
}

func TestBuildReqBodyResponsesWebSearch(t *testing.T) {
	restoreAIConfig(t)
	config.ConfigStruct.Ai.BaseUrl = "https://example.com/v1/responses"
	config.ConfigStruct.Ai.WebSearch = testBoolPtr(true)
	config.ConfigStruct.Ai.ForceWebSearch = testBoolPtr(true)
	config.ConfigStruct.Ai.SearchContextSize = "high"

	image := Content{Type: "image_url"}
	image.ImgUrl.Url = "https://example.com/image.png"
	body, err := buildReqBody("test-model", []any{
		Messages[string]{Role: "system", Content: "system prompt"},
		Messages[[]Content]{Role: "user", Content: []Content{{Type: "text", Text: "hello"}, image}},
	})
	if err != nil {
		t.Fatalf("buildReqBody returned error: %v", err)
	}

	var got struct {
		Model        string `json:"model"`
		Instructions string `json:"instructions"`
		Input        []struct {
			Role    string `json:"role"`
			Content []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				ImageURL string `json:"image_url"`
			} `json:"content"`
		} `json:"input"`
		Tools []struct {
			Type              string `json:"type"`
			SearchContextSize string `json:"search_context_size"`
		} `json:"tools"`
		ToolChoice string `json:"tool_choice"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("request body json = %s, error: %v", string(body), err)
	}
	if got.Model != "test-model" {
		t.Fatalf("model = %q", got.Model)
	}
	if got.Instructions != "system prompt" {
		t.Fatalf("instructions = %q", got.Instructions)
	}
	if len(got.Input) != 1 || got.Input[0].Role != "user" {
		t.Fatalf("input roles = %+v", got.Input)
	}
	if len(got.Input[0].Content) != 2 || got.Input[0].Content[0].Type != "input_text" || got.Input[0].Content[1].Type != "input_image" {
		t.Fatalf("input content = %+v", got.Input[0].Content)
	}
	if got.Input[0].Content[1].ImageURL != "https://example.com/image.png" {
		t.Fatalf("image_url = %q", got.Input[0].Content[1].ImageURL)
	}
	if len(got.Tools) != 1 || got.Tools[0].Type != responsesWebSearchToolType || got.Tools[0].SearchContextSize != "high" {
		t.Fatalf("tools = %+v", got.Tools)
	}
	if got.ToolChoice != "required" {
		t.Fatalf("tool_choice = %q", got.ToolChoice)
	}
}

func TestSendReqResponsesFallsBackToCompatPayload(t *testing.T) {
	restoreAIConfig(t)
	oldLogger := loger.Loger
	loger.Loger = zap.NewNop()
	t.Cleanup(func() { loger.Loger = oldLogger })
	config.ConfigStruct.Ai.WebSearch = testBoolPtr(true)
	config.ConfigStruct.Ai.SearchContextSize = "medium"

	attempts := 0
	var bodies []map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		attempts++
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"message":"openai_error","type":"bad_response_status_code"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"output_text":"compat-ok","usage":{"total_tokens":9}}`))
	}))
	defer server.Close()

	config.ConfigStruct.Ai.BaseUrl = server.URL + "/v1/responses"
	resp := SendReq("test-model", []any{
		Messages[string]{Role: "system", Content: "system prompt"},
		Messages[string]{Role: "user", Content: "hello"},
	})
	if len(resp.Choices) != 1 || resp.Choices[0].Msg.Content != "compat-ok" {
		t.Fatalf("response = %+v", resp)
	}
	if attempts != 2 || len(bodies) != 2 {
		t.Fatalf("attempts = %d, bodies = %d", attempts, len(bodies))
	}

	var first struct {
		Instructions string `json:"instructions"`
		Tools        []struct {
			Type string `json:"type"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(mustRawMessage(t, bodies[0], "tools"), &first.Tools); err != nil {
		t.Fatalf("first tools: %v", err)
	}
	_ = json.Unmarshal(bodies[0]["instructions"], &first.Instructions)
	if first.Instructions != "system prompt" || len(first.Tools) != 1 || first.Tools[0].Type != responsesWebSearchToolType {
		t.Fatalf("first body instructions=%q tools=%+v", first.Instructions, first.Tools)
	}

	var second struct {
		Input []struct {
			Role string `json:"role"`
		} `json:"input"`
		Tools []struct {
			Type string `json:"type"`
		} `json:"tools"`
	}
	if _, ok := bodies[1]["instructions"]; ok {
		t.Fatalf("compat body should not include instructions: %s", string(bodies[1]["instructions"]))
	}
	if err := json.Unmarshal(mustRawMessage(t, bodies[1], "input"), &second.Input); err != nil {
		t.Fatalf("compat input: %v", err)
	}
	if err := json.Unmarshal(mustRawMessage(t, bodies[1], "tools"), &second.Tools); err != nil {
		t.Fatalf("compat tools: %v", err)
	}
	if len(second.Input) != 2 || second.Input[0].Role != "developer" || second.Input[1].Role != "user" {
		t.Fatalf("compat input = %+v", second.Input)
	}
	if len(second.Tools) != 1 || second.Tools[0].Type != legacyResponsesWebSearchToolType {
		t.Fatalf("compat tools = %+v", second.Tools)
	}
}

func TestSendReqEmptyModelReturnsWithoutFatal(t *testing.T) {
	oldLogger := loger.Loger
	loger.Loger = zap.NewNop()
	t.Cleanup(func() { loger.Loger = oldLogger })

	resp := SendReq("", []any{Messages[string]{Role: "user", Content: "hello"}})
	if len(resp.Choices) != 0 {
		t.Fatalf("SendReq empty model returned choices: %+v", resp.Choices)
	}
}

func TestSendReqChatCompletionsRetriesStreamWhenEmpty(t *testing.T) {
	restoreAIConfig(t)
	oldLogger := loger.Loger
	loger.Loger = zap.NewNop()
	t.Cleanup(func() { loger.Loger = oldLogger })
	config.ConfigStruct.Ai.WebSearch = testBoolPtr(false)

	attempts := 0
	var streams []bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		attempts++
		var body struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		streams = append(streams, body.Stream)
		w.Header().Set("Content-Type", "text/event-stream")
		if !body.Stream {
			_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"total_tokens\":12}}\n\ndata: [DONE]\n\n"))
			return
		}
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"stream ok\"}}],\"usage\":{\"total_tokens\":13}}\n\ndata: [DONE]\n\n"))
	}))
	defer server.Close()

	config.ConfigStruct.Ai.BaseUrl = server.URL + "/v1/chat/completions"
	resp := SendReq("test-model", []any{Messages[string]{Role: "user", Content: "hello"}})
	if len(resp.Choices) != 1 || resp.Choices[0].Msg.Content != "stream ok" {
		t.Fatalf("response = %+v", resp)
	}
	if attempts != 2 || len(streams) != 2 || streams[0] || !streams[1] {
		t.Fatalf("attempts=%d streams=%v, want [false true]", attempts, streams)
	}

	resp = SendReq("test-model", []any{Messages[string]{Role: "user", Content: "hello again"}})
	if len(resp.Choices) != 1 || resp.Choices[0].Msg.Content != "stream ok" {
		t.Fatalf("cached response = %+v", resp)
	}
	if attempts != 3 || len(streams) != 3 || !streams[2] {
		t.Fatalf("cached attempts=%d streams=%v, want third stream=true", attempts, streams)
	}
}

func TestSendReqRetriesWithoutImagesOnFileDownloadFailure(t *testing.T) {
	restoreAIConfig(t)
	oldLogger := loger.Loger
	loger.Loger = zap.NewNop()
	t.Cleanup(func() { loger.Loger = oldLogger })
	config.ConfigStruct.Ai.WebSearch = testBoolPtr(false)

	image := Content{Type: "image_url"}
	image.ImgUrl.Url = "https://img.example.com/blocked.jpg"

	type requestBody struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		Stream bool `json:"stream"`
	}

	attempts := 0
	var bodies []requestBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		attempts++
		var body requestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"error getting file type: failed to download file, status code: 403","code":"count_token_failed"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"text ok"}}],"usage":{"total_tokens":11}}`))
	}))
	defer server.Close()

	config.ConfigStruct.Ai.BaseUrl = server.URL + "/v1/chat/completions"
	resp := SendReq("test-model", []any{
		Messages[[]Content]{Role: "user", Content: []Content{{Type: "text", Text: "hello"}, image}},
	})
	if len(resp.Choices) != 1 || resp.Choices[0].Msg.Content != "text ok" {
		t.Fatalf("response = %+v", resp)
	}
	if attempts != 2 || len(bodies) != 2 {
		t.Fatalf("attempts=%d bodies=%d, want 2", attempts, len(bodies))
	}

	var firstContent []Content
	if err := json.Unmarshal(bodies[0].Messages[0].Content, &firstContent); err != nil {
		t.Fatalf("first content: %v", err)
	}
	if len(firstContent) != 2 || firstContent[1].Type != "image_url" {
		t.Fatalf("first content = %+v, want text+image", firstContent)
	}

	var secondContent []Content
	if err := json.Unmarshal(bodies[1].Messages[0].Content, &secondContent); err != nil {
		t.Fatalf("second content: %v", err)
	}
	if len(secondContent) != 1 || secondContent[0].Type != "text" || secondContent[0].Text != "hello" {
		t.Fatalf("second content = %+v, want text only", secondContent)
	}
}

func TestSendChatCompletionResponsesFallsBackToCompatPayload(t *testing.T) {
	restoreAIConfig(t)

	attempts := 0
	var bodies []map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		attempts++
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"message":"openai_error","type":"bad_response_status_code"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"output_text":"route-ok","usage":{"total_tokens":4}}`))
	}))
	defer server.Close()

	config.ConfigStruct.Ai.BaseUrl = server.URL + "/v1/responses"
	got, err := sendChatCompletion(context.Background(), "route-model", []chatCompletionMessage{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "hello"},
	})
	if err != nil {
		t.Fatalf("sendChatCompletion returned error: %v", err)
	}
	if got != "route-ok" {
		t.Fatalf("response text = %q, want route-ok", got)
	}
	if attempts != 2 || len(bodies) != 2 {
		t.Fatalf("attempts = %d, bodies = %d", attempts, len(bodies))
	}
	if _, ok := bodies[1]["instructions"]; ok {
		t.Fatalf("compat body should not include instructions: %s", string(bodies[1]["instructions"]))
	}
	var input []struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(mustRawMessage(t, bodies[1], "input"), &input); err != nil {
		t.Fatalf("compat input: %v", err)
	}
	if len(input) != 2 || input[0].Role != "developer" || input[1].Role != "user" {
		t.Fatalf("compat input = %+v", input)
	}
}

func TestSendChatCompletionRetriesStreamWhenEmpty(t *testing.T) {
	restoreAIConfig(t)

	attempts := 0
	var streams []bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		attempts++
		var body struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		streams = append(streams, body.Stream)
		w.Header().Set("Content-Type", "text/event-stream")
		if !body.Stream {
			_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"total_tokens\":12}}\n\ndata: [DONE]\n\n"))
			return
		}
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"stream ok\"}}],\"usage\":{\"total_tokens\":13}}\n\ndata: [DONE]\n\n"))
	}))
	defer server.Close()

	config.ConfigStruct.Ai.BaseUrl = server.URL + "/v1/chat/completions"
	got, err := sendChatCompletion(context.Background(), "route-model", []chatCompletionMessage{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("sendChatCompletion returned error: %v", err)
	}
	if got != "stream ok" {
		t.Fatalf("response text = %q, want stream ok", got)
	}
	if attempts != 2 || len(streams) != 2 || streams[0] || !streams[1] {
		t.Fatalf("attempts=%d streams=%v, want [false true]", attempts, streams)
	}

	got, err = sendChatCompletion(context.Background(), "route-model", []chatCompletionMessage{{Role: "user", Content: "hello again"}})
	if err != nil {
		t.Fatalf("cached sendChatCompletion returned error: %v", err)
	}
	if got != "stream ok" {
		t.Fatalf("cached response text = %q, want stream ok", got)
	}
	if attempts != 3 || len(streams) != 3 || !streams[2] {
		t.Fatalf("cached attempts=%d streams=%v, want third stream=true", attempts, streams)
	}
}

func TestSendChatCompletionCachedStreamFallsBackToNonStream(t *testing.T) {
	restoreAIConfig(t)

	attempts := 0
	var streams []bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		attempts++
		var body struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		streams = append(streams, body.Stream)
		w.Header().Set("Content-Type", "application/json")
		if body.Stream {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"stream unavailable"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"non-stream ok"}}],"usage":{"total_tokens":7}}`))
	}))
	defer server.Close()

	config.ConfigStruct.Ai.BaseUrl = server.URL + "/v1/chat/completions"
	cacheKey := chatCompletionsCacheKey(config.ConfigStruct.Ai.BaseUrl, "route-model")
	chatCompletionsModeCache.Store(cacheKey, chatCompletionsModeStream)

	got, err := sendChatCompletion(context.Background(), "route-model", []chatCompletionMessage{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("sendChatCompletion returned error: %v", err)
	}
	if got != "non-stream ok" {
		t.Fatalf("response text = %q, want non-stream ok", got)
	}
	if attempts != 2 || len(streams) != 2 || !streams[0] || streams[1] {
		t.Fatalf("attempts=%d streams=%v, want [true false]", attempts, streams)
	}
	if mode := chatCompletionsCachedMode(cacheKey); mode != chatCompletionsModeDefault {
		t.Fatalf("cached mode = %q, want default", mode)
	}
}

func mustRawMessage(t *testing.T, body map[string]json.RawMessage, key string) json.RawMessage {
	t.Helper()
	value, ok := body[key]
	if !ok {
		t.Fatalf("missing %q in body: %+v", key, body)
	}
	return value
}

func TestParseResponsesResp(t *testing.T) {
	resp, err := parseResponsesResp([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"hello"},{"type":"output_text","text":"world"}]}],"usage":{"total_tokens":42}}`))
	if err != nil {
		t.Fatalf("parseResponsesResp returned error: %v", err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Msg.Content != "hello\nworld" {
		t.Fatalf("content = %+v", resp.Choices)
	}
	if resp.Usage.TotalToken != 42 {
		t.Fatalf("total_tokens = %d", resp.Usage.TotalToken)
	}

	resp, err = parseResponsesResp([]byte(`{"output_text":"direct","usage":{"total_tokens":7}}`))
	if err != nil {
		t.Fatalf("parseResponsesResp output_text returned error: %v", err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Msg.Content != "direct" || resp.Usage.TotalToken != 7 {
		t.Fatalf("response = %+v", resp)
	}
}

func TestParseAITextResponseChatCompletionsSSE(t *testing.T) {
	body := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}],\"usage\":{\"total_tokens\":1}}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}],\"usage\":{\"total_tokens\":2}}\n\n" +
		"data: [DONE]\n\n")
	text, tokens, err := ParseAITextResponse(body, false)
	if err != nil {
		t.Fatalf("ParseAITextResponse returned error: %v", err)
	}
	if text != "hello world" {
		t.Fatalf("text = %q, want hello world", text)
	}
	if tokens != 2 {
		t.Fatalf("tokens = %d, want 2", tokens)
	}
}

func TestParseAITextResponseChatCompletionsSSEUsageOnly(t *testing.T) {
	body := []byte("data: {\"id\":\"\",\"object\":\"chat.completion.chunk\",\"choices\":[],\"usage\":{\"total_tokens\":4528}}\n\n" +
		"data: [DONE]\n\n")
	text, tokens, err := ParseAITextResponse(body, false)
	if err != nil {
		t.Fatalf("ParseAITextResponse returned error: %v", err)
	}
	if text != "" {
		t.Fatalf("text = %q, want empty", text)
	}
	if tokens != 4528 {
		t.Fatalf("tokens = %d, want 4528", tokens)
	}
}

func TestParseAITextResponseResponsesSSE(t *testing.T) {
	body := []byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\" world\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"total_tokens\":9}}}\n\n" +
		"data: [DONE]\n\n")
	text, tokens, err := ParseAITextResponse(body, true)
	if err != nil {
		t.Fatalf("ParseAITextResponse returned error: %v", err)
	}
	if text != "hello world" {
		t.Fatalf("text = %q, want hello world", text)
	}
	if tokens != 9 {
		t.Fatalf("tokens = %d, want 9", tokens)
	}
}

func TestSendChatCompletionUsesResponsesInput(t *testing.T) {
	restoreAIConfig(t)

	type requestBody struct {
		Model        string `json:"model"`
		Instructions string `json:"instructions"`
		Input        []struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"input"`
		Stream bool `json:"stream"`
	}
	type requestResult struct {
		body requestBody
		err  error
	}

	resultCh := make(chan requestResult, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var body requestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			resultCh <- requestResult{err: err}
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		resultCh <- requestResult{body: body}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"route-ok","usage":{"total_tokens":3}}`))
	}))
	defer server.Close()

	config.ConfigStruct.Ai.BaseUrl = server.URL + "/v1/responses"
	got, err := sendChatCompletion(context.Background(), "route-model", []chatCompletionMessage{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "hello"},
	})
	if err != nil {
		t.Fatalf("sendChatCompletion returned error: %v", err)
	}
	if got != "route-ok" {
		t.Fatalf("response text = %q, want route-ok", got)
	}

	result := <-resultCh
	if result.err != nil {
		t.Fatalf("decode request body: %v", result.err)
	}
	if result.body.Model != "route-model" {
		t.Fatalf("model = %q", result.body.Model)
	}
	if result.body.Instructions != "system prompt" {
		t.Fatalf("instructions = %q", result.body.Instructions)
	}
	if len(result.body.Input) != 1 || result.body.Input[0].Role != "user" {
		t.Fatalf("input roles = %+v", result.body.Input)
	}
	if len(result.body.Input[0].Content) != 1 || result.body.Input[0].Content[0].Type != "input_text" || result.body.Input[0].Content[0].Text != "hello" {
		t.Fatalf("user content = %+v", result.body.Input[0].Content)
	}
}

func restoreAIConfig(t *testing.T) {
	t.Helper()
	resetChatCompletionsModeCacheForTest()
	oldModel := config.ConfigStruct.Ai.Model
	oldPrompt := config.ConfigStruct.Ai.Prompt
	oldBaseURL := config.ConfigStruct.Ai.BaseUrl
	oldToken := config.ConfigStruct.Ai.Token
	oldWebSearch := config.ConfigStruct.Ai.WebSearch
	oldForceWebSearch := config.ConfigStruct.Ai.ForceWebSearch
	oldSearchContextSize := config.ConfigStruct.Ai.SearchContextSize
	t.Cleanup(func() {
		config.ConfigStruct.Ai.Model = oldModel
		config.ConfigStruct.Ai.Prompt = oldPrompt
		config.ConfigStruct.Ai.BaseUrl = oldBaseURL
		config.ConfigStruct.Ai.Token = oldToken
		config.ConfigStruct.Ai.WebSearch = oldWebSearch
		config.ConfigStruct.Ai.ForceWebSearch = oldForceWebSearch
		config.ConfigStruct.Ai.SearchContextSize = oldSearchContextSize
		resetChatCompletionsModeCacheForTest()
	})
}

func testBoolPtr(v bool) *bool {
	return &v
}
