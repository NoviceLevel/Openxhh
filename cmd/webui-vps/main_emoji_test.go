package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCollectXHHEmojisAddsGroupCodeAliases(t *testing.T) {
	const zombieURL = "https://imgheybox.max-c.com/heybox/emoji/cube_95.png"
	groups := []any{
		map[string]any{
			"group_code": "cube",
			"emojis": []any{
				map[string]any{"code": "僵尸", "img": zombieURL},
			},
		},
	}

	emojis := map[string]string{}
	collectXHHEmojis(groups, emojis)

	for _, key := range []string{"僵尸", "[僵尸]", "cube_僵尸", "[cube_僵尸]"} {
		if emojis[key] != zombieURL {
			t.Fatalf("emojis[%q] = %q, want %q", key, emojis[key], zombieURL)
		}
	}
}

func TestCollectXHHEmojisDoesNotDoublePrefix(t *testing.T) {
	const likeURL = "https://imgheybox.max-c.com/heybox/emoji/cube_14.png"
	groups := []any{
		map[string]any{
			"group_code": "cube",
			"emojis": []any{
				map[string]any{"code": "cube_喜欢", "img": likeURL},
			},
		},
	}

	emojis := map[string]string{}
	collectXHHEmojis(groups, emojis)

	if emojis["cube_喜欢"] != likeURL {
		t.Fatalf("emojis[cube_喜欢] = %q, want %q", emojis["cube_喜欢"], likeURL)
	}
	if _, ok := emojis["cube_cube_喜欢"]; ok {
		t.Fatal("unexpected cube_cube_喜欢 alias")
	}
}

func TestCachedXHHEmojiLibraryUsesFreshCache(t *testing.T) {
	state := &serverState{
		emojiCache:        map[string]string{"cube_喜欢": "https://example.com/like.png"},
		emojiCacheVersion: "v1",
		emojiCacheUntil:   time.Now().Add(time.Hour),
	}
	originalFetch := fetchEmojiLibrary
	t.Cleanup(func() { fetchEmojiLibrary = originalFetch })
	fetchEmojiLibrary = func(context.Context, appConfig, xhhSession) (map[string]string, string, error) {
		t.Fatal("fresh cache should avoid fetching")
		return nil, "", nil
	}

	emojis, version, warning, err := state.cachedXHHEmojiLibrary(context.Background(), appConfig{}, xhhSession{}, time.Now())
	if err != nil {
		t.Fatalf("cachedXHHEmojiLibrary returned error: %v", err)
	}
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
	if version != "v1" {
		t.Fatalf("version = %q, want v1", version)
	}
	if emojis["cube_喜欢"] != "https://example.com/like.png" {
		t.Fatalf("cached emoji = %q", emojis["cube_喜欢"])
	}
	emojis["cube_喜欢"] = "changed"
	if state.emojiCache["cube_喜欢"] != "https://example.com/like.png" {
		t.Fatal("cached map was mutated by caller")
	}
}

func TestCachedXHHEmojiLibraryFallsBackToStaleCache(t *testing.T) {
	state := &serverState{
		emojiCache:        map[string]string{"cube_哭": "https://example.com/cry.png"},
		emojiCacheVersion: "old",
		emojiCacheUntil:   time.Now().Add(-time.Hour),
	}
	originalFetch := fetchEmojiLibrary
	t.Cleanup(func() { fetchEmojiLibrary = originalFetch })
	fetchEmojiLibrary = func(context.Context, appConfig, xhhSession) (map[string]string, string, error) {
		return nil, "", errors.New("network down")
	}

	emojis, version, warning, err := state.cachedXHHEmojiLibrary(context.Background(), appConfig{}, xhhSession{}, time.Now())
	if err != nil {
		t.Fatalf("cachedXHHEmojiLibrary returned error: %v", err)
	}
	if version != "old" {
		t.Fatalf("version = %q, want old", version)
	}
	if warning != "network down" {
		t.Fatalf("warning = %q, want network down", warning)
	}
	if emojis["cube_哭"] != "https://example.com/cry.png" {
		t.Fatalf("cached emoji = %q", emojis["cube_哭"])
	}
}

func TestCachedXHHEmojiLibraryReturnsFetchErrorWithoutCache(t *testing.T) {
	state := &serverState{}
	originalFetch := fetchEmojiLibrary
	t.Cleanup(func() { fetchEmojiLibrary = originalFetch })
	fetchEmojiLibrary = func(context.Context, appConfig, xhhSession) (map[string]string, string, error) {
		return nil, "", errors.New("network down")
	}

	emojis, version, warning, err := state.cachedXHHEmojiLibrary(context.Background(), appConfig{}, xhhSession{}, time.Now())
	if err == nil {
		t.Fatal("expected fetch error")
	}
	if emojis != nil || version != "" || warning != "" {
		t.Fatalf("unexpected fallback values: emojis=%v version=%q warning=%q", emojis, version, warning)
	}
}

func TestCachedXHHEmojiLibraryStoresSuccessfulFetch(t *testing.T) {
	state := &serverState{}
	now := time.Now()
	originalFetch := fetchEmojiLibrary
	t.Cleanup(func() { fetchEmojiLibrary = originalFetch })
	fetchEmojiLibrary = func(context.Context, appConfig, xhhSession) (map[string]string, string, error) {
		return map[string]string{"cube_笑": "https://example.com/smile.png"}, "new", nil
	}

	emojis, version, warning, err := state.cachedXHHEmojiLibrary(context.Background(), appConfig{}, xhhSession{}, now)
	if err != nil {
		t.Fatalf("cachedXHHEmojiLibrary returned error: %v", err)
	}
	if warning != "" || version != "new" {
		t.Fatalf("version=%q warning=%q", version, warning)
	}
	if emojis["cube_笑"] != "https://example.com/smile.png" {
		t.Fatalf("emoji = %q", emojis["cube_笑"])
	}
	if state.emojiCacheVersion != "new" || !state.emojiCacheUntil.Equal(now.Add(xhhEmojiCacheTTL)) {
		t.Fatalf("cache metadata not stored: version=%q until=%v", state.emojiCacheVersion, state.emojiCacheUntil)
	}
	if state.emojiCache["cube_笑"] != "https://example.com/smile.png" {
		t.Fatalf("stored emoji = %q", state.emojiCache["cube_笑"])
	}
}

func TestParseFailedRecordsRemovesAutomaticRetrySuccess(t *testing.T) {
	content := strings.Join([]string{
		`2026-06-04 01:05:00 INFO [XHH]正在处理@消息 {"msg_id":3881692903,"comment_id":880649280,"link_id":182683946,"user_id":44777403,"user_name":"小金鱼鸭子","text":"可以转人工嘛","raw_text":"可以转人工嘛"}`,
		`2026-06-04 01:05:01 INFO [Ai]正在询问Ai {"msg_id":3881692903,"comment_id":880649280,"link_id":182683946,"user_id":44777403,"user_name":"小金鱼鸭子","question":"可以转人工嘛","raw_question":"可以转人工嘛"}`,
		`2026-06-04 01:05:02 ERROR [XHH]无法回复评论，将重试 {"msg_id":3881692903,"comment_id":880649280,"link_id":182683946,"user_id":44777403,"user_name":"小金鱼鸭子","question":"可以转人工嘛"}`,
		`2026-06-04 01:19:07 INFO [XHH]正在处理@消息 {"msg_id":3881692903,"comment_id":880649280,"link_id":182683946,"user_id":44777403,"user_name":"小金鱼鸭子","text":"可以转人工嘛","raw_text":"可以转人工嘛"}`,
		`2026-06-04 01:19:07 INFO [Ai]Ai说： {"text":"何事？本大人可不是客服。","本次消耗token":1943,"msg_id":3881692903,"comment_id":880649280,"link_id":182683946,"user_id":44777403,"user_name":"小金鱼鸭子","question":"可以转人工嘛","raw_question":"可以转人工嘛"}`,
	}, "\n")

	records := parseFailedRecords(content, nil)
	if len(records) != 0 {
		t.Fatalf("parseFailedRecords returned %d records, want 0: %+v", len(records), records)
	}
}
