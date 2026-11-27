package main

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNaturalEarth1(t *testing.T) {
	x, y := naturalEarth1(0, 0)
	if x != 0 || y != 0 {
		t.Fatalf("origin = (%v, %v)", x, y)
	}
	// d3.geoNaturalEarth1 (unit scale): λ=π at the equator → x = π·0.8707.
	if x, _ := naturalEarth1(180, 0); math.Abs(x-math.Pi*0.8707) > 1e-12 {
		t.Errorf("x(180,0) = %v", x)
	}
	if _, y := naturalEarth1(0, 90); math.Abs(y-1.4224) > 1e-3 {
		t.Errorf("y(0,90) = %v, want ≈ 1.4224", y)
	}
	// Symmetric in longitude and latitude.
	x1, y1 := naturalEarth1(40, 30)
	x2, y2 := naturalEarth1(-40, -30)
	if x1 != -x2 || y1 != -y2 {
		t.Errorf("not symmetric: (%v,%v) vs (%v,%v)", x1, y1, x2, y2)
	}
	// Meridians converge towards the poles.
	if xe, _ := naturalEarth1(90, 0); func() bool { xp, _ := naturalEarth1(90, 80); return xp >= xe }() {
		t.Error("meridian does not narrow towards the pole")
	}
}

func TestFrame(t *testing.T) {
	f := newFrame(1000)
	if f.Width != 1000 || f.Height < 500 || f.Height > 540 {
		t.Fatalf("frame = %+v", f)
	}
	if x, y := f.toView(0, 0); x != 500 || y != f.Height/2 {
		t.Errorf("center = (%v, %v)", x, y)
	}
	if x, _ := f.toView(180, 0); math.Abs(x-1000) > 1e-9 {
		t.Errorf("east edge x = %v", x)
	}
	if _, y := f.toView(0, 90); y > 1 { // north is up (small y)
		t.Errorf("north pole y = %v", y)
	}
}

func TestTenths(t *testing.T) {
	cases := map[int64]string{0: "0", 5: "0.5", 10: "1", 123: "12.3", -4: "-0.4", -120: "-12", 7000: "700"}
	for in, want := range cases {
		if got := tenths(in); got != want {
			t.Errorf("tenths(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestSVGPath(t *testing.T) {
	// A frame with scale 1 per projected unit would round everything to 0; use a
	// 1000-wide frame and a ~10° square near the origin.
	f := newFrame(1000)
	sq := [][][][2]float64{{{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}}}}
	p := f.svgPath(sq)
	if !strings.HasPrefix(p, "M500,") || !strings.HasSuffix(p, "z") || strings.Count(p, "M") != 1 {
		t.Fatalf("path = %q", p)
	}
	if got := strings.Count(p[strings.Index(p, "l")+1:], " ") + 1; got != 3 {
		t.Errorf("square has %d relative segments, want 3 (closing point dropped): %q", got, p)
	}
	// Points that round onto the previous one are dropped; a ring that collapses
	// to fewer than 3 points disappears.
	tiny := [][][][2]float64{{{{0, 0}, {0.00001, 0}, {0, 0.00001}, {0, 0}}}}
	if p := f.svgPath(tiny); p != "" {
		t.Errorf("tiny ring = %q, want empty", p)
	}
	// Two polygons (and a hole) produce one M per ring.
	multi := [][][][2]float64{sq[0], {{{20, 20}, {30, 20}, {30, 30}, {20, 20}}, {{22, 22}, {28, 22}, {28, 28}, {22, 22}}}}
	if got := strings.Count(f.svgPath(multi), "M"); got != 3 {
		t.Errorf("rings = %d, want 3", got)
	}
	if !pathRe.MatchString(p) {
		t.Errorf("path does not match the validator's pattern: %q", p)
	}
}

const fixture = `{"type":"FeatureCollection","features":[
 {"properties":{"ISO_A2":"AU","ISO_A2_EH":"AU","ISO_A3_EH":"AUS","NAME":"Australia","CONTINENT":"Oceania","REGION_UN":"Oceania","SUBREGION":"Australia and New Zealand","LABEL_X":134.0,"LABEL_Y":-24.1,"ADM0_A3":"AUS"},
  "geometry":{"type":"Polygon","coordinates":[[[113,-22],[153,-28],[146,-39],[113,-22]]]}},
 {"properties":{"ISO_A2":"-99","ISO_A2_EH":"AU","ISO_A3_EH":"AUS","NAME":"Indian Ocean Ter.","CONTINENT":"Asia","ADM0_A3":"IOA"},
  "geometry":{"type":"MultiPolygon","coordinates":[[[[105,-10],[106,-10],[106,-11],[105,-10]]]]}},
 {"properties":{"ISO_A2":"-99","ISO_A2_EH":"FR","ISO_A3_EH":"FRA","NAME":"France","CONTINENT":"Europe","REGION_UN":"Europe","SUBREGION":"Western Europe","LABEL_X":2.5,"LABEL_Y":46.7,"ADM0_A3":"FRA"},
  "geometry":{"type":"Polygon","coordinates":[[[-4,48],[8,49],[3,43],[-4,48]]]}},
 {"properties":{"ISO_A2":"-99","ISO_A2_EH":"-99","NAME":"Somaliland","ADM0_A3":"SOL"},
  "geometry":{"type":"Polygon","coordinates":[[[43,10],[48,11],[47,8],[43,10]]]}}
]}`

func TestBuildCountries(t *testing.T) {
	cache := t.TempDir()
	for _, l := range mapLevels {
		if err := os.WriteFile(filepath.Join(cache, l.source), []byte(fixture), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	countries, maps, err := buildCountries(cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(countries) != 2 || countries[0].ISO2 != "AU" || countries[1].ISO2 != "FR" {
		t.Fatalf("countries = %+v", countries)
	}
	au := countries[0]
	if au.Name != "Australia" || au.Continent != "Oceania" || au.ISO3 != "AUS" || au.Label != [2]float64{134, -24.1} {
		t.Errorf("AU metadata should come from the primary feature: %+v", au)
	}
	if countries[1].Name != "France" {
		t.Errorf("France via ISO_A2_EH: %+v", countries[1])
	}
	m := maps[0]
	if strings.Count(m.Countries["AU"], "M") != 2 {
		t.Errorf("AU should merge both features' rings: %q", m.Countries["AU"])
	}
	if _, ok := m.Other["SOL"]; !ok || len(m.Other) != 1 {
		t.Errorf("Somaliland should be drawn but not playable: %v", m.Other)
	}
	if m.Projection != "naturalEarth1" || m.ViewBox[2] != mapWidth {
		t.Errorf("map header = %+v", m)
	}

	// The written output passes check.
	out := t.TempDir()
	if err := writeJSON(filepath.Join(out, "countries.json"), countries); err != nil {
		t.Fatal(err)
	}
	for _, mm := range maps {
		if err := writeJSON(filepath.Join(out, "map", mm.File), mm); err != nil {
			t.Fatal(err)
		}
	}
	_ = writeJSON(filepath.Join(out, "airports.json"), []Airport{})
	_ = writeJSON(filepath.Join(out, "cities.json"), []City{})
	_ = writeCandidates(filepath.Join(out, "candidates.json"), nil)
	if err := check(out); err == nil || !strings.Contains(err.Error(), "want at least 200") {
		t.Errorf("check on 2 countries = %v, want the minimum-count error", err)
	}
}

func TestBuildCountriesErrors(t *testing.T) {
	if _, _, err := buildCountries(t.TempDir()); err == nil {
		t.Error("missing source files accepted")
	}
	cache := t.TempDir()
	for _, l := range mapLevels {
		_ = os.WriteFile(filepath.Join(cache, l.source), []byte(`{"features":[{"geometry":{"type":"Point"}}]}`), 0o600)
	}
	if _, _, err := buildCountries(cache); err == nil || !strings.Contains(err.Error(), "unsupported geometry") {
		t.Errorf("point geometry: %v", err)
	}
}

func TestParseFeaturesErrors(t *testing.T) {
	for _, bad := range []string{
		`not json`,
		`{"features":[{"geometry":{"type":"Polygon","coordinates":"x"}}]}`,
		`{"features":[{"geometry":{"type":"MultiPolygon","coordinates":[1]}}]}`,
	} {
		if _, err := parseFeatures([]byte(bad)); err == nil {
			t.Errorf("parseFeatures(%s) succeeded", bad)
		}
	}
	f := Feature{Props: map[string]any{"n": 1.5, "s": "x"}}
	if f.str("n") != "" || f.num("s") != 0 || f.str("s") != "x" || f.num("n") != 1.5 {
		t.Error("property accessors")
	}
}

func TestIsoCode(t *testing.T) {
	for in, want := range map[string]string{"FR": "FR", "-99": "", "": "", "ABC": ""} {
		if got := isoCode(Feature{Props: map[string]any{"ISO_A2_EH": in}}); got != want {
			t.Errorf("isoCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFetchAndVerify(t *testing.T) {
	body := []byte("hello world data")
	sum := sha256.Sum256(body)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	dir := t.TempDir()
	good := []Source{{File: "a.txt", URL: srv.URL + "/a", SHA256: hex.EncodeToString(sum[:])}}
	if err := fetchAll(dir, good); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(got) != string(body) {
		t.Fatalf("downloaded %q", got)
	}
	if err := fetchAll(dir, good); err != nil { // cached: verified again, not re-downloaded
		t.Fatal(err)
	}

	bad := []Source{{File: "b.txt", URL: srv.URL + "/b", SHA256: strings.Repeat("0", 64)}}
	if err := fetchAll(dir, bad); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Errorf("checksum mismatch: %v", err)
	}
	missing := []Source{{File: "c.txt", URL: srv.URL + "/missing", SHA256: "x"}}
	if err := fetchAll(dir, missing); err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("404: %v", err)
	}
}

func TestCheckCommittedWorldData(t *testing.T) {
	// Validates the real committed files, the same as make content-check.
	if err := check("../../content/world"); err != nil {
		t.Fatal(err)
	}
}

func TestCheckFindsProblems(t *testing.T) {
	dir := t.TempDir()
	countries := []Country{{ISO2: "zz"}, {ISO2: "AA", Name: "A", Continent: "X", Label: [2]float64{200, 0},
		Stats: Stats{Population: -1, Business: 2, Hemisphere: "up", Climate: "polar"}}}
	_ = writeJSON(filepath.Join(dir, "countries.json"), countries)
	for _, l := range mapLevels {
		_ = writeJSON(filepath.Join(dir, "map", l.file), WorldMap{Countries: map[string]string{"QQ": "bogus"}})
	}
	_ = writeJSON(filepath.Join(dir, "airports.json"), []Airport{})
	_ = writeJSON(filepath.Join(dir, "cities.json"), []City{})
	_ = writeCandidates(filepath.Join(dir, "candidates.json"), nil)
	err := check(dir)
	for _, want := range []string{"want at least 200", `bad or duplicate iso2 "zz"`, "not sorted", "zz has no name", "AA label out of range", "QQ is not in countries.json", "QQ has a malformed path",
		"AA has missing or negative stats", "AA profile outside", "AA has unknown hemisphere", "AA has no source for population",
		"AA wealth 0 outside [0.3, 3]"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("check error missing %q: %v", want, err)
		}
	}
	if err := check(t.TempDir()); err == nil {
		t.Error("missing files accepted")
	}
}
