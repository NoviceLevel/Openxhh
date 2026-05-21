package xhh

import (
	"fmt"
	"openxhh/config"
)

func Start() {
	fmt.Println("[XHH] Starting")
	go func() {
		CheckAt()
	}()
	go func() {
		AutoReply()
	}()
	go func() {
		TrackInboundReplies()
	}()
	if config.ConfigStruct.FeedReply.Enabled {
		go func() {
			AutoFeedReply()
		}()
	}
}
