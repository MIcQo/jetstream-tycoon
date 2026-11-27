package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/content"
)

// TestRealContentIsValid checks the committed content, the same as make content-check.
func TestRealContentIsValid(t *testing.T) {
	c, refs, err := Load("../../content")
	if err != nil {
		t.Fatal(err)
	}
	if err := content.Validate(c, refs); err != nil {
		t.Fatal(err)
	}
	if len(c.Planes) < 3 || len(c.AirportLevels) != 5 || len(c.Seasons) != 8 || len(c.Holidays) == 0 {
		t.Errorf("unexpected content sizes: %d planes, %d levels, %d seasons, %d holidays",
			len(c.Planes), len(c.AirportLevels), len(c.Seasons), len(c.Holidays))
	}
	if !refs.Countries["US"] || !refs.Regions["Europe"] || !refs.I18nKeys["planes.skylark_50.name"] {
		t.Error("references not loaded")
	}
}

// TestSouthernSeasonsMirrorNorthern documents the authoring rule for seasons.yaml.
func TestSouthernSeasonsMirrorNorthern(t *testing.T) {
	c, _, err := Load("../../content")
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]content.Season{}
	for _, s := range c.Seasons {
		by[s.Climate+"/"+s.Hemisphere] = s
	}
	for _, climate := range []string{"tropical", "subtropical", "temperate", "cold"} {
		n, s := by[climate+"/north"], by[climate+"/south"]
		for m := 0; m < 12; m++ {
			if s.Tourism[m] != n.Tourism[(m+6)%12] || s.Business[m] != n.Business[(m+6)%12] {
				t.Errorf("%s: southern month %d is not the northern month %d", climate, m+1, (m+6)%12+1)
			}
		}
	}
}

func copyContent(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{"planes.yaml", "airport_levels.yaml", "balance.yaml", "seasons.yaml", "holidays.yaml", "i18n/en.json", "world/countries.json"} {
		data, err := os.ReadFile(filepath.Join("../../content", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, f)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadIsStrict(t *testing.T) {
	dir := copyContent(t)
	path := filepath.Join(dir, "planes.yaml")
	data, _ := os.ReadFile(path)
	typo := strings.Replace(string(data), "rangeKm: 1500", "rangKm: 1500", 1)
	if err := os.WriteFile(path, []byte(typo), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "planes.yaml") || !strings.Contains(err.Error(), "rangKm") {
		t.Errorf("a misspelled field must fail, got %v", err)
	}
}

func TestLoadErrors(t *testing.T) {
	for _, missing := range []string{"balance.yaml", "world/countries.json", "i18n/en.json"} {
		dir := copyContent(t)
		if err := os.Remove(filepath.Join(dir, missing)); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Load(dir); err == nil {
			t.Errorf("missing %s accepted", missing)
		}
	}
	dir := copyContent(t)
	if err := os.WriteFile(filepath.Join(dir, "world/countries.json"), []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "countries.json") {
		t.Errorf("bad countries.json: %v", err)
	}
}
