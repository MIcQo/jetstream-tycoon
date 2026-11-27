package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

// Country is the game metadata for one country (content/world/countries.json).
type Country struct {
	ISO2      string     `json:"iso2"`
	ISO3      string     `json:"iso3"`
	Name      string     `json:"name"`
	Continent string     `json:"continent"`
	Region    string     `json:"region"`
	Subregion string     `json:"subregion"`
	Label     [2]float64 `json:"label"` // lon, lat of the label point
	Stats     Stats      `json:"stats"`

	// Natural Earth estimates, used only as fallbacks for missing World Bank data.
	nePop         float64
	neGDPMillions float64
}

// WorldMap is one detail level of the pre-projected map (content/world/map/*.json).
type WorldMap struct {
	File       string            `json:"-"`
	Projection string            `json:"projection"`
	ViewBox    [4]float64        `json:"viewBox"`
	Countries  map[string]string `json:"countries"` // ISO2 → SVG path
	Other      map[string]string `json:"other"`     // areas without an ISO code (drawn, not playable)
}

// mapWidth is the viewBox width shared by all detail levels, so layers line up.
const mapWidth = 1000

var mapLevels = []struct{ file, source string }{
	{"world-110m.json", "ne_110m_admin_0_countries.geojson"}, // world zoom
	{"world-50m.json", "ne_50m_admin_0_countries.geojson"},   // regional zoom
}

// isoCode returns the playable ISO 3166-1 alpha-2 code, or "" for areas without one.
// Natural Earth's *_EH fields fill in codes such as France and Norway that are "-99"
// in ISO_A2.
func isoCode(f Feature) string {
	if c := f.str("ISO_A2_EH"); len(c) == 2 && c != "-9" {
		return c
	}
	return ""
}

// round6 keeps label coordinates to ~0.1 m, which keeps diffs stable.
func round6(v float64) float64 { return math.Round(v*1e6) / 1e6 }

func buildCountries(cache string) ([]Country, []WorldMap, error) {
	frame := newFrame(mapWidth)
	var countries []Country
	var maps []WorldMap
	for i, lvl := range mapLevels {
		data, err := os.ReadFile(filepath.Join(cache, lvl.source))
		if err != nil {
			return nil, nil, err
		}
		feats, err := parseFeatures(data)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", lvl.source, err)
		}
		m := WorldMap{
			File:       lvl.file,
			Projection: "naturalEarth1",
			ViewBox:    [4]float64{0, 0, frame.Width, frame.Height},
			Countries:  map[string]string{},
			Other:      map[string]string{},
		}
		// Several features can share a code (e.g. Australia's Indian Ocean Territories
		// carry AU): their shapes are merged, and metadata comes from the feature whose
		// own ISO_A2 matches (the primary one).
		byISO := map[string][]Feature{}
		var order []string
		for _, f := range feats {
			iso := isoCode(f)
			if iso == "" {
				m.Other[f.str("ADM0_A3")] = frame.svgPath(f.Polygons)
				continue
			}
			if _, ok := byISO[iso]; !ok {
				order = append(order, iso)
			}
			byISO[iso] = append(byISO[iso], f)
		}
		for _, iso := range order {
			group := byISO[iso]
			primary := group[0]
			var polys [][][][2]float64
			for _, f := range group {
				if f.str("ISO_A2") == iso {
					primary = f
				}
				polys = append(polys, f.Polygons...)
			}
			m.Countries[iso] = frame.svgPath(polys)
			if i == len(mapLevels)-1 { // the most detailed level defines the country list
				countries = append(countries, Country{
					ISO2:          iso,
					ISO3:          primary.str("ISO_A3_EH"),
					Name:          primary.str("NAME"),
					Continent:     primary.str("CONTINENT"),
					Region:        primary.str("REGION_UN"),
					Subregion:     primary.str("SUBREGION"),
					Label:         [2]float64{round6(primary.num("LABEL_X")), round6(primary.num("LABEL_Y"))},
					nePop:         primary.num("POP_EST"),
					neGDPMillions: primary.num("GDP_MD"),
				})
			}
		}
		maps = append(maps, m)
	}
	sort.Slice(countries, func(i, j int) bool { return countries[i].ISO2 < countries[j].ISO2 })
	return countries, maps, nil
}
