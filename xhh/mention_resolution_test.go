package xhh

import (
	"encoding/json"
	"testing"
)

func TestExtractMentionReferenceTargetTrimsParticle(t *testing.T) {
	if got := extractMentionReferenceTarget("@机器人 要艾特她啦"); got != "她" {
		t.Fatalf("extractMentionReferenceTarget = %q, want 她", got)
	}
}

func TestFindPostAuthorMention(t *testing.T) {
	var resp LinkInfoS
	resp.Result.Link.UserID = json.RawMessage(`1001`)
	resp.Result.Link.User.UserName = "neko"
	resp.Result.Link.Text = `[{"type":"html","text":"<a data-user-id=\"2002\" href=\"u\" target=\"_blank\">@M0nika</a> 这是我的主人"}]`

	got := findPostAuthorMention(resp, 3003)
	want := buildMention(1001, "neko")
	if got != want {
		t.Fatalf("findPostAuthorMention = %q, want %q", got, want)
	}
}

func TestFindUniquePostLinkedMentionParsesLinkTextJSON(t *testing.T) {
	var resp LinkInfoS
	resp.Result.Link.Text = `[{"type":"html","text":"<a data-user-id=\"2002\" href=\"u\" target=\"_blank\">@M0nika</a> 这是我的主人"}]`

	got := findUniquePostLinkedMention(resp, 3003)
	want := buildMention(2002, "M0nika")
	if got != want {
		t.Fatalf("findUniquePostLinkedMention = %q, want %q", got, want)
	}
}
