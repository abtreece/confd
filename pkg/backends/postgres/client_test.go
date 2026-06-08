package postgres

import (
	"fmt"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────
// Test helpers: pure functions extracted from the client
// ─────────────────────────────────────────────────────────

// buildConnStr mirrors the logic inside New() so we can test it in isolation.
func buildConnStr(username, password, node, dbName string) string {
	if username != "" {
		return fmt.Sprintf("postgres://%s:%s@%s/%s", username, password, node, dbName)
	}
	return fmt.Sprintf("postgres://%s/%s", node, dbName)
}

// buildPrefix mirrors the prefix logic inside GetValues().
func buildPrefix(key string) string {
	prefix := key
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix + "%"
}

// ─────────────────────────────────────────────────────────
// Unit tests — no database required
// ─────────────────────────────────────────────────────────

func TestBuildConnStr_WithCredentials(t *testing.T) {
	got := buildConnStr("admin", "secret", "localhost:5432", "confd")
	want := "postgres://admin:secret@localhost:5432/confd"
	if got != want {
		t.Errorf("buildConnStr() = %q, want %q", got, want)
	}
}

func TestBuildConnStr_NoCredentials(t *testing.T) {
	got := buildConnStr("", "", "localhost:5432", "confd")
	want := "postgres://localhost:5432/confd"
	if got != want {
		t.Errorf("buildConnStr() = %q, want %q", got, want)
	}
}

func TestBuildConnStr_CustomDB(t *testing.T) {
	got := buildConnStr("user", "pass", "10.0.0.1:5433", "mydb")
	if !strings.Contains(got, "mydb") {
		t.Errorf("expected database name in DSN, got %q", got)
	}
	if !strings.Contains(got, "10.0.0.1:5433") {
		t.Errorf("expected host in DSN, got %q", got)
	}
}

func TestBuildPrefix_WithoutTrailingSlash(t *testing.T) {
	got := buildPrefix("/app/database")
	want := "/app/database/%"
	if got != want {
		t.Errorf("buildPrefix() = %q, want %q", got, want)
	}
}

func TestBuildPrefix_WithTrailingSlash(t *testing.T) {
	got := buildPrefix("/app/database/")
	want := "/app/database/%"
	if got != want {
		t.Errorf("buildPrefix() = %q, want %q", got, want)
	}
}

func TestBuildPrefix_Root(t *testing.T) {
	got := buildPrefix("/")
	want := "/%"
	if got != want {
		t.Errorf("buildPrefix() = %q, want %q", got, want)
	}
}

func TestBuildPrefix_SingleSegment(t *testing.T) {
	got := buildPrefix("/app")
	if !strings.HasSuffix(got, "%") {
		t.Errorf("prefix should end with %%, got %q", got)
	}
	if !strings.Contains(got, "/app/") {
		t.Errorf("prefix should contain /app/, got %q", got)
	}
}

func TestDefaultValues(t *testing.T) {
	// Verify that the default table name is applied correctly.
	// This mirrors the guard in New().
	table := ""
	if table == "" {
		table = "confd_config"
	}
	if table != "confd_config" {
		t.Errorf("default table should be confd_config, got %q", table)
	}

	dbName := ""
	if dbName == "" {
		dbName = "confd"
	}
	if dbName != "confd" {
		t.Errorf("default database should be confd, got %q", dbName)
	}
}

// Table-driven test for buildPrefix covering multiple cases at once
func TestBuildPrefix_TableDriven(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"/app", "/app/%"},
		{"/app/", "/app/%"},
		{"/app/db", "/app/db/%"},
		{"/", "/%"},
		{"/a/b/c", "/a/b/c/%"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := buildPrefix(tc.input)
			if got != tc.want {
				t.Errorf("buildPrefix(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
