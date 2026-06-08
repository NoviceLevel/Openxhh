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

func TestRenderTerminalQRCodeCompactsNarrowTerminalOutput(t *testing.T) {
	code, err := qrcode.New("https://example.com/login?q=mobile", qrcode.Low)
	if err != nil {
		t.Fatalf("qrcode.New returned error: %v", err)
	}

	rendered := renderTerminalQRCode(code, 40)
	if rendered == "" {
		t.Fatal("renderTerminalQRCode returned empty output")
	}
	if !strings.ContainsAny(rendered, "\u2580\u2584") {
		t.Fatalf("narrow terminal QR should use half-block compaction: %q", rendered)
	}
	lineCount := len(strings.Split(strings.TrimRight(rendered, "\n"), "\n"))
	wantLines := (len(code.Bitmap()) + 1) / 2
	if lineCount != wantLines {
		t.Fatalf("narrow terminal QR line count = %d, want %d", lineCount, wantLines)
	}
}

func TestRenderTerminalQRCodeUsesImageHintWhenTerminalTooNarrow(t *testing.T) {
	code, err := qrcode.New("https://example.com/login?q=mobile", qrcode.Low)
	if err != nil {
		t.Fatalf("qrcode.New returned error: %v", err)
	}

	rendered := renderTerminalQRCode(code, 20)
	if !strings.Contains(rendered, "qrcode.png") {
		t.Fatalf("narrow terminal hint = %q, want qrcode.png", rendered)
	}
	if strings.ContainsAny(rendered, "\u2580\u2584\u2588") {
		t.Fatalf("too-narrow terminal should not render QR blocks: %q", rendered)
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
	if !strings.Contains(rendered, "\u2588\u2588") {
		t.Fatalf("wide terminal QR missing double-width full blocks: %q", rendered)
	}
}
