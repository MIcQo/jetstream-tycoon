// Command datagen builds the committed world data in content/world from pinned,
// checksummed public datasets (TECH_DESIGN §2, M1).
//
//	go run ./tools/datagen fetch   # download sources into the cache, verifying SHA-256
//	go run ./tools/datagen build   # fetch if needed, then regenerate content/world
//	go run ./tools/datagen check   # validate committed content/world (no network; CI)
//	go run ./tools/datagen worldbank # refresh the committed World Bank snapshot
//
// Raw downloads live in tools/datagen/.cache (git-ignored). Outputs are deterministic,
// so a rebuild from the same sources produces identical files.
package main

import (
	"fmt"
	"os"
	"time"
)

const (
	cacheDir = "tools/datagen/.cache"
	worldDir = "content/world"
)

func main() {
	cmd := "check"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "fetch":
		err = fetchAll(cacheDir, sources)
	case "build":
		if err = fetchAll(cacheDir, sources); err == nil {
			err = build(cacheDir, worldDir)
		}
	case "check":
		err = check(worldDir)
	case "worldbank":
		err = refreshWorldBank("https://api.worldbank.org/v2", cacheDir, worldBankSnapshot)
	default:
		err = fmt.Errorf("unknown command %q (want fetch, build or check)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "datagen:", err)
		os.Exit(1)
	}
}

func build(cache, out string) error {
	countries, maps, err := buildCountries(cache)
	if err != nil {
		return err
	}
	var wb WorldBank
	if err := readJSON(worldBankSnapshot, &wb); err != nil {
		return fmt.Errorf("world bank snapshot (run `datagen worldbank`): %w", err)
	}
	applyStats(countries, wb)
	if err := writeJSON(out+"/countries.json", countries); err != nil {
		return err
	}
	playable := map[string]bool{}
	for _, c := range countries {
		playable[c.ISO2] = true
	}
	airports, cities, err := buildAirports(cache, playable)
	if err != nil {
		return err
	}
	if err := writeJSON(out+"/airports.json", airports); err != nil {
		return err
	}
	if err := writeJSON(out+"/cities.json", cities); err != nil {
		return err
	}
	catalogs, fallbacks := buildNames(countries, cities)
	if err := writeNames(worldI18nDir, catalogs); err != nil {
		return err
	}
	for _, l := range sortedKeys(fallbacks) {
		if fallbacks[l] > 0 {
			fmt.Printf("datagen: %s: %d names fall back to English\n", l, fallbacks[l])
		}
	}
	for _, m := range maps {
		if err := writeJSON(out+"/map/"+m.File, m); err != nil {
			return err
		}
	}
	fmt.Printf("datagen: %d countries, %d airports, %d cities, %d map levels written to %s\n",
		len(countries), len(airports), len(cities), len(maps), out)
	return nil
}

// refreshWorldBank fetches the indicators for the playable countries and writes
// the committed snapshot.
func refreshWorldBank(baseURL, cache, out string) error {
	if err := fetchAll(cache, sources); err != nil {
		return err
	}
	countries, _, err := buildCountries(cache)
	if err != nil {
		return err
	}
	wb, err := fetchWorldBank(baseURL, countries, time.Now())
	if err != nil {
		return err
	}
	for _, ind := range sortedKeys(wb.Indicators) {
		fmt.Printf("datagen worldbank: %s: %d countries\n", ind, len(wb.Indicators[ind]))
	}
	return writeJSON(out, wb)
}
