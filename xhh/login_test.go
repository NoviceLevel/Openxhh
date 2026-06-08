package xhh

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
