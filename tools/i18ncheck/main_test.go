package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateI18nDirValid(t *testing.T) {
	dir := t.TempDir()
	en := `{"route.panel.title": "Route {name}", "fleet.count": "{count, plural, one {# plane} other {# planes}}"}`
	if err := os.WriteFile(filepath.Join(dir, "en.json"), []byte(en), 0o600); err != nil {
		t.Fatal(err)
	}
	issues, err := ValidateI18nDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
}

func TestValidateI18nDirDetectsErrors(t *testing.T) {
	dir := t.TempDir()
	en := `{"badKey": "Unbalanced {brace", "route.panel.title": ""}`
	de := `{}`
	if err := os.WriteFile(filepath.Join(dir, "en.json"), []byte(en), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "de.json"), []byte(de), 0o600); err != nil {
		t.Fatal(err)
	}
	issues, err := ValidateI18nDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(issues, "\n")
	for _, want := range []string{
		"invalid key \"badKey\"",
		"unbalanced ICU braces",
		"empty translation",
		"missing key \"route.panel.title\"",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected issue containing %q, got:\n%s", want, joined)
		}
	}
}
