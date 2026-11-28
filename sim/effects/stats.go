package effects

// Stat names. Every modifier targets one of these; each has the range its final
// value is clamped to (TECH_DESIGN §6, step 4), which bounds stacked bonuses.
const (
	StatDemand         = "demand"         // demand multiplier
	StatFuelPrice      = "fuelPrice"      // fuel price multiplier
	StatReliability    = "reliability"    // flight success multiplier
	StatUpkeep         = "upkeep"         // upkeep multiplier
	StatAttractiveness = "attractiveness" // attractiveness multiplier
	StatRevenue        = "revenue"        // ticket revenue multiplier
	StatAirportClosed  = "airportClosed"  // 1 = closed (set only)
	StatRouteSuspended = "routeSuspended" // 1 = suspended (set only)
)

// Range is the inclusive range a stat's final value is clamped to.
type Range struct{ Min, Max float64 }

// Stats is the stat registry.
var Stats = map[string]Range{
	StatDemand:         {0.2, 3},
	StatFuelPrice:      {0.4, 3},
	StatReliability:    {0.5, 1.2},
	StatUpkeep:         {0.5, 3},
	StatAttractiveness: {0.2, 3},
	StatRevenue:        {0.2, 3},
	StatAirportClosed:  {0, 1},
	StatRouteSuspended: {0, 1},
}
