package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// i18nKeyRe is the key format enforced by tools/i18ncheck (ADR-0016).
var i18nKeyRe = regexp.MustCompile(`^[a-z0-9_]+(\.[a-z0-9_]+)+$`)

// worldI18nDir holds generated world names, one catalog per locale. It is kept
// apart from the hand-written catalogs in content/i18n, and datagen check (not
// i18ncheck) validates key parity across its files.
const worldI18nDir = "content/i18n/world"

// neLanguages maps Natural Earth NAME_xx suffixes to BCP 47 locale codes.
var neLanguages = map[string]string{
	"AR": "ar", "BN": "bn", "DE": "de", "EL": "el", "EN": "en", "ES": "es", "FA": "fa",
	"FR": "fr", "HE": "he", "HI": "hi", "HU": "hu", "ID": "id", "IT": "it", "JA": "ja",
	"KO": "ko", "NL": "nl", "PL": "pl", "PT": "pt", "RU": "ru", "SV": "sv", "TR": "tr",
	"UK": "uk", "UR": "ur", "VI": "vi", "ZH": "zh-Hans", "ZHT": "zh-Hant",
}

// localizedNames extracts NAME_xx fields as locale → name.
func localizedNames(f Feature) map[string]string {
	out := map[string]string{}
	for suffix, locale := range neLanguages {
		if v := strings.TrimSpace(f.str("NAME_" + suffix)); v != "" {
			out[locale] = v
		}
	}
	return out
}

// englishName prefers NAME_EN ("South Georgia and the South Sandwich Islands")
// over Natural Earth's abbreviated NAME ("S. Geo. and the Is.").
func englishName(f Feature) string {
	if v := strings.TrimSpace(f.str("NAME_EN")); v != "" {
		return v
	}
	return f.str("NAME")
}

func countryKey(iso2 string) string { return "country." + strings.ToLower(iso2) + ".name" }

// cityKey turns "ne:1159151622" / "oa:EGLL" into an i18n key segment.
func cityKey(id string) string {
	return "city." + strings.ToLower(strings.NewReplacer(":", "_", "-", "_").Replace(id)) + ".name"
}

// Locales returns the generated locale codes in order.
func locales() []string {
	out := make([]string, 0, len(neLanguages))
	for _, l := range neLanguages {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// buildNames returns one catalog per locale with every country and city name. A
// missing translation falls back to the English name, so all catalogs have the
// same keys. It also returns, per locale, how many names fell back.
func buildNames(countries []Country, cities []City) (map[string]map[string]string, map[string]int) {
	catalogs := map[string]map[string]string{}
	fallbacks := map[string]int{}
	for _, l := range locales() {
		catalogs[l] = map[string]string{}
	}
	add := func(key, english string, names map[string]string) {
		for _, l := range locales() {
			v := names[l]
			if l == "en" || v == "" {
				if l != "en" {
					fallbacks[l]++
				}
				v = english
			}
			catalogs[l][key] = v
		}
	}
	for _, c := range countries {
		add(countryKey(c.ISO2), c.Name, c.names)
	}
	for _, c := range cities {
		english := c.Name
		if v := c.names["en"]; v != "" {
			english = v
		}
		add(cityKey(c.ID), english, c.names)
	}
	return catalogs, fallbacks
}

func writeNames(dir string, catalogs map[string]map[string]string) error {
	for _, l := range sortedKeys(catalogs) {
		if err := writeJSON(filepath.Join(dir, l+".json"), catalogs[l]); err != nil {
			return err
		}
	}
	return nil
}

// checkNames verifies every locale catalog has exactly the English keys, with
// non-empty values, and that every country and city has a name.
func checkNames(dir string, countries []Country, cities []City) []error {
	var en map[string]string
	if err := readJSON(filepath.Join(dir, "en.json"), &en); err != nil {
		return []error{err}
	}
	var errs []error
	for _, k := range sortedKeys(en) {
		if !i18nKeyRe.MatchString(k) {
			errs = append(errs, fmt.Errorf("world names: key %q does not match the i18n key format", k))
		}
	}
	for _, c := range countries {
		if en[countryKey(c.ISO2)] == "" {
			errs = append(errs, fmt.Errorf("world names: no English name for country %s", c.ISO2))
		}
	}
	for _, c := range cities {
		if en[cityKey(c.ID)] == "" {
			errs = append(errs, fmt.Errorf("world names: no English name for city %s", c.ID))
		}
	}
	for _, l := range locales() {
		var cat map[string]string
		if err := readJSON(filepath.Join(dir, l+".json"), &cat); err != nil {
			errs = append(errs, err)
			continue
		}
		if len(cat) != len(en) {
			errs = append(errs, fmt.Errorf("world names: %s has %d keys, en has %d", l, len(cat), len(en)))
		}
		for _, k := range sortedKeys(en) {
			if cat[k] == "" {
				errs = append(errs, fmt.Errorf("world names: %s is missing %s", l, k))
				break
			}
		}
	}
	return errs
}
