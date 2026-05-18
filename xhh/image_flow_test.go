package xhh

import "testing"

func TestAppendUniqueMention(t *testing.T) {
	trigger := buildMention(1001, "触发者")
	target := buildMention(2002, "目标用户")

	mentions := appendUniqueMention(nil, trigger)
	mentions = appendUniqueMention(mentions, target)
	if got := len(mentions); got != 2 {
		t.Fatalf("len(mentions) = %d, want 2", got)
	}

	mentions = appendUniqueMention(mentions, buildMention(1001, "触发者新名字"))
	if got := len(mentions); got != 2 {
		t.Fatalf("len(mentions) after duplicate = %d, want 2", got)
	}
}

func TestExtractMentionUserID(t *testing.T) {
	if got := extractMentionUserID(buildMention(1001, "触发者")); got != "1001" {
		t.Fatalf("extractMentionUserID = %q, want 1001", got)
	}
}

func TestNormalizeImageReplyText(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{name: "normal reply", text: " 图来啦，快看看喵 ", want: "图来啦，快看看喵"},
		{name: "remove quotes and newlines", text: "“图片来啦\n快看看”", want: "图片来啦 快看看"},
		{name: "hide generated prompt", text: "已生成：一只猫", want: defaultImageReplyText},
		{name: "hide prompt word", text: "prompt 已经处理好了", want: defaultImageReplyText},
		{name: "empty reply", text: "   ", want: defaultImageReplyText},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeImageReplyText(tt.text); got != tt.want {
				t.Fatalf("normalizeImageReplyText(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}
