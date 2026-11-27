package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// writeJSON writes v as JSON with sorted map keys and a trailing newline, so
// regenerating unchanged data produces byte-identical files.
func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

var (
	iso2Re = regexp.MustCompile(`^[A-Z]{2}$`)
	pathRe = regexp.MustCompile(`^(M-?[0-9.]+,-?[0-9.]+l(-?[0-9.]+,-?[0-9.]+ ?)+z)+$`)
)

// check validates the committed world data without network access (CI content-check).
func check(dir string) error {
	var countries []Country
	if err := readJSON(filepath.Join(dir, "countries.json"), &countries); err != nil {
		return err
	}
	var errs []error
	if len(countries) < 200 {
		errs = append(errs, fmt.Errorf("countries.json: %d countries, want at least 200", len(countries)))
	}
	seen := map[string]bool{}
	for i, c := range countries {
		if !iso2Re.MatchString(c.ISO2) || seen[c.ISO2] {
			errs = append(errs, fmt.Errorf("countries.json[%d]: bad or duplicate iso2 %q", i, c.ISO2))
		}
		if i > 0 && countries[i-1].ISO2 >= c.ISO2 {
			errs = append(errs, fmt.Errorf("countries.json: not sorted at %s", c.ISO2))
		}
		seen[c.ISO2] = true
		if c.Name == "" || c.Continent == "" {
			errs = append(errs, fmt.Errorf("countries.json: %s has no name or continent", c.ISO2))
		}
		if c.Label[0] < -180 || c.Label[0] > 180 || c.Label[1] < -90 || c.Label[1] > 90 {
			errs = append(errs, fmt.Errorf("countries.json: %s label out of range", c.ISO2))
		}
	}
	for _, lvl := range mapLevels {
		var m WorldMap
		if err := readJSON(filepath.Join(dir, "map", lvl.file), &m); err != nil {
			return err
		}
		for iso, p := range m.Countries {
			if !seen[iso] {
				errs = append(errs, fmt.Errorf("%s: %s is not in countries.json", lvl.file, iso))
			}
			if p != "" && !pathRe.MatchString(p) {
				errs = append(errs, fmt.Errorf("%s: %s has a malformed path", lvl.file, iso))
			}
		}
	}
	airportErrs, nAirports, nCities, err := checkAirports(dir, seen)
	if err != nil {
		return err
	}
	errs = append(errs, airportErrs...)
	if len(errs) == 0 {
		fmt.Printf("datagen check: %d countries, %d airports, %d cities, %d map levels OK\n",
			len(countries), nAirports, nCities, len(mapLevels))
	}
	return errors.Join(errs...)
}

var iataRe = regexp.MustCompile(`^[A-Z0-9]{3}$`)

func checkAirports(dir string, countries map[string]bool) (errs []error, nAirports, nCities int, err error) {
	var airports []Airport
	var cities []City
	if err := readJSON(filepath.Join(dir, "airports.json"), &airports); err != nil {
		return nil, 0, 0, err
	}
	if err := readJSON(filepath.Join(dir, "cities.json"), &cities); err != nil {
		return nil, 0, 0, err
	}
	cityIDs := map[string]bool{}
	for _, c := range cities {
		if cityIDs[c.ID] || c.Pop < minCityPop || c.Name == "" {
			errs = append(errs, fmt.Errorf("cities.json: %s is duplicated, unnamed or below the population floor", c.ID))
		}
		cityIDs[c.ID] = true
	}
	withAirport := map[string]bool{}
	for i, a := range airports {
		if !iataRe.MatchString(a.IATA) {
			errs = append(errs, fmt.Errorf("airports.json[%d]: bad IATA %q", i, a.IATA))
		}
		if i > 0 && airports[i-1].IATA >= a.IATA {
			errs = append(errs, fmt.Errorf("airports.json: not sorted or duplicate at %s", a.IATA))
		}
		if !countries[a.ISO2] {
			errs = append(errs, fmt.Errorf("airports.json: %s in unknown country %q", a.IATA, a.ISO2))
		}
		if !cityIDs[a.CityID] {
			errs = append(errs, fmt.Errorf("airports.json: %s has unknown city %q", a.IATA, a.CityID))
		}
		if a.BaseLevel < 2 || a.BaseLevel > 3 {
			errs = append(errs, fmt.Errorf("airports.json: %s base level %d out of range", a.IATA, a.BaseLevel))
		}
		if a.Lat < -90 || a.Lat > 90 || a.Lon < -180 || a.Lon > 180 {
			errs = append(errs, fmt.Errorf("airports.json: %s coordinates out of range", a.IATA))
		}
		withAirport[a.ISO2] = true
	}
	if len(airports) < 900 || len(withAirport) < 200 {
		errs = append(errs, fmt.Errorf("airports.json: %d airports in %d countries, want at least 900 in 200", len(airports), len(withAirport)))
	}
	return errs, len(airports), len(cities), nil
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
