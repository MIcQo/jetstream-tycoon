package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func f64(v float64) *float64 { return &v }

func testCountries() []Country {
	return []Country{
		{ISO2: "FR", ISO3: "FRA", Subregion: "Western Europe", Label: [2]float64{2, 46.7}, nePop: 1, neGDPMillions: 1},
		{ISO2: "DE", ISO3: "DEU", Subregion: "Western Europe", Label: [2]float64{10, 51}},
		{ISO2: "XK", ISO3: "-99", Subregion: "Southern Europe", Label: [2]float64{21, 42.6}},
		{ISO2: "TW", ISO3: "TWN", Subregion: "Eastern Asia", Label: [2]float64{121, 23.7}, nePop: 23_000_000, neGDPMillions: 1_100_000},
		{ISO2: "AU", ISO3: "AUS", Subregion: "Oceania", Label: [2]float64{134, -24}},
	}
}

func row(id, iso3, date string, v *float64) wbRow {
	r := wbRow{ISO3: iso3, Date: date, Value: v}
	r.Country.ID = id
	return r
}

func TestLatestByCountry(t *testing.T) {
	rows := []wbRow{
		row("FR", "FRA", "2018", f64(1)),
		row("FR", "FRA", "2019", f64(2)),
		row("FR", "FRA", "2017", f64(3)), // older than 2019: ignored
		row("ZZ", "DEU", "2019", f64(4)), // unknown ISO2, matched by ISO3
		row("XK", "XKX", "2019", f64(5)), // Kosovo: matched by ISO2
		row("ZH", "AFE", "2019", f64(6)), // aggregate: skipped
		row("AU", "AUS", "2019", nil),    // null: skipped
		row("TW", "TWN", "n/a", f64(7)),  // bad year: skipped
	}
	got := latestByCountry(rows, testCountries())
	want := map[string]Observation{"FR": {2, 2019}, "DE": {4, 2019}, "XK": {5, 2019}}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %+v, want %+v", k, got[k], v)
		}
	}
}

func TestFetchWorldBank(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Path+"?"+r.URL.RawQuery)
		_, _ = w.Write([]byte(`[{"page":1},[{"country":{"id":"FR"},"countryiso3code":"FRA","date":"2019","value":42}]]`))
	}))
	defer srv.Close()
	wb, err := fetchWorldBank(srv.URL, testCountries(), time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if wb.Fetched != "2026-11-27" || wb.License != "CC BY 4.0" || len(wb.Indicators) != 3 {
		t.Fatalf("snapshot = %+v", wb)
	}
	if wb.Indicators[indTourismArrv]["FR"].Value != 42 {
		t.Errorf("tourism = %+v", wb.Indicators[indTourismArrv])
	}
	joined := strings.Join(queries, "\n")
	if !strings.Contains(joined, "/indicator/ST.INT.ARVL?") || !strings.Contains(joined, "date=2010:2019") || !strings.Contains(joined, "mrnev=1") {
		t.Errorf("queries = %s", joined)
	}
}

func TestGetWorldBankErrors(t *testing.T) {
	cases := map[string]func(http.ResponseWriter){
		"HTTP 500":                        func(w http.ResponseWriter) { w.WriteHeader(500) },
		"unexpected response shape":       func(w http.ResponseWriter) { _, _ = w.Write([]byte(`[{"message":"bad"}]`)) },
		"invalid character":               func(w http.ResponseWriter) { _, _ = w.Write([]byte(`nope`)) },
		"cannot unmarshal object into Go": func(w http.ResponseWriter) { _, _ = w.Write([]byte(`[{},{}]`)) },
	}
	for want, h := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { h(w) }))
		_, err := fetchWorldBank(srv.URL, testCountries(), time.Now())
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want error containing %q, got %v", want, err)
		}
	}
}

func TestApplyStats(t *testing.T) {
	countries := testCountries()
	wb := WorldBank{Indicators: map[string]map[string]Observation{
		indPopulation:  {"FR": {68_000_000, 2025}, "DE": {84_000_000, 2025}, "XK": {1_600_000, 2025}, "AU": {27_000_000, 2025}},
		indGDPPerCap:   {"FR": {49_000.4, 2025}, "DE": {55_000, 2025}, "XK": {7_900, 2025}, "AU": {65_000, 2025}},
		indTourismArrv: {"FR": {218_000_000, 2019}, "DE": {39_000_000, 2019}, "AU": {9_500_000, 2019}},
	}}
	applyStats(countries, wb)
	by := map[string]Stats{}
	for _, c := range countries {
		by[c.ISO2] = c.Stats
	}

	fr := by["FR"]
	if fr.Population != 68_000_000 || fr.GDPPerCapitaUSD != 49_000 || fr.Sources["tourism"] != "wb:2019" || fr.Hemisphere != "north" || fr.Climate != "temperate" {
		t.Errorf("FR = %+v", fr)
	}
	tw := by["TW"] // not in the World Bank: Natural Earth fallback
	if tw.Population != 23_000_000 || tw.GDPPerCapitaUSD != 47_826 || tw.Sources["population"] != "ne" || tw.Sources["gdpPerCapita"] != "ne" {
		t.Errorf("TW = %+v", tw)
	}
	// Tourism estimates: XK has no subregion data → world median; TW likewise.
	if by["XK"].Sources["tourism"] != "estimate" || by["XK"].TourismArrivals <= 0 {
		t.Errorf("XK tourism = %+v", by["XK"])
	}
	// Business follows GDP per capita: AU (65k) highest, XK lowest.
	if by["AU"].Business != 1 || by["XK"].Business != 0 {
		t.Errorf("business ranks: AU %v XK %v", by["AU"].Business, by["XK"].Business)
	}
	// Tourism per resident: FR (3.2) highest.
	if by["FR"].Tourism != 1 {
		t.Errorf("FR tourism rank = %v", by["FR"].Tourism)
	}
	if by["AU"].Hemisphere != "south" || by["TW"].Climate != "subtropical" {
		t.Errorf("climate: AU %+v TW %+v", by["AU"], by["TW"])
	}

	// A country with no data at all (or Natural Earth's -99 "no data" markers) gets
	// "none" for GDP and a zero GDP and population.
	empty := []Country{{ISO2: "QQ", Subregion: "Nowhere", nePop: -99, neGDPMillions: -99}}
	applyStats(empty, WorldBank{Indicators: map[string]map[string]Observation{}})
	if s := empty[0].Stats; s.Sources["gdpPerCapita"] != "none" || s.GDPPerCapitaUSD != 0 || s.Population != 0 || s.Business != 0.5 {
		t.Errorf("empty = %+v", s)
	}
}

func TestEstimateTourismUsesSubregionMedian(t *testing.T) {
	cs := []Country{
		{ISO2: "A", Subregion: "S", Stats: Stats{Population: 100, TourismArrivals: 100, Sources: map[string]string{}}},
		{ISO2: "B", Subregion: "S", Stats: Stats{Population: 100, TourismArrivals: 300, Sources: map[string]string{}}},
		{ISO2: "C", Subregion: "S", Stats: Stats{Population: 10, Sources: map[string]string{}}},
	}
	estimateTourism(cs, map[string]Observation{"A": {}, "B": {}})
	if cs[2].Stats.TourismArrivals != 20 || cs[2].Stats.Sources["tourism"] != "estimate" { // median(1, 3) = 2 per resident
		t.Errorf("C = %+v", cs[2].Stats)
	}
}

func TestMedianRankClimate(t *testing.T) {
	if median(nil) != 0 || median([]float64{3, 1, 2}) != 2 || median([]float64{4, 1, 3, 2}) != 2.5 {
		t.Error("median")
	}
	cs := []Country{{ISO2: "B"}, {ISO2: "A"}, {ISO2: "C"}, {ISO2: "D"}}
	vals := map[string]float64{"A": 5, "B": 1, "C": 5, "D": 9}
	got := map[string]float64{}
	rank(cs, func(c *Country) float64 { return vals[c.ISO2] }, func(c *Country, r float64) { got[c.ISO2] = r })
	if got["B"] != 0 || got["A"] != 0.5 || got["C"] != 0.5 || got["D"] != 1 { // A and C tie at ranks 1 and 2 → 0.5
		t.Errorf("ranks = %v", got)
	}
	one := []Country{{ISO2: "A"}}
	rank(one, func(*Country) float64 { return 1 }, func(c *Country, r float64) { got["one"] = r })
	if got["one"] != 0.5 {
		t.Errorf("single rank = %v", got["one"])
	}
	cases := []struct {
		lat        float64
		hemi, zone string
	}{{0, "north", "tropical"}, {-23.4, "south", "tropical"}, {23.44, "north", "subtropical"}, {-35, "south", "temperate"}, {54.9, "north", "temperate"}, {55, "north", "cold"}, {-70, "south", "cold"}}
	for _, c := range cases {
		if h, z := climateOf(c.lat); h != c.hemi || z != c.zone {
			t.Errorf("climateOf(%v) = %s %s", c.lat, h, z)
		}
	}
}
