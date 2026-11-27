// Package content defines the data-driven game content (ADR-0004): plane models,
// airport levels, balance coefficients, seasons and holidays. Content is authored
// as YAML in content/, validated by tools/contentc and compiled to JSON. The
// simulation only ever decodes JSON (no YAML parser in sim.wasm).
//
// Money is in int64 cents; rates and multipliers are float64 factors.
package content

// Content is the complete, validated game content.
type Content struct {
	Planes        []PlaneModel   `json:"planes"`
	AirportLevels []AirportLevel `json:"airportLevels"`
	Balance       Balance        `json:"balance"`
	Seasons       []Season       `json:"seasons"`
	Holidays      []Holiday      `json:"holidays"`
}

// PlaneModel is an aircraft type. Names are fictional (no manufacturer brands).
type PlaneModel struct {
	ID              string       `json:"id"`
	Tier            int          `json:"tier"`
	NameKey         string       `json:"nameKey"`
	SeatUnits       int          `json:"seatUnits"` // economy seats; a business seat uses 2 units
	RangeKm         int          `json:"rangeKm"`
	SpeedKmh        int          `json:"speedKmh"`
	MinRunwayFt     int          `json:"minRunwayFt"`
	MinAirportLevel int          `json:"minAirportLevel"`
	PriceCents      int64        `json:"priceCents"`
	UpkeepDayCents  int64        `json:"upkeepDayCents"`
	BurnPerKm       float64      `json:"burnPerKm"`   // fuel units per km
	Reliability     float64      `json:"reliability"` // base probability a flight goes as planned
	MaxAgeDays      int          `json:"maxAgeDays"`
	Levels          []PlaneLevel `json:"levels"` // upgrades, level 2 onwards
}

// PlaneLevel is one upgrade step of a plane model.
type PlaneLevel struct {
	CostCents   int64   `json:"costCents"`
	SeatUnits   int     `json:"seatUnits"`   // added seat units
	Efficiency  float64 `json:"efficiency"`  // added fuel saving (fraction)
	Reliability float64 `json:"reliability"` // added reliability
}

// AirportLevel describes one of the airport levels 1..5 (TECH_DESIGN §4).
type AirportLevel struct {
	Level               int   `json:"level"`
	PaxPerDay           int   `json:"paxPerDay"`
	RouteSlots          int   `json:"routeSlots"`
	MaxPlaneTier        int   `json:"maxPlaneTier"`
	UpgradeCostCents    int64 `json:"upgradeCostCents"` // to reach this level from the one below
	UpkeepDayCents      int64 `json:"upkeepDayCents"`
	FeePerPaxCents      int64 `json:"feePerPaxCents"`
	FeePerLandingCents  int64 `json:"feePerLandingCents"`
	LeasedDeparturesCap int   `json:"leasedDeparturesCap"` // per day, for airlines holding landing rights
}

// Season is a monthly demand profile for countries of one climate zone and hemisphere.
type Season struct {
	Climate    string      `json:"climate"`    // tropical, subtropical, temperate, cold
	Hemisphere string      `json:"hemisphere"` // north, south
	Tourism    [12]float64 `json:"tourism"`    // multiplier per month, January first
	Business   [12]float64 `json:"business"`
}

// Holiday raises or lowers demand in matching countries for a period of the game
// calendar (fixed dates; moving holidays use a representative date).
type Holiday struct {
	ID        string   `json:"id"`
	NameKey   string   `json:"nameKey"`
	Start     string   `json:"start"` // "MM-DD"
	Days      int      `json:"days"`
	Countries []string `json:"countries,omitempty"` // ISO2 codes
	Regions   []string `json:"regions,omitempty"`   // UN regions, e.g. "Europe"
	Tourism   float64  `json:"tourism"`
	Business  float64  `json:"business"`
}
