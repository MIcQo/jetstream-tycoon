package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHaversine(t *testing.T) {
	// LHR → JFK is about 5,540 km.
	if d := haversineKm(51.4706, -0.461941, 40.639801, -73.7789); math.Abs(d-5540) > 15 {
		t.Errorf("LHR-JFK = %.0f km", d)
	}
	if d := haversineKm(10, 20, 10, 20); d != 0 {
		t.Errorf("same point = %v", d)
	}
	// Antipodes: half the circumference.
	if d := haversineKm(0, 0, 0, 180); math.Abs(d-math.Pi*earthKm) > 1e-6 {
		t.Errorf("antipodes = %v", d)
	}
}

var testPlaces = []place{
	{id: "ne:1", name: "Metro", pop: 8_000_000, lat: 51.50, lon: -0.12},
	{id: "ne:2", name: "Town", pop: 100_000, lat: 51.11, lon: -0.18},
	{id: "ne:3", name: "Tiny", pop: 2_000, lat: 10.00, lon: 10.00},
}

func TestMatchCityPrefersMetroOverAdjacentTown(t *testing.T) {
	gatwick := rawAirport{ident: "EGKK", lat: 51.148, lon: -0.190, iso: "GB"} // ~4 km from Town, ~45 km from Metro
	c := matchCity(gatwick, testPlaces)
	if c.ID != "ne:1" || c.Name != "Metro" || !c.Matched || c.Pop != 8_000_000 || c.ISO2 != "GB" {
		t.Fatalf("Gatwick matched %+v, want Metro", c)
	}
	// Next to a sizeable town, with only a mid-size city 77 km away, the town wins.
	near := rawAirport{ident: "X", lat: 51.11, lon: -0.18}
	far := []place{{id: "ne:2", name: "Town", pop: 200_000, lat: 51.11, lon: -0.18}, {id: "ne:9", name: "Far city", pop: 1_000_000, lat: 51.80, lon: -0.12}}
	if c := matchCity(near, far); c.Name != "Town" {
		t.Errorf("matched %q, want Town", c.Name)
	}
}

func TestMatchCityFallbacks(t *testing.T) {
	// A matched place below the population floor is raised to it.
	if c := matchCity(rawAirport{lat: 10.01, lon: 10.0}, testPlaces); c.Name != "Tiny" || c.Pop != minCityPop {
		t.Errorf("tiny place = %+v", c)
	}
	// Nothing within range: the airport's municipality, at the floor, unmatched.
	r := rawAirport{ident: "NZCH", municipality: "Remote", name: "Remote Airport", lat: -40, lon: 170, iso: "NZ"}
	c := matchCity(r, testPlaces)
	if c.ID != "oa:NZCH" || c.Name != "Remote" || c.Matched || c.Pop != minCityPop || c.Lat != -40 {
		t.Errorf("unmatched = %+v", c)
	}
	r.municipality = ""
	if c := matchCity(r, testPlaces); c.Name != "Remote Airport" {
		t.Errorf("no municipality: name = %q", c.Name)
	}
	// Just outside the radius does not match.
	edge := rawAirport{lat: 10 + (cityMatchKm+1)/111.2, lon: 10}
	if c := matchCity(edge, testPlaces); c.Matched {
		t.Errorf("place %.1f km away matched", haversineKm(edge.lat, edge.lon, 10, 10))
	}
}

func TestSelectAirports(t *testing.T) {
	playable := map[string]bool{"GB": true, "MV": true, "XX": true}
	rows := []rawAirport{
		{iata: "LHR", typ: "large_airport", sched: "yes", iso: "GB", lat: 51.47, lon: -0.46},
		{iata: "BHX", typ: "medium_airport", sched: "yes", iso: "GB", lat: 52.45, lon: -1.74}, // GB has a large: skip
		{iata: "AAA", typ: "medium_airport", sched: "yes", iso: "MV", lat: 10.0, lon: 10.0},
		{iata: "BBB", typ: "medium_airport", sched: "yes", iso: "MV", lat: 51.5, lon: -0.12}, // bigger city: chosen
		{iata: "CCC", typ: "large_airport", sched: "no", iso: "XX"},
		{iata: "", typ: "large_airport", sched: "yes", iso: "XX"},
		{iata: "DDD", typ: "large_airport", sched: "yes", iso: "ZZ"}, // not playable
		{iata: "EEE", typ: "small_airport", sched: "yes", iso: "XX"},
	}
	got := map[string]bool{}
	for _, r := range selectAirports(rows, testPlaces, playable) {
		got[r.iata] = true
	}
	if len(got) != 2 || !got["LHR"] || !got["BBB"] {
		t.Fatalf("selected %v, want LHR and BBB", got)
	}
	// Equal city population: the alphabetically first IATA wins, deterministically.
	tie := []rawAirport{
		{iata: "ZZZ", typ: "medium_airport", sched: "yes", iso: "MV", lat: -40, lon: 170},
		{iata: "MMM", typ: "medium_airport", sched: "yes", iso: "MV", lat: -41, lon: 171},
	}
	if s := selectAirports(tie, testPlaces, playable); len(s) != 1 || s[0].iata != "MMM" {
		t.Errorf("tie picked %+v", s)
	}
}

const testCSV = `"id","ident","type","name","latitude_deg","longitude_deg","elevation_ft","continent","iso_country","iso_region","municipality","scheduled_service","icao_code","iata_code","gps_code","local_code","home_link","wikipedia_link","keywords"
1,"EGLL","large_airport","Heathrow",51.4706,-0.461941,83,"EU","GB","GB-ENG","London","yes","EGLL","LHR","","","","",""
2,"XMED","medium_airport","Medium Field",10.01,10.0,0,"AF","MV","MV-1","Tiny","yes","","MED","","","","",""
`

func TestParseAirportsCSV(t *testing.T) {
	rows, err := parseAirportsCSV(strings.NewReader(testCSV))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].iata != "LHR" || rows[0].lat != 51.4706 || rows[1].icao != "" || rows[1].municipality != "Tiny" {
		t.Fatalf("rows = %+v", rows)
	}
	for _, bad := range []string{
		``,
		"ident,type\nX,Y\n",
		strings.Replace(testCSV, "51.4706", "north", 1),
		strings.Replace(testCSV, `"GB","GB-ENG"`, `"GB"`, 1), // wrong field count
	} {
		if _, err := parseAirportsCSV(strings.NewReader(bad)); err == nil {
			t.Errorf("parseAirportsCSV accepted %.40q", bad)
		}
	}
}

func TestBuildAirports(t *testing.T) {
	cache := t.TempDir()
	places := `{"features":[
	 {"properties":{"NE_ID":1,"NAME":"London","POP_MAX":8567000,"LATITUDE":51.5,"LONGITUDE":-0.12}},
	 {"properties":{"NE_ID":2,"NAME":"Tiny","POP_MAX":2000,"LATITUDE":10,"LONGITUDE":10}}]}`
	write := func(name, data string) {
		if err := os.WriteFile(filepath.Join(cache, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("airports.csv", testCSV)
	write("ne_10m_populated_places.geojson", places)
	airports, cities, err := buildAirports(cache, map[string]bool{"GB": true, "MV": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(airports) != 2 || airports[0].IATA != "LHR" || airports[1].IATA != "MED" {
		t.Fatalf("airports = %+v", airports)
	}
	lhr, med := airports[0], airports[1]
	if lhr.Size != "large" || lhr.BaseLevel != 3 || lhr.ICAO != "EGLL" || lhr.CityID != "ne:1" {
		t.Errorf("LHR = %+v", lhr)
	}
	if med.Size != "medium" || med.BaseLevel != 2 || med.ICAO != "XMED" || med.CityID != "ne:2" {
		t.Errorf("MED (ICAO falls back to ident) = %+v", med)
	}
	if len(cities) != 2 || cities[0].ID != "ne:1" || cities[1].Pop != minCityPop {
		t.Errorf("cities = %+v", cities)
	}

	if _, _, err := buildAirports(t.TempDir(), nil); err == nil {
		t.Error("missing airports.csv accepted")
	}
	write("ne_10m_populated_places.geojson", "not json")
	if _, _, err := buildAirports(cache, nil); err == nil {
		t.Error("bad places file accepted")
	}
}

func TestCheckAirportsFindsProblems(t *testing.T) {
	dir := t.TempDir()
	_ = writeJSON(filepath.Join(dir, "cities.json"), []City{{ID: "c1", Name: "C", Pop: minCityPop}, {ID: "c1", Name: "", Pop: 1}})
	_ = writeJSON(filepath.Join(dir, "airports.json"), []Airport{
		{IATA: "lhr"},
		{IATA: "BBB", ISO2: "QQ", CityID: "c1", BaseLevel: 3},
		{IATA: "AAA", ISO2: "GB", CityID: "c1", BaseLevel: 3},
		{IATA: "CCC", ISO2: "GB", CityID: "nope", BaseLevel: 3},
		{IATA: "DDD", ISO2: "GB", CityID: "c1", BaseLevel: 5},
		{IATA: "EEE", ISO2: "GB", CityID: "c1", BaseLevel: 2, Lat: 95},
	})
	errs, _, _, err := checkAirports(dir, map[string]bool{"GB": true})
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, e := range errs {
		joined += e.Error() + "\n"
	}
	for _, want := range []string{"c1 is duplicated", `bad IATA "lhr"`, `unknown country "QQ"`, "not sorted or duplicate at AAA", `unknown city "nope"`, "base level 5", "EEE coordinates", "want at least 900"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	if _, _, _, err := checkAirports(t.TempDir(), nil); err == nil {
		t.Error("missing files accepted")
	}
}
func TestMatchCityStaysInTheAirportsCountry(t *testing.T) {
	places := []place{
		{iso: "GG", id: "ne:1", name: "St Peter Port", pop: 18_000, lat: 49.46, lon: -2.54},
		{iso: "", id: "ne:2", name: "Unknown country", pop: 1_000_000, lat: 49.30, lon: -2.30},
	}
	jersey := rawAirport{ident: "EGJJ", iso: "JE", municipality: "Saint Helier", lat: 49.21, lon: -2.20}
	if c := matchCity(jersey, places); c.Name != "Unknown country" {
		t.Errorf("places without a country code may still match: got %q", c.Name)
	}
	if c := matchCity(jersey, places[:1]); c.Matched || c.Name != "Saint Helier" {
		t.Errorf("Jersey matched a Guernsey place: %+v", c)
	}
	guernsey := rawAirport{ident: "EGJB", iso: "GG", lat: 49.43, lon: -2.60}
	if c := matchCity(guernsey, places[:1]); c.Name != "St Peter Port" {
		t.Errorf("Guernsey: %+v", c)
	}
	if placeISO(Feature{Props: map[string]any{"ISO_A2": "-99"}}) != "" || placeISO(Feature{Props: map[string]any{"ISO_A2": "GG"}}) != "GG" {
		t.Error("placeISO")
	}
}

func TestSelectAirportsIsSortedAndStable(t *testing.T) {
	playable := map[string]bool{"A": true, "B": true, "C": true}
	rows := []rawAirport{
		{iata: "ZZZ", typ: "medium_airport", sched: "yes", iso: "C"},
		{iata: "MMM", typ: "large_airport", sched: "yes", iso: "B"},
		{iata: "AAA", typ: "medium_airport", sched: "yes", iso: "A"},
	}
	for i := 0; i < 20; i++ { // map iteration order varies between runs
		got := selectAirports(rows, nil, playable)
		if len(got) != 3 || got[0].iata != "AAA" || got[1].iata != "MMM" || got[2].iata != "ZZZ" {
			t.Fatalf("run %d: %v", i, got)
		}
	}
}
