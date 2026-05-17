package xhh

import (
	"html"
	"regexp"
	"strings"
)

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

func ExtractImagePrompt(text string) (string, bool) {
	cleaned := html.UnescapeString(htmlTagPattern.ReplaceAllString(text, " "))
	keywords := []string{"生成图片", "生图", "画图"}
	for _, keyword := range keywords {
		idx := strings.Index(cleaned, keyword)
		if idx < 0 {
			continue
		}
		prompt := cleaned[idx+len(keyword):]
		prompt = strings.TrimSpace(prompt)
		prompt = strings.TrimLeft(prompt, "：:，,。.!！、 ")
		prompt = strings.TrimSpace(prompt)
		if prompt == "" {
			return "", false
		}
		return prompt, true
	}
	return "", false
}
