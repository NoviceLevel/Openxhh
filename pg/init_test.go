package pg

import (
	"net/url"
	"testing"
)

func TestBuildPostgresConnStringEscapesCredentialsAndDatabase(t *testing.T) {
	got := buildPostgresConnString("user@example", "p@:s/#word", "db.example.com", "5432", "app/db")
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse conn string: %v", err)
	}
	if parsed.Scheme != "postgresql" {
		t.Fatalf("scheme = %q, want postgresql", parsed.Scheme)
	}
	if parsed.User.Username() != "user@example" {
		t.Fatalf("username = %q", parsed.User.Username())
	}
	passwd, ok := parsed.User.Password()
	if !ok || passwd != "p@:s/#word" {
		t.Fatalf("password = %q, %v", passwd, ok)
	}
	if parsed.Hostname() != "db.example.com" || parsed.Port() != "5432" {
		t.Fatalf("host = %q port = %q", parsed.Hostname(), parsed.Port())
	}
	if parsed.Path != "/app/db" {
		t.Fatalf("path = %q, want /app/db", parsed.Path)
	}
	if parsed.EscapedPath() != "/app%2Fdb" {
		t.Fatalf("escaped path = %q, want /app%%2Fdb", parsed.EscapedPath())
	}
}
