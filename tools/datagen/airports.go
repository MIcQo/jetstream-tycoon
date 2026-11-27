package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// Airport is one playable airport (content/world/airports.json).
type Airport struct {
	IATA      string  `json:"iata"`
	ICAO      string  `json:"icao"`
	Name      string  `json:"name"`
	ISO2      string  `json:"iso2"`
	CityID    string  `json:"cityId"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Size      string  `json:"size"`      // "large" or "medium" (OurAirports type)
	BaseLevel int     `json:"baseLevel"` // real-world level (TECH_DESIGN §4): large 3, medium 2
	// RunwayFt is the longest open runway; it identifies a city's main hub
	// (JFK 14,511 ft vs LGA 7,003 ft) and can later limit aircraft size.
	RunwayFt int `json:"runwayFt"`
}

// City is the population centre an airport serves (content/world/cities.json).
// Airports matched to the same place share a city, so demand can be split
// between them (e.g. LHR and LGW).
type City struct {
	ID   string `json:"id"` // "ne:<NE_ID>" or "oa:<airport ident>" when unmatched
	Name string `json:"name"`
	ISO2 string `json:"iso2"`
	Pop  int64  `json:"pop"`
	// PopFactor is Pop^0.5, precomputed for the demand formula (ADR-0021).
	PopFactor float64 `json:"popFactor"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Matched   bool    `json:"matched"` // false: no Natural Earth place within cityMatchKm

	names map[string]string // localized names (see names.go); nil for unmatched cities
}

const (
	// cityMatchKm is the search radius for the city an airport serves.
	cityMatchKm = 80.0
	// cityScaleKm controls the gravity score pop / (1 + (d/cityScaleKm)²): a large
	// metro a little farther away beats a small adjacent town (Gatwick → London, not
	// Crawley), while airports in separate cities keep their own (Rotterdam, Sharjah).
	cityScaleKm = 25.0
	// minCityPop keeps demand above zero for airports with no matched city
	// (remote islands, resorts) and for very small matched places.
	minCityPop = 50_000
	earthKm    = 6371.0088
)

type place struct {
	iso      string // ISO2 of the place's country, "" when Natural Earth has none
	id       string
	name     string
	names    map[string]string // localized names by language code
	pop      int64
	lat, lon float64
}

// rawAirport is one OurAirports row.
type rawAirport struct {
	ident, typ, name, iso, municipality, sched, icao, iata string
	lat, lon                                               float64
}

func buildAirports(cache string, playable map[string]bool) ([]Airport, []City, error) {
	rows, err := readAirportsCSV(filepath.Join(cache, "airports.csv"))
	if err != nil {
		return nil, nil, err
	}
	places, err := readPlaces(filepath.Join(cache, "ne_10m_populated_places.geojson"))
	if err != nil {
		return nil, nil, err
	}
	runways, err := readRunways(filepath.Join(cache, "runways.csv"))
	if err != nil {
		return nil, nil, err
	}
	picked := selectAirports(rows, places, playable)

	cities := map[string]City{}
	var airports []Airport
	for _, r := range picked {
		city := matchCity(r, places)
		if _, ok := cities[city.ID]; !ok {
			cities[city.ID] = city
		}
		size, level := "large", 3
		if r.typ == "medium_airport" {
			size, level = "medium", 2
		}
		icao := r.icao
		if icao == "" {
			icao = r.ident
		}
		airports = append(airports, Airport{
			IATA: r.iata, ICAO: icao, Name: r.name, ISO2: r.iso, CityID: city.ID,
			Lat: round6(r.lat), Lon: round6(r.lon), Size: size, BaseLevel: level,
			RunwayFt: runways[r.ident],
		})
	}
	sort.Slice(airports, func(i, j int) bool { return airports[i].IATA < airports[j].IATA })
	cityList := make([]City, 0, len(cities))
	for _, c := range cities {
		cityList = append(cityList, c)
	}
	sort.Slice(cityList, func(i, j int) bool { return cityList[i].ID < cityList[j].ID })
	return airports, cityList, nil
}

// selectAirports keeps every scheduled large airport with an IATA code, plus, for
// each playable country without one, its scheduled medium airport serving the
// largest city, so that every country with commercial service is playable.
func selectAirports(rows []rawAirport, places []place, playable map[string]bool) []rawAirport {
	var out []rawAirport
	hasLarge := map[string]bool{}
	bestMedium := map[string]rawAirport{}
	bestPop := map[string]int64{}
	for _, r := range rows {
		if r.sched != "yes" || r.iata == "" || !playable[r.iso] {
			continue
		}
		switch r.typ {
		case "large_airport":
			out = append(out, r)
			hasLarge[r.iso] = true
		case "medium_airport":
			pop := matchCity(r, places).Pop
			if b, ok := bestMedium[r.iso]; !ok || pop > bestPop[r.iso] || (pop == bestPop[r.iso] && r.iata < b.iata) {
				bestMedium[r.iso], bestPop[r.iso] = r, pop
			}
		}
	}
	for _, iso := range sortedKeys(bestMedium) {
		if !hasLarge[iso] {
			out = append(out, bestMedium[iso])
		}
	}
	// Process airports in IATA order so city attributes never depend on map order.
	sort.Slice(out, func(i, j int) bool { return out[i].iata < out[j].iata })
	return out
}

// placeISO returns the place's ISO2 code, or "" when Natural Earth has none ("-99").
func placeISO(f Feature) string {
	if c := f.str("ISO_A2"); len(c) == 2 && c != "-9" {
		return c
	}
	return ""
}

// matchCity returns the Natural Earth place in the airport's own country within
// cityMatchKm with the highest gravity score, or a city built from the airport's
// own municipality. Restricting to the same country keeps, for example, Jersey's
// airport from serving a town in Guernsey.
func matchCity(r rawAirport, places []place) City {
	best, bestScore := -1, 0.0
	for i, p := range places {
		if p.iso != "" && p.iso != r.iso {
			continue
		}
		d := haversineKm(r.lat, r.lon, p.lat, p.lon)
		if d > cityMatchKm {
			continue
		}
		k := d / cityScaleKm
		score := float64(max(p.pop, 1)) / (1 + k*k)
		if score > bestScore { // places are sorted by id, so ties keep the lowest id
			best, bestScore = i, score
		}
	}
	if best >= 0 {
		p := places[best]
		return City{ID: p.id, Name: p.name, ISO2: r.iso, Pop: max(p.pop, minCityPop), Lat: round6(p.lat), Lon: round6(p.lon), Matched: true, names: p.names}
	}
	name := r.municipality
	if name == "" {
		name = r.name
	}
	return City{ID: "oa:" + r.ident, Name: name, ISO2: r.iso, Pop: minCityPop, Lat: round6(r.lat), Lon: round6(r.lon)}
}

// haversineKm is the great-circle distance in kilometres.
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const rad = math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthKm * math.Asin(math.Min(1, math.Sqrt(a)))
}

func readPlaces(path string) ([]place, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	feats, err := parsePoints(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	out := make([]place, 0, len(feats))
	for _, f := range feats {
		out = append(out, place{
			id:    "ne:" + strconv.FormatInt(int64(f.num("NE_ID")), 10),
			iso:   placeISO(f),
			name:  f.str("NAME"),
			names: localizedNames(f),
			pop:   int64(f.num("POP_MAX")),
			lat:   f.num("LATITUDE"),
			lon:   f.num("LONGITUDE"),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out, nil
}

func readAirportsCSV(path string) ([]rawAirport, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return parseAirportsCSV(f)
}

func parseAirportsCSV(r io.Reader) ([]rawAirport, error) {
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("airports.csv header: %w", err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[h] = i
	}
	for _, need := range []string{"ident", "type", "name", "latitude_deg", "longitude_deg", "iso_country", "municipality", "scheduled_service", "icao_code", "iata_code"} {
		if _, ok := col[need]; !ok {
			return nil, fmt.Errorf("airports.csv: missing column %q", need)
		}
	}
	var out []rawAirport
	for line := 2; ; line++ {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("airports.csv line %d: %w", line, err)
		}
		lat, err1 := strconv.ParseFloat(rec[col["latitude_deg"]], 64)
		lon, err2 := strconv.ParseFloat(rec[col["longitude_deg"]], 64)
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("airports.csv line %d: bad coordinates", line)
		}
		out = append(out, rawAirport{
			ident: rec[col["ident"]], typ: rec[col["type"]], name: rec[col["name"]],
			iso: rec[col["iso_country"]], municipality: rec[col["municipality"]],
			sched: rec[col["scheduled_service"]], icao: rec[col["icao_code"]],
			iata: rec[col["iata_code"]], lat: lat, lon: lon,
		})
	}
}

// readRunways returns the longest open runway (feet) per airport ident.
func readRunways(path string) (map[string]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return parseRunwaysCSV(f)
}

func parseRunwaysCSV(r io.Reader) (map[string]int, error) {
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("runways.csv header: %w", err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[h] = i
	}
	for _, need := range []string{"airport_ident", "length_ft", "closed"} {
		if _, ok := col[need]; !ok {
			return nil, fmt.Errorf("runways.csv: missing column %q", need)
		}
	}
	out := map[string]int{}
	for line := 2; ; line++ {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("runways.csv line %d: %w", line, err)
		}
		if rec[col["closed"]] == "1" {
			continue
		}
		length, err := strconv.Atoi(rec[col["length_ft"]])
		if err != nil {
			continue // some runways have no recorded length
		}
		if id := rec[col["airport_ident"]]; length > out[id] {
			out[id] = length
		}
	}
}
