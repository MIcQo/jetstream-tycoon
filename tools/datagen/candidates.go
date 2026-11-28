package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

const (
	// candidatesPerAirport is how many destinations the AI considers per airport
	// (ADR-0020); the player can still open any route. longHaulSlots of them are
	// reserved for destinations at least longHaulKm away, so short hops (which
	// dominate the gravity score) don't crowd out intercontinental routes.
	candidatesPerAirport = 30
	longHaulSlots        = 10
	longHaulKm           = 3000
	// minRouteKm excludes pairs too close for a sensible air route.
	minRouteKm = 100
	// distExponent and popExponent are the fixed exponents of the base demand
	// formula (TECH_DESIGN §7.0), precomputed here per ADR-0021.
	distExponent = 0.8
	popExponent  = 0.5
	gdpExponent  = 0.7
	// Wealth is GDP per capita relative to the world median, clamped (TECH_DESIGN §7.0).
	minWealth = 0.3
	maxWealth = 3.0
)

// Candidate is one destination from an airport. It is written as a compact JSON
// array [to, km, distFactor] because the file is embedded in sim.wasm.
type Candidate struct {
	To         string
	Km         int     // great-circle distance, rounded
	DistFactor float64 // km^0.8, for the demand formula
}

// MarshalJSON writes [to, km, distFactor].
func (c Candidate) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{c.To, c.Km, c.DistFactor})
}

// UnmarshalJSON reads [to, km, distFactor].
func (c *Candidate) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if len(raw) != 3 {
		return fmt.Errorf("candidate: want [to, km, distFactor], got %d elements", len(raw))
	}
	if err := json.Unmarshal(raw[0], &c.To); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[1], &c.Km); err != nil {
		return err
	}
	return json.Unmarshal(raw[2], &c.DistFactor)
}

// applyWealth sets each country's wealth factor from its GDP per capita and the
// median over countries that have GDP data.
func applyWealth(countries []Country) {
	var gdps []float64
	for _, c := range countries {
		if c.Stats.GDPPerCapitaUSD > 0 {
			gdps = append(gdps, c.Stats.GDPPerCapitaUSD)
		}
	}
	med := median(gdps)
	for i := range countries {
		w := 1.0
		if g := countries[i].Stats.GDPPerCapitaUSD; g > 0 && med > 0 {
			w = math.Min(maxWealth, math.Max(minWealth, g/med))
		}
		countries[i].Stats.Wealth = round3(w)
		countries[i].Stats.PriceBase = priceBase(countries[i].Stats)
	}
}

// priceBase is pop^0.5 · gdpPerCapita^0.7, rounded to an integer.
func priceBase(s Stats) float64 {
	return math.Round(math.Pow(float64(max(s.Population, 0)), popExponent) * math.Pow(math.Max(s.GDPPerCapitaUSD, 0), gdpExponent))
}

// applyPopFactors sets cityPop^0.5 on every city.
func applyPopFactors(cities []City) {
	for i := range cities {
		cities[i].PopFactor = round3(math.Pow(float64(cities[i].Pop), popExponent))
	}
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

// buildCandidates ranks, for every airport, the destination cities by the static
// part of the base demand formula, popFactor_A · popFactor_B / km^0.8 · mean
// wealth. Each destination city is represented by its airport closest to the city
// centre (CDG rather than Beauvais for Paris). The best candidatesPerAirport are
// kept, longHaulSlots of them reserved for long-haul destinations. Same-city and
// very short pairs are excluded, and ties break on IATA code (deterministic).
func buildCandidates(airports []Airport, cities []City, countries []Country) (map[string][]Candidate, error) {
	cityByID := map[string]City{}
	for _, c := range cities {
		cityByID[c.ID] = c
	}
	wealth := map[string]float64{}
	for _, c := range countries {
		wealth[c.ISO2] = c.Stats.Wealth
	}
	// The representative airport of each city: its main hub.
	rep := map[string]Airport{}
	for _, a := range airports {
		c, ok := cityByID[a.CityID]
		if !ok {
			return nil, fmt.Errorf("airport %s: unknown city %s", a.IATA, a.CityID)
		}
		if cur, ok := rep[a.CityID]; !ok || isMainHub(a, cur, c) {
			rep[a.CityID] = a
		}
	}
	reps := make([]Airport, 0, len(rep))
	for _, id := range sortedKeys(rep) {
		reps = append(reps, rep[id])
	}

	type scored struct {
		c     Candidate
		score float64
	}
	better := func(x, y scored) bool {
		if x.score != y.score {
			return x.score > y.score
		}
		return x.c.To < y.c.To
	}
	out := make(map[string][]Candidate, len(airports))
	for _, a := range airports {
		ca := cityByID[a.CityID]
		var short, long []scored
		for _, b := range reps {
			if b.CityID == a.CityID {
				continue
			}
			km := haversineKm(a.Lat, a.Lon, b.Lat, b.Lon)
			if km < minRouteKm {
				continue
			}
			// distFactor uses the rounded km, so anything recomputing it from the
			// stored km (checks, the simulation) gets the same value.
			roundedKm := int(math.Round(km))
			distFactor := math.Pow(float64(roundedKm), distExponent)
			score := ca.PopFactor * cityByID[b.CityID].PopFactor / distFactor * (wealth[a.ISO2] + wealth[b.ISO2]) / 2
			s := scored{Candidate{To: b.IATA, Km: roundedKm, DistFactor: round3(distFactor)}, score}
			if km >= longHaulKm {
				long = append(long, s)
			} else {
				short = append(short, s)
			}
		}
		sort.Slice(short, func(i, j int) bool { return better(short[i], short[j]) })
		sort.Slice(long, func(i, j int) bool { return better(long[i], long[j]) })
		// Reserve long-haul slots, give the rest to the best remaining candidates.
		nLong := min(longHaulSlots, len(long))
		picked := append([]scored(nil), long[:nLong]...)
		rest := append(append([]scored(nil), short...), long[nLong:]...)
		sort.Slice(rest, func(i, j int) bool { return better(rest[i], rest[j]) })
		picked = append(picked, rest[:min(candidatesPerAirport-nLong, len(rest))]...)
		sort.Slice(picked, func(i, j int) bool { return better(picked[i], picked[j]) })
		cands := make([]Candidate, len(picked))
		for i := range picked {
			cands[i] = picked[i].c
		}
		out[a.IATA] = cands
	}
	return out, nil
}

// writeCandidates writes one airport per line, so diffs stay readable while the
// file stays compact.
func writeCandidates(path string, cands map[string][]Candidate) error {
	var b []byte
	b = append(b, "{\n"...)
	keys := sortedKeys(cands)
	for i, k := range keys {
		line, err := json.Marshal(cands[k])
		if err != nil {
			return err
		}
		b = append(b, fmt.Sprintf("%q:%s", k, line)...)
		if i < len(keys)-1 {
			b = append(b, ',')
		}
		b = append(b, '\n')
	}
	b = append(b, "}\n"...)
	return os.WriteFile(path, b, 0o644)
}

// isMainHub reports whether a is a better representative of city c than cur:
// longer runway, then closer to the centre, then lower IATA code.
func isMainHub(a, cur Airport, c City) bool {
	if a.RunwayFt != cur.RunwayFt {
		return a.RunwayFt > cur.RunwayFt
	}
	da := haversineKm(a.Lat, a.Lon, c.Lat, c.Lon)
	dc := haversineKm(cur.Lat, cur.Lon, c.Lat, c.Lon)
	if da != dc {
		return da < dc
	}
	return a.IATA < cur.IATA
}
