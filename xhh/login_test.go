package xhh

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/skip2/go-qrcode"
)

func TestWriteCookieFileUsesPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookie.json")
	if err := writeCookieFile(path, []byte(`{"cookie":"secret"}`)); err != nil {
		t.Fatalf("writeCookieFile returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat cookie file: %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if got := info.Mode().Perm(); got != cookieFileMode {
		t.Fatalf("cookie file mode = %v, want %v", got, cookieFileMode)
	}
}

func TestRenderTerminalQRCodeAvoidsHalfBlocksOnNarrowTerminal(t *testing.T) {
	code, err := qrcode.New("https://example.com/login?q=mobile", qrcode.Low)
	if err != nil {
		t.Fatalf("qrcode.New returned error: %v", err)
	}

	rendered := renderTerminalQRCode(code, 20)
	if rendered == "" {
		t.Fatal("renderTerminalQRCode returned empty output")
	}
	if strings.ContainsAny(rendered, "▀▄") {
		t.Fatalf("narrow terminal QR contains half-block characters: %q", rendered)
	}
	if !strings.Contains(rendered, "█") {
		t.Fatalf("narrow terminal QR missing full block characters: %q", rendered)
	}
}

func TestRenderTerminalQRCodeUsesDoubleWidthWhenItFits(t *testing.T) {
	code, err := qrcode.New("https://example.com/login?q=desktop", qrcode.Low)
	if err != nil {
		t.Fatalf("qrcode.New returned error: %v", err)
	}

	rendered := renderTerminalQRCode(code, 200)
	if rendered == "" {
		t.Fatal("renderTerminalQRCode returned empty output")
	}
	if !strings.Contains(rendered, "██") {
		t.Fatalf("wide terminal QR missing double-width full blocks: %q", rendered)
	}
}
