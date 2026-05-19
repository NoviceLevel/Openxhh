package main

import (
	"flag"
	"fmt"
	"openxhh/config"
	"openxhh/loger"
	"openxhh/xhh"
	"os"
)

func main() {
	commentID := flag.Int("comment_id", 0, "trigger comment id")
	linkID := flag.Int("link_id", 0, "link id")
	rootID := flag.Int("root_id", 0, "root comment id")
	userID := flag.Int("userid", 0, "trigger user id")
	text := flag.String("text", "", "trigger comment text")
	mockImage := flag.Bool("mock_image", true, "use a placeholder image instead of calling image API")
	flag.Parse()

	if *commentID == 0 || *linkID == 0 || *userID == 0 || *text == "" {
		fmt.Println("usage: go run ./cmd/dry_run_image_comment -comment_id 123 -link_id 181099114 -root_id 123 -userid 456 -text \"@机器人 生图 一只猫\"")
		os.Exit(1)
	}

	loger.InitLog()
	config.InitConfig()
	xhh.Init()

	mentionControl := xhh.ParseMentionControl(*text)
	result := xhh.ProcessImageGenerationComment(*linkID, *commentID, *rootID, *userID, mentionControl.CleanedText, xhh.ImageCommentOptions{
		DryRun:            true,
		MockImage:         *mockImage,
		MentionTargetText: mentionControl.TargetText,
	})
	if !result.Handled {
		fmt.Println("dry-run: comment is not an image generation command")
		return
	}
	if result.Err != nil {
		fmt.Println("dry-run error:", result.Err)
		os.Exit(1)
	}
}
