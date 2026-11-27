// Command datagen builds the committed world data in content/world from pinned,
// checksummed public datasets (TECH_DESIGN §2, M1).
//
//	go run ./tools/datagen fetch   # download sources into the cache, verifying SHA-256
//	go run ./tools/datagen build   # fetch if needed, then regenerate content/world
//	go run ./tools/datagen check   # validate committed content/world (no network; CI)
//
// Raw downloads live in tools/datagen/.cache (git-ignored). Outputs are deterministic,
// so a rebuild from the same sources produces identical files.
package main

import (
	"fmt"
	"os"
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
	for _, m := range maps {
		if err := writeJSON(out+"/map/"+m.File, m); err != nil {
			return err
		}
	}
	fmt.Printf("datagen: %d countries, %d airports, %d cities, %d map levels written to %s\n",
		len(countries), len(airports), len(cities), len(maps), out)
	return nil
}
