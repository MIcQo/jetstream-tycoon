package content

import (
	"encoding/json"
	"fmt"
)

// World is the subset of content/world the simulation needs. Display data (names,
// map paths, label points) stays in the UI files.
type World struct {
	Countries  []Country              `json:"countries"`
	Airports   []Airport              `json:"airports"`
	Cities     []City                 `json:"cities"`
	Candidates map[string][]Candidate `json:"candidates"`
}

// Country is a playable country with its economic and climate data.
type Country struct {
	ISO2      string       `json:"iso2"`
	Region    string       `json:"region"`
	Subregion string       `json:"subregion"`
	Stats     CountryStats `json:"stats"`
}

// CountryStats are precomputed by tools/datagen (see content/world/ATTRIBUTION.md).
type CountryStats struct {
	Population      int64   `json:"population"`
	GDPPerCapitaUSD float64 `json:"gdpPerCapitaUsd"`
	TourismArrivals int64   `json:"tourismArrivals"`
	Business        float64 `json:"business"`
	Tourism         float64 `json:"tourism"`
	Wealth          float64 `json:"wealth"`
	PriceBase       float64 `json:"priceBase"`
	Hemisphere      string  `json:"hemisphere"`
	Climate         string  `json:"climate"`
}

// Airport is a playable airport.
type Airport struct {
	IATA      string  `json:"iata"`
	ISO2      string  `json:"iso2"`
	CityID    string  `json:"cityId"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Size      string  `json:"size"`
	BaseLevel int     `json:"baseLevel"`
	RunwayFt  int     `json:"runwayFt"`
}

// City is the population centre airports serve.
type City struct {
	ID        string  `json:"id"`
	ISO2      string  `json:"iso2"`
	Pop       int64   `json:"pop"`
	PopFactor float64 `json:"popFactor"`
}

// Candidate is a destination considered from an airport, stored as [to, km, distFactor].
type Candidate struct {
	To         string
	Km         int
	DistFactor float64
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
