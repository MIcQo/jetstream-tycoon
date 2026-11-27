package main

import (
	"encoding/json"
	"fmt"
)

// Feature is a GeoJSON feature with string properties and (multi)polygon geometry.
type Feature struct {
	Props    map[string]any
	Polygons [][][][2]float64 // polygon → ring → point (lon, lat)
}

type rawFeature struct {
	Properties map[string]any `json:"properties"`
	Geometry   struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	} `json:"geometry"`
}

// parseFeatures decodes a FeatureCollection of Polygon/MultiPolygon features.
func parseFeatures(data []byte) ([]Feature, error) {
	var fc struct {
		Features []rawFeature `json:"features"`
	}
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, err
	}
	out := make([]Feature, 0, len(fc.Features))
	for i, rf := range fc.Features {
		f := Feature{Props: rf.Properties}
		switch rf.Geometry.Type {
		case "Polygon":
			var p [][][2]float64
			if err := json.Unmarshal(rf.Geometry.Coordinates, &p); err != nil {
				return nil, fmt.Errorf("feature %d: %w", i, err)
			}
			f.Polygons = [][][][2]float64{p}
		case "MultiPolygon":
			if err := json.Unmarshal(rf.Geometry.Coordinates, &f.Polygons); err != nil {
				return nil, fmt.Errorf("feature %d: %w", i, err)
			}
		default:
			return nil, fmt.Errorf("feature %d: unsupported geometry %q", i, rf.Geometry.Type)
		}
		out = append(out, f)
	}
	return out, nil
}

// str returns a string property, or "" when missing or not a string.
func (f Feature) str(key string) string {
	s, _ := f.Props[key].(string)
	return s
}

// num returns a numeric property, or 0 when missing or not a number.
func (f Feature) num(key string) float64 {
	n, _ := f.Props[key].(float64)
	return n
}

// parsePoints decodes a FeatureCollection of Point features (properties only are
// needed; coordinates are also present as properties in Natural Earth places).
func parsePoints(data []byte) ([]Feature, error) {
	var fc struct {
		Features []struct {
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, err
	}
	out := make([]Feature, 0, len(fc.Features))
	for _, f := range fc.Features {
		out = append(out, Feature{Props: f.Properties})
	}
	return out, nil
}
