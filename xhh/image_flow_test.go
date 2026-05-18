package xhh

import "testing"

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
