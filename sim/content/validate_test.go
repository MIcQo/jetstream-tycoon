package content_test

import (
	"math"
	"strings"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/content"
)

func refs() content.Refs {
	return content.Refs{
		Countries: map[string]bool{"US": true, "JP": true},
		Regions:   map[string]bool{"Europe": true},
		I18nKeys:  map[string]bool{"planes.a.name": true, "planes.b.name": true, "holidays.h.name": true},
	}
}

func season(climate, hemi string) content.Season {
	s := content.Season{Climate: climate, Hemisphere: hemi}
	for m := range s.Tourism {
		s.Tourism[m], s.Business[m] = 1, 1
	}
	return s
}

// valid returns content that passes Validate; each test breaks one thing.
func valid() content.Content {
	levels := make([]content.AirportLevel, 5)
	for i := range levels {
		levels[i] = content.AirportLevel{
			Level: i + 1, PaxPerDay: 1000 * (i + 1), RouteSlots: 2 * (i + 1), MaxPlaneTier: i + 1,
			UpgradeCostCents: int64(i) * 1_000_000_00, UpkeepDayCents: 100, FeePerPaxCents: 100,
			FeePerLandingCents: 100, LeasedDeparturesCap: 2,
		}
	}
	plane := func(id string, tier int) content.PlaneModel {
		return content.PlaneModel{
			ID: id, Tier: tier, NameKey: "planes." + id + ".name", SeatUnits: 50, RangeKm: 1500, SpeedKmh: 500,
			MinRunwayFt: 4000, MinAirportLevel: tier, PriceCents: 18_000_000_00, UpkeepDayCents: 3_500_00,
			BurnPerKm: 1.6, Reliability: 0.96, MaxAgeDays: 10950,
			Levels: []content.PlaneLevel{{CostCents: 1_000_000_00, SeatUnits: 4, Efficiency: 0.03, Reliability: 0.005}},
		}
	}
	var seasons []content.Season
	for _, c := range []string{"tropical", "subtropical", "temperate", "cold"} {
		for _, h := range []string{"north", "south"} {
			seasons = append(seasons, season(c, h))
		}
	}
	return content.Content{
		Planes:        []content.PlaneModel{plane("a", 1), plane("b", 2)},
		AirportLevels: levels,
		Balance:       validBalance(),
		Seasons:       seasons,
		Holidays: []content.Holiday{{
			ID: "h", NameKey: "holidays.h.name", Start: "12-20", Days: 14,
			Countries: []string{"US"}, Regions: []string{"Europe"}, Tourism: 1.3, Business: 0.6,
		}},
	}
}

func validBalance() content.Balance {
	return content.Balance{
		Demand:     content.DemandBalance{D0: 0.04, BusinessShareMin: 0.05, BusinessShareMax: 0.2, ConnectFactor: 0.4, ConnectingDiscount: 0.85, MediumAirportShare: 0.6},
		Pricing:    content.PricingBalance{RefBaseCents: 2500, RefPerKmCents: 9, BizMultiplier: 3.5, ElasticityEco: 1.5, ElasticityBiz: 0.8, PriceFactorMin: 0.1, PriceFactorMax: 2},
		Service:    content.ServiceBalance{Base: 0.7, ConditionWeight: 0.3, PlaneLevelWeight: 0.05, AirportLevelWeight: 0.03, BusinessMultiplier: 2},
		Reputation: content.ReputationBalance{Start: 50, DelayPenalty: 0.2, CancelPenalty: 1, OnTimeTarget: 0.9, GainPerPoint: 0.02, DecayPerDay: 0.005},
		Flights:    content.FlightBalance{TurnaroundTicks: 1, DelayShareOfFailures: 0.7, DelayRevenueFactor: 0.5, ConditionReliability: 0.4, CancelRebookShare: 0.5},
		Aging:      content.AgingBalance{ConditionDecayPerHour: 0.0002, OverhaulCostShare: 0.08, OverhaulDays: 5, DailyDepreciation: 0.99965, ReconfigureTicks: 48},
		Fuel:       content.FuelBalance{MeanCentsPerUnit: 80, Theta: 0.05, Sigma: 0.02, ShockChance: 0.01, ShockMin: 0.2, ShockMax: 0.6, ShockDays: 20, MinFactor: 0.4, MaxFactor: 3},
		Countries:  content.CountryBalance{PriceK: 5000, UpkeepShare: 0.0005, StartCashCents: 20_000_000_00, StartCashMinCents: 10_000_000_00, StartCashMaxCents: 60_000_000_00, StartCashExponent: 0.3, OwnerFeeShare: 0.3, ActiveServiceDays: 7},
		Leases:     content.LeaseBalance{UpfrontShareMin: 0.05, UpfrontShareMax: 0.15, YearlyLeaseShare: 0.3, LeasedFeeMultiplier: 1.5},
		Finance: content.FinanceBalance{BaseRate: 0.04, OverdraftRate: 0.25, LiquidationDays: 14, Restructurings: 1, MaxLoanWeeks: 520,
			CreditTiers: []content.CreditTier{{MinScore: 800, Spread: 0.01}, {MinScore: 350, Spread: 0.1}, {MinScore: 0, Spread: 0.1, NoLoans: true}}},
		Competition: content.CompetitionBalance{PriceAdjust: 0.1, UndercutShare: 0.6, UndercutDays: 7, UndercutMin: 0.05, UndercutMax: 0.1},
	}
}

func TestValidContentPasses(t *testing.T) {
	if err := content.Validate(valid(), refs()); err != nil {
		t.Fatalf("valid content rejected:\n%v", err)
	}
}

// expect applies a change and checks Validate reports exactly the given problem.
func expect(t *testing.T, name, want string, change func(*content.Content)) {
	t.Helper()
	c := valid()
	change(&c)
	err := content.Validate(c, refs())
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("%s: want error containing %q, got %v", name, want, err)
	}
}

// accept applies a change that must still validate (boundary values).
func accept(t *testing.T, name string, change func(*content.Content)) {
	t.Helper()
	c := valid()
	change(&c)
	if err := content.Validate(c, refs()); err != nil {
		t.Errorf("%s: boundary value rejected: %v", name, err)
	}
}

func TestNumericBoundaries(t *testing.T) {
	// Range limits are inclusive on both ends; NaN is never valid.
	accept(t, "reliability at lower bound", func(c *content.Content) { c.Planes[0].Reliability = 0.5; c.Planes[0].Levels = nil })
	accept(t, "reliability at upper bound", func(c *content.Content) { c.Planes[0].Reliability = 1; c.Planes[0].Levels = nil })
	expect(t, "reliability below", "reliability = 0.49", func(c *content.Content) { c.Planes[0].Reliability = 0.49 })
	expect(t, "reliability NaN", "reliability = NaN", func(c *content.Content) { c.Planes[0].Reliability = math.NaN() })
	expect(t, "reliability above at max level", "reliability at max level", func(c *content.Content) { c.Planes[0].Reliability = 0.999 })
	accept(t, "tier at lower bound", func(c *content.Content) { c.Planes[0].Tier = 1 })
	accept(t, "seat units at bounds", func(c *content.Content) { c.Planes[0].SeatUnits = 10; c.Planes[1].SeatUnits = 1000 })
	expect(t, "seat units below", "seatUnits = 9", func(c *content.Content) { c.Planes[0].SeatUnits = 9 })
	expect(t, "seat units above", "seatUnits = 1001", func(c *content.Content) { c.Planes[0].SeatUnits = 1001 })
}

func TestPlanes(t *testing.T) {
	expect(t, "none", "at least one plane", func(c *content.Content) { c.Planes = nil })
	expect(t, "bad id", `id "Bad-ID" must be snake_case`, func(c *content.Content) { c.Planes[0].ID = "Bad-ID" })
	expect(t, "duplicate id", `duplicate id "a"`, func(c *content.Content) { c.Planes[1].ID = "a"; c.Planes[1].NameKey = "planes.a.name" })
	expect(t, "missing i18n key", `i18n key "planes.zzz.name" not found`, func(c *content.Content) { c.Planes[0].NameKey = "planes.zzz.name" })
	expect(t, "tier", "tier = 6", func(c *content.Content) { c.Planes[0].Tier = 6 })
	expect(t, "range", "rangeKm = 299", func(c *content.Content) { c.Planes[0].RangeKm = 299 })
	expect(t, "speed", "speedKmh = 1101", func(c *content.Content) { c.Planes[0].SpeedKmh = 1101 })
	expect(t, "runway", "minRunwayFt = -1", func(c *content.Content) { c.Planes[0].MinRunwayFt = -1 })
	expect(t, "min airport level", "minAirportLevel = 0", func(c *content.Content) { c.Planes[0].MinAirportLevel = 0 })
	expect(t, "price", "priceCents = 99", func(c *content.Content) { c.Planes[0].PriceCents = 99 })
	expect(t, "upkeep", "upkeepDayCents = -1", func(c *content.Content) { c.Planes[0].UpkeepDayCents = -1 })
	expect(t, "burn", "burnPerKm = 0.05", func(c *content.Content) { c.Planes[0].BurnPerKm = 0.05 })
	expect(t, "max age", "maxAgeDays = 100", func(c *content.Content) { c.Planes[0].MaxAgeDays = 100 })
	expect(t, "level cost", "levels[0].costCents = 0", func(c *content.Content) { c.Planes[0].Levels[0].CostCents = 0 })
	expect(t, "level seats", "levels[0].seatUnits = 101", func(c *content.Content) { c.Planes[0].Levels[0].SeatUnits = 101 })
	expect(t, "level efficiency", "levels[0].efficiency = 0.31", func(c *content.Content) { c.Planes[0].Levels[0].Efficiency = 0.31 })
	expect(t, "level reliability", "levels[0].reliability = 0.06", func(c *content.Content) { c.Planes[0].Levels[0].Reliability = 0.06 })
	// Tier must fit the plane's minimum airport level.
	expect(t, "tier too big for level", "tier 2 does not fit airport level 1", func(c *content.Content) { c.Planes[1].MinAirportLevel = 1 })
	accept(t, "tier equal to level max tier", func(c *content.Content) { c.Planes[1].MinAirportLevel = 2 })
	accept(t, "out-of-range level is reported once, not indexed", func(c *content.Content) {})
	c := valid()
	c.Planes[0].MinAirportLevel = 9
	if err := content.Validate(c, refs()); err == nil || strings.Contains(err.Error(), "does not fit") {
		t.Errorf("level 9: %v", err)
	}
}

func TestAirportLevels(t *testing.T) {
	expect(t, "count", "4 levels, want exactly 5", func(c *content.Content) { c.AirportLevels = c.AirportLevels[:4] })
	expect(t, "numbering", "airportLevels[2].level = 4, want 3..3", func(c *content.Content) { c.AirportLevels[2].Level = 4 })
	expect(t, "first level costs nothing", "airportLevels[0].upgradeCostCents = 5", func(c *content.Content) { c.AirportLevels[0].UpgradeCostCents = 5 })
	expect(t, "later levels cost something", "airportLevels[1].upgradeCostCents = 0", func(c *content.Content) { c.AirportLevels[1].UpgradeCostCents = 0 })
	expect(t, "capacity must grow", "airportLevels[1].paxPerDay (vs previous) = 1000", func(c *content.Content) { c.AirportLevels[1].PaxPerDay = 1000 })
	accept(t, "capacity grows by one", func(c *content.Content) { c.AirportLevels[1].PaxPerDay = 1001 })
	expect(t, "slots must not shrink", "routeSlots (vs previous) = 1", func(c *content.Content) { c.AirportLevels[1].RouteSlots = 1 })
	accept(t, "slots may stay equal", func(c *content.Content) { c.AirportLevels[1].RouteSlots = 2 })
	expect(t, "tier must not shrink", "maxPlaneTier (vs previous) = 1", func(c *content.Content) {
		c.AirportLevels[2].MaxPlaneTier = 1
		c.Planes[1].MinAirportLevel = 2
	})
	expect(t, "max tier range", "maxPlaneTier = 6", func(c *content.Content) { c.AirportLevels[4].MaxPlaneTier = 6 })
	expect(t, "pax range", "paxPerDay = 99", func(c *content.Content) { c.AirportLevels[0].PaxPerDay = 99 })
	expect(t, "slots range", "routeSlots = 0", func(c *content.Content) { c.AirportLevels[0].RouteSlots = 0 })
	expect(t, "leased cap", "leasedDeparturesCap = 0", func(c *content.Content) { c.AirportLevels[0].LeasedDeparturesCap = 0 })
	expect(t, "upkeep", "upkeepDayCents = -1", func(c *content.Content) { c.AirportLevels[0].UpkeepDayCents = -1 })
	expect(t, "fee per pax", "feePerPaxCents = -1", func(c *content.Content) { c.AirportLevels[0].FeePerPaxCents = -1 })
	expect(t, "landing fee", "feePerLandingCents = -1", func(c *content.Content) { c.AirportLevels[0].FeePerLandingCents = -1 })
}

func TestBalance(t *testing.T) {
	cases := []struct {
		name, want string
		change     func(*content.Balance)
	}{
		{"d0", "balance.demand.d0", func(b *content.Balance) { b.Demand.D0 = 0 }},
		{"biz max below min", "businessShareMax", func(b *content.Balance) { b.Demand.BusinessShareMax = 0.01 }},
		{"biz min", "businessShareMin", func(b *content.Balance) { b.Demand.BusinessShareMin = -0.1 }},
		{"connect factor", "connectFactor", func(b *content.Balance) { b.Demand.ConnectFactor = 1.1 }},
		{"connecting discount", "connectingDiscount", func(b *content.Balance) { b.Demand.ConnectingDiscount = 0.4 }},
		{"medium share", "mediumAirportShare", func(b *content.Balance) { b.Demand.MediumAirportShare = 0 }},
		{"ref base", "refBaseCents", func(b *content.Balance) { b.Pricing.RefBaseCents = -1 }},
		{"ref per km", "refPerKmCents", func(b *content.Balance) { b.Pricing.RefPerKmCents = 0 }},
		{"biz multiplier", "bizMultiplier", func(b *content.Balance) { b.Pricing.BizMultiplier = 0.9 }},
		{"elasticity eco", "elasticityEco", func(b *content.Balance) { b.Pricing.ElasticityEco = 6 }},
		{"elasticity biz", "elasticityBiz", func(b *content.Balance) { b.Pricing.ElasticityBiz = 0 }},
		{"price factor min", "priceFactorMin", func(b *content.Balance) { b.Pricing.PriceFactorMin = 1.5 }},
		{"price factor max", "priceFactorMax", func(b *content.Balance) { b.Pricing.PriceFactorMax = 0.5 }},
		{"service base", "service.base", func(b *content.Balance) { b.Service.Base = 1.1 }},
		{"service condition", "conditionWeight", func(b *content.Balance) { b.Service.ConditionWeight = -1 }},
		{"service plane level", "planeLevelWeight", func(b *content.Balance) { b.Service.PlaneLevelWeight = 2 }},
		{"service airport level", "airportLevelWeight", func(b *content.Balance) { b.Service.AirportLevelWeight = 2 }},
		{"service business", "service.businessMultiplier", func(b *content.Balance) { b.Service.BusinessMultiplier = 0.5 }},
		{"reputation start", "reputation.start", func(b *content.Balance) { b.Reputation.Start = 101 }},
		{"delay penalty", "delayPenalty", func(b *content.Balance) { b.Reputation.DelayPenalty = -1 }},
		{"cancel below delay", "cancelPenalty", func(b *content.Balance) { b.Reputation.CancelPenalty = 0.1 }},
		{"on-time target", "onTimeTarget", func(b *content.Balance) { b.Reputation.OnTimeTarget = 0.4 }},
		{"gain per point", "gainPerPoint", func(b *content.Balance) { b.Reputation.GainPerPoint = 2 }},
		{"decay", "decayPerDay", func(b *content.Balance) { b.Reputation.DecayPerDay = 0.2 }},
		{"turnaround", "turnaroundTicks", func(b *content.Balance) { b.Flights.TurnaroundTicks = 25 }},
		{"delay share", "delayShareOfFailures", func(b *content.Balance) { b.Flights.DelayShareOfFailures = 1.1 }},
		{"delay revenue", "delayRevenueFactor", func(b *content.Balance) { b.Flights.DelayRevenueFactor = -0.1 }},
		{"condition reliability", "conditionReliability", func(b *content.Balance) { b.Flights.ConditionReliability = 2 }},
		{"rebook", "cancelRebookShare", func(b *content.Balance) { b.Flights.CancelRebookShare = 2 }},
		{"decay per hour", "conditionDecayPerHour", func(b *content.Balance) { b.Aging.ConditionDecayPerHour = 0.1 }},
		{"overhaul cost", "overhaulCostShare", func(b *content.Balance) { b.Aging.OverhaulCostShare = 2 }},
		{"overhaul days", "overhaulDays", func(b *content.Balance) { b.Aging.OverhaulDays = 91 }},
		{"depreciation", "dailyDepreciation", func(b *content.Balance) { b.Aging.DailyDepreciation = 0.9 }},
		{"reconfigure", "reconfigureTicks", func(b *content.Balance) { b.Aging.ReconfigureTicks = -1 }},
		{"fuel mean", "meanCentsPerUnit", func(b *content.Balance) { b.Fuel.MeanCentsPerUnit = 0 }},
		{"theta", "theta", func(b *content.Balance) { b.Fuel.Theta = 2 }},
		{"sigma", "sigma", func(b *content.Balance) { b.Fuel.Sigma = 1 }},
		{"shock chance", "shockChance", func(b *content.Balance) { b.Fuel.ShockChance = 0.5 }},
		{"shock min", "shockMin", func(b *content.Balance) { b.Fuel.ShockMin = -1 }},
		{"shock max below min", "shockMax", func(b *content.Balance) { b.Fuel.ShockMax = 0.1 }},
		{"shock days", "shockDays", func(b *content.Balance) { b.Fuel.ShockDays = 0 }},
		{"fuel min factor", "minFactor", func(b *content.Balance) { b.Fuel.MinFactor = 2 }},
		{"fuel max factor", "maxFactor", func(b *content.Balance) { b.Fuel.MaxFactor = 0.5 }},
		{"price k", "priceK", func(b *content.Balance) { b.Countries.PriceK = 0 }},
		{"upkeep share", "upkeepShare", func(b *content.Balance) { b.Countries.UpkeepShare = 0.1 }},
		{"start cash outside min/max", "startCashCents", func(b *content.Balance) { b.Countries.StartCashCents = 1 }},
		{"start cash min", "startCashMinCents", func(b *content.Balance) { b.Countries.StartCashMinCents = 0 }},
		{"start cash max below min", "startCashMaxCents", func(b *content.Balance) {
			b.Countries.StartCashMaxCents = 5_000_000_00
			b.Countries.StartCashCents = 5_000_000_00
		}},
		{"start exponent", "startCashExponent", func(b *content.Balance) { b.Countries.StartCashExponent = 2 }},
		{"owner share", "ownerFeeShare", func(b *content.Balance) { b.Countries.OwnerFeeShare = 2 }},
		{"active days", "activeServiceDays", func(b *content.Balance) { b.Countries.ActiveServiceDays = 0 }},
		{"lease min", "upfrontShareMin", func(b *content.Balance) { b.Leases.UpfrontShareMin = -1 }},
		{"lease max below min", "upfrontShareMax", func(b *content.Balance) { b.Leases.UpfrontShareMax = 0.01 }},
		{"yearly lease", "yearlyLeaseShare", func(b *content.Balance) { b.Leases.YearlyLeaseShare = 2 }},
		{"leased fee", "leasedFeeMultiplier", func(b *content.Balance) { b.Leases.LeasedFeeMultiplier = 0.5 }},
		{"base rate", "baseRate", func(b *content.Balance) { b.Finance.BaseRate = 0.6 }},
		{"overdraft below base", "overdraftRate", func(b *content.Balance) { b.Finance.OverdraftRate = 0.01 }},
		{"liquidation", "liquidationDays", func(b *content.Balance) { b.Finance.LiquidationDays = 0 }},
		{"restructurings", "restructurings", func(b *content.Balance) { b.Finance.Restructurings = 11 }},
		{"loan weeks", "maxLoanWeeks", func(b *content.Balance) { b.Finance.MaxLoanWeeks = 1 }},
		{"undercut adjust", "priceAdjust", func(b *content.Balance) { b.Competition.PriceAdjust = 2 }},
		{"undercut share", "undercutShare", func(b *content.Balance) { b.Competition.UndercutShare = -1 }},
		{"undercut days", "undercutDays", func(b *content.Balance) { b.Competition.UndercutDays = 0 }},
		{"undercut min", "undercutMin", func(b *content.Balance) { b.Competition.UndercutMin = -0.1 }},
		{"undercut max below min", "undercutMax", func(b *content.Balance) { b.Competition.UndercutMax = 0.01 }},
	}
	for _, tc := range cases {
		expect(t, tc.name, tc.want, func(c *content.Content) { tc.change(&c.Balance) })
	}
}

func TestCreditTiers(t *testing.T) {
	expect(t, "none", "at least one tier", func(c *content.Content) { c.Balance.Finance.CreditTiers = nil })
	expect(t, "not descending", "minScore (descending)", func(c *content.Content) { c.Balance.Finance.CreditTiers[1].MinScore = 800 })
	expect(t, "spread falls as score falls", "spread (not below the better tier)", func(c *content.Content) { c.Balance.Finance.CreditTiers[1].Spread = 0.005 })
	expect(t, "last tier not at zero", "last tier must start at score 0, got 100", func(c *content.Content) { c.Balance.Finance.CreditTiers[2].MinScore = 100 })
	expect(t, "score range", "minScore = 1001", func(c *content.Content) { c.Balance.Finance.CreditTiers[0].MinScore = 1001 })
	expect(t, "spread range", "spread = 2", func(c *content.Content) { c.Balance.Finance.CreditTiers[2].Spread = 2 })
	accept(t, "single tier at zero", func(c *content.Content) {
		c.Balance.Finance.CreditTiers = []content.CreditTier{{MinScore: 0, Spread: 0.05}}
	})
	accept(t, "adjacent scores", func(c *content.Content) { c.Balance.Finance.CreditTiers[1].MinScore = 799 })
}

func TestSeasons(t *testing.T) {
	expect(t, "missing profile", "missing profile for cold/south", func(c *content.Content) { c.Seasons = c.Seasons[:7] })
	expect(t, "duplicate", "duplicate climate/hemisphere", func(c *content.Content) { c.Seasons[7] = season("cold", "north") })
	expect(t, "unknown climate", "want exactly 8", func(c *content.Content) { c.Seasons = append(c.Seasons, season("arctic", "north")) })
	expect(t, "tourism range", "tourism[11] = 2.6", func(c *content.Content) { c.Seasons[0].Tourism[11] = 2.6 })
	expect(t, "business range", "business[0] = 0.2", func(c *content.Content) { c.Seasons[0].Business[0] = 0.2 })
}

func TestHolidays(t *testing.T) {
	expect(t, "bad id", "must be snake_case", func(c *content.Content) { c.Holidays[0].ID = "X mas" })
	expect(t, "duplicate", `duplicate id "h"`, func(c *content.Content) { c.Holidays = append(c.Holidays, c.Holidays[0]) })
	expect(t, "missing key", "i18n key", func(c *content.Content) { c.Holidays[0].NameKey = "holidays.nope.name" })
	expect(t, "days", "days = 61", func(c *content.Content) { c.Holidays[0].Days = 61 })
	expect(t, "tourism", "tourism = 3.1", func(c *content.Content) { c.Holidays[0].Tourism = 3.1 })
	expect(t, "business", "business = 0.1", func(c *content.Content) { c.Holidays[0].Business = 0.1 })
	expect(t, "no target", "needs countries or regions", func(c *content.Content) { c.Holidays[0].Countries, c.Holidays[0].Regions = nil, nil })
	expect(t, "unknown country", `unknown country "XX"`, func(c *content.Content) { c.Holidays[0].Countries = []string{"XX"} })
	expect(t, "unknown region", `unknown region "Atlantis"`, func(c *content.Content) { c.Holidays[0].Regions = []string{"Atlantis"} })
	accept(t, "countries only", func(c *content.Content) { c.Holidays[0].Regions = nil })
	accept(t, "regions only", func(c *content.Content) { c.Holidays[0].Countries = nil })
}

func TestDates(t *testing.T) {
	for _, ok := range []string{"01-01", "02-28", "12-31", "04-30"} {
		accept(t, "date "+ok, func(c *content.Content) { c.Holidays[0].Start = ok })
	}
	bad := map[string]string{
		"02-29": "start day = 29, want 1..28", // no leap years on the game calendar
		"04-31": "start day = 31, want 1..30",
		"13-01": "want MM-DD",
		"00-10": "want MM-DD",
		"1-1":   "want MM-DD",
		"12/31": "want MM-DD",
		"ab-cd": "want MM-DD",
		"12-xx": "want MM-DD",
		"12-00": "start day = 0",
	}
	for in, want := range bad {
		expect(t, "date "+in, want, func(c *content.Content) { c.Holidays[0].Start = in })
	}
}

func TestDecode(t *testing.T) {
	c, err := content.Decode([]byte(`{"planes":[{"id":"a","tier":1}],"balance":{"demand":{"d0":0.04}}}`))
	if err != nil || len(c.Planes) != 1 || c.Planes[0].ID != "a" || c.Balance.Demand.D0 != 0.04 {
		t.Fatalf("Decode = %+v, %v", c, err)
	}
	if _, err := content.Decode([]byte(`{"planes":[{"id":"a","seats":5}]}`)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("unknown field: %v", err)
	}
	if _, err := content.Decode([]byte(`nope`)); err == nil {
		t.Error("invalid JSON accepted")
	}
}
