package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateDirValid(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "0001_init.up.sql"), "CREATE TABLE t (id UInt64) ENGINE = Memory;")
	mustWrite(t, filepath.Join(dir, "0001_init.down.sql"), "DROP TABLE IF EXISTS t;")

	issues, err := ValidateDir(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
}

func TestValidateDirDetectsMissingDownAndNullable(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "0002_events.up.sql"), "CREATE TABLE t (x Nullable(String));")

	issues, err := ValidateDir(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(issues, "\n")
	for _, want := range []string{
		"ClickHouse migrations must not use Nullable",
		"non-contiguous migration version 0002 (expected 0001)",
		"missing .down.sql",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected issue containing %q, got:\n%s", want, joined)
		}
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
