package main

import "testing"

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
