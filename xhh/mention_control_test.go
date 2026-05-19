package xhh

import "testing"

func TestParseMentionControl(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantCleaned string
		wantTarget  string
	}{
		{
			name:        "give him see mention",
			text:        "帮我生成一只猫并给他看@张三 @小猫娘喵喵",
			wantCleaned: "帮我生成一只猫",
			wantTarget:  "张三",
		},
		{
			name:        "mention target with at",
			text:        "@小猫娘喵喵 并艾特@张三看看",
			wantCleaned: "",
			wantTarget:  "张三",
		},
		{
			name:        "robot wake only",
			text:        "@小猫娘喵喵 你怎么看",
			wantCleaned: "你怎么看",
			wantTarget:  "",
		},
		{
			name:        "target before robot mention",
			text:        "帮我生成一张图，并艾特小明看@小猫娘喵喵",
			wantCleaned: "帮我生成一张图",
			wantTarget:  "小明",
		},
		{
			name:        "html mention target",
			text:        `@机器人 并给他看<a data-user-id="1">@张三</a>`,
			wantCleaned: "",
			wantTarget:  "张三",
		},
		{
			name:        "target contains me inside username",
			text:        "生成一只猫，并艾特麻溜转我五块查看",
			wantCleaned: "生成一只猫",
			wantTarget:  "麻溜转我五块",
		},
		{
			name:        "do not treat me as target",
			text:        "@机器人 告诉我这个是什么意思",
			wantCleaned: "告诉我这个是什么意思",
			wantTarget:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMentionControl(tt.text)
			if got.CleanedText != tt.wantCleaned {
				t.Fatalf("CleanedText = %q, want %q", got.CleanedText, tt.wantCleaned)
			}
			if got.TargetText != tt.wantTarget {
				t.Fatalf("TargetText = %q, want %q", got.TargetText, tt.wantTarget)
			}
		})
	}
}
