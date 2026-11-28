package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalizedAndEnglishNames(t *testing.T) {
	f := Feature{Props: map[string]any{"NAME": "S. Geo.", "NAME_EN": " South Georgia ", "NAME_DE": "Südgeorgien", "NAME_ZH": "南乔治亚", "NAME_ZHT": "南喬治亞", "NAME_FR": ""}}
	n := localizedNames(f)
	if n["de"] != "Südgeorgien" || n["zh-Hans"] != "南乔治亚" || n["zh-Hant"] != "南喬治亞" || n["en"] != "South Georgia" {
		t.Errorf("names = %v", n)
	}
	if _, ok := n["fr"]; ok {
		t.Error("empty name kept")
	}
	if englishName(f) != "South Georgia" {
		t.Errorf("englishName = %q", englishName(f))
	}
	if englishName(Feature{Props: map[string]any{"NAME": "Short"}}) != "Short" {
		t.Error("englishName fallback to NAME")
	}
}

func TestKeys(t *testing.T) {
	cases := map[string]string{
		countryKey("FR"):         "country.fr.name",
		cityKey("ne:1159151622"): "city.ne_1159151622.name",
		cityKey("oa:US-0042"):    "city.oa_us_0042.name",
	}
	for got, want := range cases {
		if got != want || !i18nKeyRe.MatchString(got) {
			t.Errorf("key %q, want %q (valid format)", got, want)
		}
	}
	if len(locales()) != 26 || locales()[0] != "ar" {
		t.Errorf("locales = %v", locales())
	}
}

func TestBuildNamesFallsBackToEnglish(t *testing.T) {
	countries := []Country{{ISO2: "DE", Name: "Germany", names: map[string]string{"en": "Germany", "de": "Deutschland"}}}
	cities := []City{
		{ID: "ne:1", Name: "München", names: map[string]string{"en": "Munich", "de": "München"}},
		{ID: "oa:X", Name: "Remote"}, // unmatched: English only
	}
	cats, fallbacks := buildNames(countries, cities)
	if cats["de"]["country.de.name"] != "Deutschland" || cats["ja"]["country.de.name"] != "Germany" {
		t.Errorf("country names: de=%q ja=%q", cats["de"]["country.de.name"], cats["ja"]["country.de.name"])
	}
	if cats["en"]["city.ne_1.name"] != "Munich" || cats["de"]["city.ne_1.name"] != "München" {
		t.Errorf("city names: %q %q", cats["en"]["city.ne_1.name"], cats["de"]["city.ne_1.name"])
	}
	if cats["fr"]["city.oa_x.name"] != "Remote" {
		t.Errorf("unmatched city: %q", cats["fr"]["city.oa_x.name"])
	}
	// de lacks only the remote city; ja lacks all three names.
	if fallbacks["de"] != 1 || fallbacks["ja"] != 3 || fallbacks["en"] != 0 {
		t.Errorf("fallbacks = %v", fallbacks)
	}
	for l, c := range cats {
		if len(c) != 3 {
			t.Errorf("%s has %d keys", l, len(c))
		}
	}

	dir := t.TempDir()
	if err := writeNames(dir, cats); err != nil {
		t.Fatal(err)
	}
	if errs := checkNames(dir, countries, cities); len(errs) != 0 {
		t.Fatalf("checkNames on generated output: %v", errs)
	}
}

func TestCheckNamesFindsProblems(t *testing.T) {
	dir := t.TempDir()
	cats, _ := buildNames([]Country{{ISO2: "DE", Name: "Germany"}}, nil)
	cats["en"]["Bad Key"] = "x"
	delete(cats["fr"], "country.de.name")
	cats["fr"]["country.xx.name"] = ""
	if err := writeNames(dir, cats); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(dir, "it.json"))
	errs := checkNames(dir, []Country{{ISO2: "DE"}, {ISO2: "QQ"}}, []City{{ID: "ne:9"}})
	joined := ""
	for _, e := range errs {
		joined += e.Error() + "\n"
	}
	for _, want := range []string{`key "Bad Key" does not match`, "no English name for country QQ", "no English name for city ne:9", "it.json", "fr is missing"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	if errs := checkNames(t.TempDir(), nil, nil); len(errs) != 1 {
		t.Errorf("missing en.json: %v", errs)
	}
}
