package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/content"
	"github.com/MIcQo/jetstream-tycoon/sim/content/bundle"
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

func TestBuildIsDeterministicAndVersioned(t *testing.T) {
	c, _, err := Load("../../content")
	if err != nil {
		t.Fatal(err)
	}
	w, err := LoadWorld("../../content/world")
	if err != nil {
		t.Fatal(err)
	}
	gz1, v1, err := Build(c, w)
	if err != nil {
		t.Fatal(err)
	}
	gz2, v2, _ := Build(c, w)
	if !bytes.Equal(gz1, gz2) || v1 != v2 {
		t.Fatal("two builds of the same content differ")
	}
	b, err := bundle.Decode(gz1)
	if err != nil || b.ContentVersion != v1 || len(b.World.Airports) != len(w.Airports) {
		t.Fatalf("round trip: %v %v", b.ContentVersion, err)
	}
	c.Balance.Demand.D0 *= 2
	if _, v3, _ := Build(c, w); v3 == v1 {
		t.Error("changing a balance value did not change contentVersion")
	}
	// The committed bundle matches the committed content.
	committed, err := os.ReadFile("../../" + bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(committed, gz1) {
		t.Error("committed bundle is stale: run make content-build")
	}
}

func TestRunCommands(t *testing.T) {
	dir := copyContent(t)
	for _, f := range []string{"airports.json", "cities.json", "candidates.json"} {
		data, err := os.ReadFile(filepath.Join("../../content/world", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "world", f), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(t.TempDir(), "content.json.gz")
	if err := run("check", dir, out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Error("check must not write the bundle")
	}
	if err := run("verify", dir, out); err == nil {
		t.Error("verify without a bundle succeeded")
	}
	if err := run("build", dir, out); err != nil {
		t.Fatal(err)
	}
	if err := run("verify", dir, out); err != nil {
		t.Fatalf("verify after build: %v", err)
	}
	// Change the content: the bundle is now stale.
	path := filepath.Join(dir, "balance.yaml")
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), "d0: 0.04", "d0: 0.05", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run("verify", dir, out); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Errorf("verify on changed content: %v", err)
	}
	if err := run("nope", dir, out); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("unknown command: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "world", "cities.json")); err != nil {
		t.Fatal(err)
	}
	if err := run("build", dir, out); err == nil {
		t.Error("build without cities.json succeeded")
	}
}
