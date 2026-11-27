package main

import (
	"math"
	"sort"
	"strconv"
)

// Stats are the economic and geographic inputs for demand and pricing
// (TECH_DESIGN §7.0), stored on each Country.
type Stats struct {
	Population      int64   `json:"population"`
	GDPPerCapitaUSD float64 `json:"gdpPerCapitaUsd"`
	TourismArrivals int64   `json:"tourismArrivals"`
	// Business and Tourism are percentile ranks in [0, 1] of GDP per capita and of
	// tourist arrivals per resident across all countries.
	Business float64 `json:"business"`
	Tourism  float64 `json:"tourism"`
	// Wealth is GDP per capita / world median, clamped to [0.3, 3] (TECH_DESIGN §7.0).
	Wealth     float64 `json:"wealth"`
	Hemisphere string  `json:"hemisphere"` // "north" or "south" (label latitude)
	Climate    string  `json:"climate"`    // tropical, subtropical, temperate, cold
	// Sources says where each value came from, e.g. "wb:2024", "ne", "estimate".
	Sources map[string]string `json:"sources"`
}

// applyStats fills Stats on every country from the World Bank snapshot, falling
// back to Natural Earth estimates for population and GDP, and to the subregion's
// median arrivals per resident for tourism.
func applyStats(countries []Country, wb WorldBank) {
	pop := wb.Indicators[indPopulation]
	gdp := wb.Indicators[indGDPPerCap]
	arr := wb.Indicators[indTourismArrv]

	for i := range countries {
		c := &countries[i]
		s := Stats{Sources: map[string]string{}}
		if o, ok := pop[c.ISO2]; ok {
			s.Population, s.Sources["population"] = int64(o.Value), "wb:"+strconv.Itoa(o.Year)
		} else {
			s.Population, s.Sources["population"] = int64(max(c.nePop, 0)), "ne"
		}
		if o, ok := gdp[c.ISO2]; ok {
			s.GDPPerCapitaUSD, s.Sources["gdpPerCapita"] = o.Value, "wb:"+strconv.Itoa(o.Year)
		} else if c.nePop > 0 && c.neGDPMillions > 0 { // Natural Earth uses -99 for "no data"
			s.GDPPerCapitaUSD, s.Sources["gdpPerCapita"] = c.neGDPMillions*1e6/c.nePop, "ne"
		} else {
			s.Sources["gdpPerCapita"] = "none"
		}
		s.GDPPerCapitaUSD = math.Round(s.GDPPerCapitaUSD)
		if o, ok := arr[c.ISO2]; ok {
			s.TourismArrivals, s.Sources["tourism"] = int64(o.Value), "wb:"+strconv.Itoa(o.Year)
		}
		s.Hemisphere, s.Climate = climateOf(c.Label[1])
		c.Stats = s
	}
	estimateTourism(countries, arr)
	rank(countries, func(c *Country) float64 { return c.Stats.GDPPerCapitaUSD }, func(c *Country, r float64) { c.Stats.Business = r })
	rank(countries, arrivalsPerResident, func(c *Country, r float64) { c.Stats.Tourism = r })
}

func arrivalsPerResident(c *Country) float64 {
	if c.Stats.Population <= 0 {
		return 0
	}
	return float64(c.Stats.TourismArrivals) / float64(c.Stats.Population)
}

// estimateTourism fills missing arrivals with the subregion's median arrivals per
// resident (or the world median if the subregion has no data).
func estimateTourism(countries []Country, arr map[string]Observation) {
	bySub := map[string][]float64{}
	var world []float64
	for i := range countries {
		c := &countries[i]
		if _, ok := arr[c.ISO2]; ok && c.Stats.Population > 0 {
			v := arrivalsPerResident(c)
			bySub[c.Subregion] = append(bySub[c.Subregion], v)
			world = append(world, v)
		}
	}
	for i := range countries {
		c := &countries[i]
		if _, ok := arr[c.ISO2]; ok {
			continue
		}
		ratio := median(world)
		if vs := bySub[c.Subregion]; len(vs) > 0 {
			ratio = median(vs)
		}
		c.Stats.TourismArrivals = int64(math.Round(ratio * float64(c.Stats.Population)))
		c.Stats.Sources["tourism"] = "estimate"
	}
}

func median(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	s := append([]float64(nil), vs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// rank sets the percentile rank in [0, 1] (ties share their average rank), rounded
// to three decimals. Order is by value, then ISO2, so results are deterministic.
func rank(countries []Country, value func(*Country) float64, set func(*Country, float64)) {
	n := len(countries)
	if n < 2 {
		for i := range countries {
			set(&countries[i], 0.5)
		}
		return
	}
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool {
		va, vb := value(&countries[idx[a]]), value(&countries[idx[b]])
		if va != vb {
			return va < vb
		}
		return countries[idx[a]].ISO2 < countries[idx[b]].ISO2
	})
	for i := 0; i < n; {
		j := i
		v := value(&countries[idx[i]])
		for j+1 < n && value(&countries[idx[j+1]]) == v {
			j++
		}
		r := math.Round(float64(i+j)/2/float64(n-1)*1000) / 1000
		for k := i; k <= j; k++ {
			set(&countries[idx[k]], r)
		}
		i = j + 1
	}
}

// climateOf derives the hemisphere and a coarse climate zone from latitude.
func climateOf(lat float64) (hemisphere, zone string) {
	hemisphere = "north"
	if lat < 0 {
		hemisphere = "south"
	}
	switch a := math.Abs(lat); {
	case a < 23.44:
		zone = "tropical"
	case a < 35:
		zone = "subtropical"
	case a < 55:
		zone = "temperate"
	default:
		zone = "cold" // ≥ 55°: short summers (e.g. Scandinavia, Iceland, Canada's north)
	}
	return hemisphere, zone
}
