package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCandidateJSONRoundTrip(t *testing.T) {
	c := Candidate{To: "JFK", Km: 5540, DistFactor: 980.123}
	b, err := json.Marshal(c)
	if err != nil || string(b) != `["JFK",5540,980.123]` {
		t.Fatalf("marshal = %s, %v", b, err)
	}
	var back Candidate
	if err := json.Unmarshal(b, &back); err != nil || back != c {
		t.Fatalf("unmarshal = %+v, %v", back, err)
	}
	for _, bad := range []string{`{}`, `["JFK",1]`, `[1,2,3]`, `["JFK","x",3]`, `["JFK",1,"x"]`} {
		if err := json.Unmarshal([]byte(bad), &back); err == nil {
			t.Errorf("unmarshal(%s) succeeded", bad)
		}
	}
}

func TestApplyWealthAndPopFactors(t *testing.T) {
	cs := []Country{
		{ISO2: "A", Stats: Stats{GDPPerCapitaUSD: 1_000}},
		{ISO2: "B", Stats: Stats{GDPPerCapitaUSD: 10_000}},
		{ISO2: "C", Stats: Stats{GDPPerCapitaUSD: 100_000}},
		{ISO2: "D"}, // no GDP data: neutral
	}
	applyWealth(cs) // median 10,000
	want := []float64{0.3, 1, 3, 1}
	for i, c := range cs {
		if c.Stats.Wealth != want[i] {
			t.Errorf("%s wealth = %v, want %v", c.ISO2, c.Stats.Wealth, want[i])
		}
	}
	// priceBase = pop^0.5 · gdp^0.7: 10,000 people at $1,000 → 100 · 1000^0.7.
	if got, want := priceBase(Stats{Population: 10_000, GDPPerCapitaUSD: 1_000}), math.Round(100*math.Pow(1000, 0.7)); got != want {
		t.Errorf("priceBase = %v, want %v", got, want)
	}
	if priceBase(Stats{Population: -5, GDPPerCapitaUSD: 1_000}) != 0 || priceBase(Stats{Population: 100}) != 0 {
		t.Error("priceBase must be 0 without population or GDP")
	}
	cities := []City{{Pop: 1_000_000}, {Pop: 50_000}}
	applyPopFactors(cities)
	if cities[0].PopFactor != 1000 || cities[1].PopFactor != round3(math.Sqrt(50_000)) {
		t.Errorf("pop factors = %v %v", cities[0].PopFactor, cities[1].PopFactor)
	}
}

// A small world: a two-airport metro (main hub X1 with the longest runway, city
// airport X2), a nearby town, and far-away cities.
func testWorld() ([]Airport, []City, []Country) {
	cities := []City{
		{ID: "x", Pop: 9_000_000, Lat: 0, Lon: 0},
		{ID: "near", Pop: 100_000, Lat: 0, Lon: 0.5},  // ~56 km: too close
		{ID: "mid", Pop: 4_000_000, Lat: 0, Lon: 5},   // ~556 km
		{ID: "far1", Pop: 1_000_000, Lat: 0, Lon: 40}, // ~4,450 km
		{ID: "far2", Pop: 2_000_000, Lat: 0, Lon: 60}, // ~6,670 km
	}
	applyPopFactors(cities)
	airports := []Airport{
		{IATA: "X1", CityID: "x", ISO2: "AA", Lat: 0, Lon: 0.3, RunwayFt: 13_000},
		{IATA: "X2", CityID: "x", ISO2: "AA", Lat: 0, Lon: 0.05, RunwayFt: 7_000},
		{IATA: "NEA", CityID: "near", ISO2: "AA", Lat: 0, Lon: 0.5, RunwayFt: 8_000},
		{IATA: "MID", CityID: "mid", ISO2: "AA", Lat: 0, Lon: 5, RunwayFt: 9_000},
		{IATA: "FA1", CityID: "far1", ISO2: "BB", Lat: 0, Lon: 40, RunwayFt: 9_000},
		{IATA: "FA2", CityID: "far2", ISO2: "BB", Lat: 0, Lon: 60, RunwayFt: 9_000},
	}
	countries := []Country{{ISO2: "AA", Stats: Stats{Wealth: 1}}, {ISO2: "BB", Stats: Stats{Wealth: 1}}}
	return airports, cities, countries
}

func TestBuildCandidates(t *testing.T) {
	airports, cities, countries := testWorld()
	got, err := buildCandidates(airports, cities, countries)
	if err != nil {
		t.Fatal(err)
	}
	from := func(iata string) []string {
		var out []string
		for _, c := range got[iata] {
			out = append(out, c.To)
		}
		return out
	}
	// From MID: the metro appears once, as its main hub X1 (longest runway), not X2.
	if s := strings.Join(from("MID"), ","); !strings.Contains(s, "X1") || strings.Contains(s, "X2") {
		t.Errorf("MID candidates = %s, want X1 only for the metro", s)
	}
	// From X1: no same-city X2, no NEA (under 100 km).
	for _, to := range from("X1") {
		if to == "X2" || to == "NEA" || to == "X1" {
			t.Errorf("X1 has invalid candidate %s", to)
		}
	}
	// Scores rank the bigger, nearer city first.
	if f := from("X1"); len(f) < 3 || f[0] != "MID" {
		t.Errorf("X1 candidates = %v, want MID first", f)
	}
	// Stored km and distFactor agree exactly with the rounding rule.
	for _, c := range got["X1"] {
		if want := round3(math.Pow(float64(c.Km), distExponent)); c.DistFactor != want {
			t.Errorf("%s distFactor %v, want %v", c.To, c.DistFactor, want)
		}
	}
	if len(got) != len(airports) {
		t.Errorf("got %d airports", len(got))
	}
	if _, err := buildCandidates([]Airport{{IATA: "Q", CityID: "nope"}}, cities, countries); err == nil {
		t.Error("unknown city accepted")
	}
}

func TestBuildCandidatesReservesLongHaul(t *testing.T) {
	// 40 short-haul cities with large populations and 15 small long-haul ones: the
	// long-haul ones would never make the top 30 on score alone.
	var airports []Airport
	var cities []City
	add := func(id string, pop int64, lon float64) {
		cities = append(cities, City{ID: id, Pop: pop, Lat: 0, Lon: lon})
		airports = append(airports, Airport{IATA: id, CityID: id, ISO2: "AA", Lat: 0, Lon: lon, RunwayFt: 9_000})
	}
	add("HOM", 1_000_000, 0)
	for i := 0; i < 40; i++ {
		add("S"+string(rune('A'+i%26))+string(rune('A'+i/26)), 5_000_000, 2+float64(i)*0.2)
	}
	for i := 0; i < 15; i++ {
		add("L"+string(rune('A'+i)), 60_000, 40+float64(i))
	}
	applyPopFactors(cities)
	got, err := buildCandidates(airports, cities, []Country{{ISO2: "AA", Stats: Stats{Wealth: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	list := got["HOM"]
	long := 0
	for _, c := range list {
		if c.Km >= longHaulKm {
			long++
		}
	}
	if len(list) != candidatesPerAirport || long != longHaulSlots {
		t.Errorf("HOM: %d candidates, %d long-haul; want %d and %d", len(list), long, candidatesPerAirport, longHaulSlots)
	}
}

func TestIsMainHub(t *testing.T) {
	c := City{Lat: 0, Lon: 0}
	long := Airport{IATA: "B", RunwayFt: 12_000, Lon: 0.5}
	short := Airport{IATA: "A", RunwayFt: 8_000, Lon: 0.1}
	if !isMainHub(long, short, c) || isMainHub(short, long, c) {
		t.Error("longest runway should win")
	}
	near := Airport{IATA: "Z", RunwayFt: 10_000, Lon: 0.1}
	far := Airport{IATA: "A", RunwayFt: 10_000, Lon: 0.5}
	if !isMainHub(near, far, c) {
		t.Error("equal runways: closer to the centre should win")
	}
	twinA := Airport{IATA: "A", RunwayFt: 10_000, Lon: 0.2}
	twinB := Airport{IATA: "B", RunwayFt: 10_000, Lon: 0.2}
	if !isMainHub(twinA, twinB, c) || isMainHub(twinB, twinA, c) {
		t.Error("full tie: lower IATA should win")
	}
}

func TestParseRunways(t *testing.T) {
	csv := `"id","airport_ref","airport_ident","length_ft","width_ft","surface","lighted","closed"
1,1,"KJFK",14511,200,"ASP",1,0
2,1,"KJFK",10000,150,"ASP",1,0
3,2,"KLGA",7003,150,"ASP",1,0
4,2,"KLGA",9999,150,"ASP",1,1
5,3,"XXXX",,150,"ASP",1,0
`
	got, err := parseRunwaysCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if got["KJFK"] != 14511 || got["KLGA"] != 7003 || got["XXXX"] != 0 {
		t.Errorf("runways = %v (closed and empty lengths must be ignored)", got)
	}
	for _, bad := range []string{"", "a,b\n1,2\n", csv + "6,4,\"SHORT\"\n"} {
		if _, err := parseRunwaysCSV(strings.NewReader(bad)); err == nil {
			t.Errorf("parseRunwaysCSV accepted %.30q", bad)
		}
	}
}

func TestWriteCandidatesIsCompactAndReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	in := map[string][]Candidate{"BBB": {{To: "AAA", Km: 200, DistFactor: 69.2}}, "AAA": {{To: "BBB", Km: 200, DistFactor: 69.2}}}
	if err := writeCandidates(path, in); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	want := "{\n\"AAA\":[[\"BBB\",200,69.2]],\n\"BBB\":[[\"AAA\",200,69.2]]\n}\n"
	if string(b) != want {
		t.Fatalf("file = %q", b)
	}
	var back map[string][]Candidate
	if err := json.Unmarshal(b, &back); err != nil || back["AAA"][0].To != "BBB" {
		t.Fatalf("round trip: %v %v", back, err)
	}
}

func TestCheckCandidatesFindsProblems(t *testing.T) {
	dir := t.TempDir()
	_ = writeJSON(filepath.Join(dir, "airports.json"), []Airport{{IATA: "AAA"}, {IATA: "BBB"}, {IATA: "CCC"}})
	_ = writeCandidates(filepath.Join(dir, "candidates.json"), map[string][]Candidate{
		"AAA": {{To: "AAA", Km: 200, DistFactor: round3(math.Pow(200, 0.8))}},
		"BBB": {{To: "AAA", Km: 200, DistFactor: 1}},
		"ZZZ": {{To: "AAA", Km: 200, DistFactor: 1}},
	})
	errs, err := checkCandidates(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, e := range errs {
		joined += e.Error() + "\n"
	}
	for _, want := range []string{"AAA → AAA is invalid", "BBB → AAA is invalid", "ZZZ is unknown"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	if _, err := checkCandidates(t.TempDir(), nil); err == nil {
		t.Error("missing files accepted")
	}
}
