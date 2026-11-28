package content

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

// Refs are the external names content may reference.
type Refs struct {
	Countries map[string]bool // ISO2 codes from content/world/countries.json
	Regions   map[string]bool // UN regions from content/world/countries.json
	I18nKeys  map[string]bool // keys of content/i18n/en.json
}

// Decode parses compiled content JSON. Unknown fields are errors, so a typo in
// content can't silently fall back to a zero value.
func Decode(data []byte) (Content, error) {
	var c Content
	if err := DecodeInto(data, &c); err != nil {
		return Content{}, fmt.Errorf("content: %w", err)
	}
	return c, nil
}

// DecodeInto strictly decodes JSON into v (unknown fields are errors).
func DecodeInto(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

var idRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Validate checks every rule and returns all problems found (nil when valid).
func Validate(c Content, refs Refs) error {
	v := &checker{refs: refs}
	v.planes(c.Planes, c.AirportLevels)
	v.airportLevels(c.AirportLevels)
	v.balance(c.Balance)
	v.seasons(c.Seasons)
	v.holidays(c.Holidays)
	return errors.Join(v.errs...)
}

type checker struct {
	refs Refs
	errs []error
}

func (v *checker) fail(format string, args ...any) {
	v.errs = append(v.errs, fmt.Errorf(format, args...))
}

// num checks lo <= x <= hi.
func (v *checker) num(path string, x, lo, hi float64) {
	if !(x >= lo && x <= hi) { // also rejects NaN
		v.fail("%s = %v, want %v..%v", path, x, lo, hi)
	}
}

func (v *checker) integer(path string, x, lo, hi int64) {
	if x < lo || x > hi {
		v.fail("%s = %d, want %d..%d", path, x, lo, hi)
	}
}

func (v *checker) key(path, key string) {
	if !v.refs.I18nKeys[key] {
		v.fail("%s: i18n key %q not found in content/i18n/en.json", path, key)
	}
}

func (v *checker) id(path, id string, seen map[string]bool) {
	if !idRe.MatchString(id) {
		v.fail("%s: id %q must be snake_case", path, id)
	}
	if seen[id] {
		v.fail("%s: duplicate id %q", path, id)
	}
	seen[id] = true
}

func (v *checker) planes(planes []PlaneModel, levels []AirportLevel) {
	if len(planes) == 0 {
		v.fail("planes: at least one plane model is required")
	}
	seen := map[string]bool{}
	for i, p := range planes {
		path := fmt.Sprintf("planes[%d] %s", i, p.ID)
		v.id(path, p.ID, seen)
		v.key(path, p.NameKey)
		v.integer(path+".tier", int64(p.Tier), 1, 5)
		v.integer(path+".seatUnits", int64(p.SeatUnits), 10, 1000)
		v.integer(path+".rangeKm", int64(p.RangeKm), 300, 20000)
		v.integer(path+".speedKmh", int64(p.SpeedKmh), 250, 1100)
		v.integer(path+".minRunwayFt", int64(p.MinRunwayFt), 0, 14000)
		v.integer(path+".minAirportLevel", int64(p.MinAirportLevel), 1, 5)
		v.integer(path+".priceCents", p.PriceCents, 100_000_00, 1_000_000_000_00)
		v.integer(path+".upkeepDayCents", p.UpkeepDayCents, 0, 10_000_000_00)
		v.num(path+".burnPerKm", p.BurnPerKm, 0.1, 30)
		v.num(path+".reliability", p.Reliability, 0.5, 1)
		v.integer(path+".maxAgeDays", int64(p.MaxAgeDays), 1825, 20000)
		total := p.Reliability
		for j, l := range p.Levels {
			lp := fmt.Sprintf("%s.levels[%d]", path, j)
			v.integer(lp+".costCents", l.CostCents, 1, 1_000_000_000_00)
			v.integer(lp+".seatUnits", int64(l.SeatUnits), 0, 100)
			v.num(lp+".efficiency", l.Efficiency, 0, 0.3)
			v.num(lp+".reliability", l.Reliability, 0, 0.05)
			total += l.Reliability
		}
		v.num(path+" reliability at max level", total, 0.5, 1)
		// The plane's minimum airport level must allow its tier.
		if lvl := p.MinAirportLevel; lvl >= 1 && lvl <= len(levels) && levels[lvl-1].MaxPlaneTier < p.Tier {
			v.fail("%s: tier %d does not fit airport level %d (max tier %d)", path, p.Tier, lvl, levels[lvl-1].MaxPlaneTier)
		}
	}
}

func (v *checker) airportLevels(levels []AirportLevel) {
	if len(levels) != 5 {
		v.fail("airportLevels: %d levels, want exactly 5", len(levels))
		return
	}
	for i, l := range levels {
		path := fmt.Sprintf("airportLevels[%d]", i)
		v.integer(path+".level", int64(l.Level), int64(i+1), int64(i+1))
		v.integer(path+".maxPlaneTier", int64(l.MaxPlaneTier), 1, 5)
		v.integer(path+".paxPerDay", int64(l.PaxPerDay), 100, 10_000_000)
		v.integer(path+".routeSlots", int64(l.RouteSlots), 1, 1000)
		v.integer(path+".leasedDeparturesCap", int64(l.LeasedDeparturesCap), 1, 1000)
		v.integer(path+".upkeepDayCents", l.UpkeepDayCents, 0, 100_000_000_00)
		v.integer(path+".feePerPaxCents", l.FeePerPaxCents, 0, 1_000_00)
		v.integer(path+".feePerLandingCents", l.FeePerLandingCents, 0, 100_000_00)
		if i == 0 {
			v.integer(path+".upgradeCostCents", l.UpgradeCostCents, 0, 0)
			continue
		}
		v.integer(path+".upgradeCostCents", l.UpgradeCostCents, 1, 100_000_000_000_00)
		prev := levels[i-1]
		// Higher levels never offer less.
		v.integer(path+".paxPerDay (vs previous)", int64(l.PaxPerDay), int64(prev.PaxPerDay)+1, 10_000_000)
		v.integer(path+".routeSlots (vs previous)", int64(l.RouteSlots), int64(prev.RouteSlots), 1000)
		v.integer(path+".maxPlaneTier (vs previous)", int64(l.MaxPlaneTier), int64(prev.MaxPlaneTier), 5)
	}
}

func (v *checker) balance(b Balance) {
	d := b.Demand
	v.num("balance.demand.d0", d.D0, 0.001, 1e6)
	v.num("balance.demand.businessShareMin", d.BusinessShareMin, 0, 0.5)
	v.num("balance.demand.businessShareMax", d.BusinessShareMax, d.BusinessShareMin, 0.5)
	v.num("balance.demand.connectFactor", d.ConnectFactor, 0, 1)
	v.num("balance.demand.connectingDiscount", d.ConnectingDiscount, 0.5, 1)
	v.num("balance.demand.mediumAirportShare", d.MediumAirportShare, 0.1, 1)

	p := b.Pricing
	v.integer("balance.pricing.refBaseCents", p.RefBaseCents, 0, 10_000_00)
	v.num("balance.pricing.refPerKmCents", p.RefPerKmCents, 0.1, 100)
	v.num("balance.pricing.bizMultiplier", p.BizMultiplier, 1, 10)
	v.num("balance.pricing.elasticityEco", p.ElasticityEco, 0.1, 5)
	v.num("balance.pricing.elasticityBiz", p.ElasticityBiz, 0.1, 5)
	v.num("balance.pricing.priceFactorMin", p.PriceFactorMin, 0, 1)
	v.num("balance.pricing.priceFactorMax", p.PriceFactorMax, 1, 10)

	s := b.Service
	v.num("balance.service.base", s.Base, 0, 1)
	v.num("balance.service.conditionWeight", s.ConditionWeight, 0, 1)
	v.num("balance.service.planeLevelWeight", s.PlaneLevelWeight, 0, 1)
	v.num("balance.service.airportLevelWeight", s.AirportLevelWeight, 0, 1)
	v.num("balance.service.businessMultiplier", s.BusinessMultiplier, 1, 5)

	r := b.Reputation
	v.num("balance.reputation.start", r.Start, 0, 100)
	v.num("balance.reputation.delayPenalty", r.DelayPenalty, 0, 10)
	v.num("balance.reputation.cancelPenalty", r.CancelPenalty, r.DelayPenalty, 20)
	v.num("balance.reputation.onTimeTarget", r.OnTimeTarget, 0.5, 1)
	v.num("balance.reputation.gainPerPoint", r.GainPerPoint, 0, 1)
	v.num("balance.reputation.decayPerDay", r.DecayPerDay, 0, 0.1)

	f := b.Flights
	v.integer("balance.flights.turnaroundTicks", int64(f.TurnaroundTicks), 0, 24)
	v.num("balance.flights.delayShareOfFailures", f.DelayShareOfFailures, 0, 1)
	v.num("balance.flights.delayRevenueFactor", f.DelayRevenueFactor, 0, 1)
	v.num("balance.flights.conditionReliability", f.ConditionReliability, 0, 1)
	v.num("balance.flights.cancelRebookShare", f.CancelRebookShare, 0, 1)

	a := b.Aging
	v.num("balance.aging.conditionDecayPerHour", a.ConditionDecayPerHour, 0, 0.01)
	v.num("balance.aging.overhaulCostShare", a.OverhaulCostShare, 0, 1)
	v.integer("balance.aging.overhaulDays", int64(a.OverhaulDays), 0, 90)
	v.num("balance.aging.dailyDepreciation", a.DailyDepreciation, 0.99, 1)
	v.integer("balance.aging.reconfigureTicks", int64(a.ReconfigureTicks), 0, 24*30)

	fu := b.Fuel
	v.integer("balance.fuel.meanCentsPerUnit", fu.MeanCentsPerUnit, 1, 10_000)
	v.num("balance.fuel.theta", fu.Theta, 0, 1)
	v.num("balance.fuel.sigma", fu.Sigma, 0, 0.5)
	v.num("balance.fuel.shockChance", fu.ShockChance, 0, 0.2)
	v.num("balance.fuel.shockMin", fu.ShockMin, 0, 2)
	v.num("balance.fuel.shockMax", fu.ShockMax, fu.ShockMin, 2)
	v.integer("balance.fuel.shockDays", int64(fu.ShockDays), 1, 365)
	v.num("balance.fuel.minFactor", fu.MinFactor, 0.05, 1)
	v.num("balance.fuel.maxFactor", fu.MaxFactor, 1, 10)

	c := b.Countries
	v.num("balance.countries.priceK", c.PriceK, 1e-9, 1e9)
	v.num("balance.countries.upkeepShare", c.UpkeepShare, 0, 0.01)
	v.integer("balance.countries.startCashMinCents", c.StartCashMinCents, 1, 1e14)
	v.integer("balance.countries.startCashCents", c.StartCashCents, c.StartCashMinCents, c.StartCashMaxCents)
	v.integer("balance.countries.startCashMaxCents", c.StartCashMaxCents, c.StartCashMinCents, 1e14)
	v.num("balance.countries.startCashExponent", c.StartCashExponent, 0, 1)
	v.num("balance.countries.ownerFeeShare", c.OwnerFeeShare, 0, 1)
	v.integer("balance.countries.activeServiceDays", int64(c.ActiveServiceDays), 1, 365)

	l := b.Leases
	v.num("balance.leases.upfrontShareMin", l.UpfrontShareMin, 0, 1)
	v.num("balance.leases.upfrontShareMax", l.UpfrontShareMax, l.UpfrontShareMin, 1)
	v.num("balance.leases.yearlyLeaseShare", l.YearlyLeaseShare, 0, 1)
	v.num("balance.leases.leasedFeeMultiplier", l.LeasedFeeMultiplier, 1, 5)

	fi := b.Finance
	v.num("balance.finance.baseRate", fi.BaseRate, 0, 0.5)
	v.num("balance.finance.overdraftRate", fi.OverdraftRate, fi.BaseRate, 2)
	v.integer("balance.finance.liquidationDays", int64(fi.LiquidationDays), 1, 365)
	v.integer("balance.finance.restructurings", int64(fi.Restructurings), 0, 10)
	v.integer("balance.finance.maxLoanWeeks", int64(fi.MaxLoanWeeks), 4, 52*30)
	v.creditTiers(fi.CreditTiers)

	co := b.Competition
	v.num("balance.competition.priceAdjust", co.PriceAdjust, 0, 1)
	v.num("balance.competition.undercutShare", co.UndercutShare, 0, 1)
	v.integer("balance.competition.undercutDays", int64(co.UndercutDays), 1, 365)
	v.num("balance.competition.undercutMin", co.UndercutMin, 0, 0.5)
	v.num("balance.competition.undercutMax", co.UndercutMax, co.UndercutMin, 0.5)
}

// creditTiers must cover scores 0..1000: descending MinScore ending at 0, with
// spreads that grow as the score falls.
func (v *checker) creditTiers(tiers []CreditTier) {
	if len(tiers) == 0 {
		v.fail("balance.finance.creditTiers: at least one tier is required")
		return
	}
	for i, t := range tiers {
		path := fmt.Sprintf("balance.finance.creditTiers[%d]", i)
		v.integer(path+".minScore", int64(t.MinScore), 0, 1000)
		v.num(path+".spread", t.Spread, 0, 1)
		if i > 0 {
			prev := tiers[i-1]
			v.integer(path+".minScore (descending)", int64(t.MinScore), 0, int64(prev.MinScore)-1)
			v.num(path+".spread (not below the better tier)", t.Spread, prev.Spread, 1)
		}
	}
	if last := tiers[len(tiers)-1]; last.MinScore != 0 {
		v.fail("balance.finance.creditTiers: the last tier must start at score 0, got %d", last.MinScore)
	}
}

var (
	climates    = []string{"tropical", "subtropical", "temperate", "cold"}
	hemispheres = []string{"north", "south"}
)

// seasons must define each climate and hemisphere combination exactly once.
func (v *checker) seasons(seasons []Season) {
	seen := map[string]bool{}
	for i, s := range seasons {
		path := fmt.Sprintf("seasons[%d] %s/%s", i, s.Climate, s.Hemisphere)
		k := s.Climate + "/" + s.Hemisphere
		if seen[k] {
			v.fail("%s: duplicate climate/hemisphere", path)
		}
		seen[k] = true
		for m := 0; m < 12; m++ {
			v.num(fmt.Sprintf("%s.tourism[%d]", path, m), s.Tourism[m], 0.3, 2.5)
			v.num(fmt.Sprintf("%s.business[%d]", path, m), s.Business[m], 0.3, 2.5)
		}
	}
	for _, c := range climates {
		for _, h := range hemispheres {
			if !seen[c+"/"+h] {
				v.fail("seasons: missing profile for %s/%s", c, h)
			}
		}
	}
	if want := len(climates) * len(hemispheres); len(seen) != want || len(seasons) != want {
		v.fail("seasons: %d profiles, want exactly %d (unknown climate or hemisphere?)", len(seasons), want)
	}
}

// daysInMonth for the game calendar (no leap years).
var daysInMonth = [12]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

func (v *checker) holidays(hs []Holiday) {
	seen := map[string]bool{}
	for i, h := range hs {
		path := fmt.Sprintf("holidays[%d] %s", i, h.ID)
		v.id(path, h.ID, seen)
		v.key(path, h.NameKey)
		v.date(path+".start", h.Start)
		v.integer(path+".days", int64(h.Days), 1, 60)
		v.num(path+".tourism", h.Tourism, 0.2, 3)
		v.num(path+".business", h.Business, 0.2, 3)
		if len(h.Countries)+len(h.Regions) == 0 {
			v.fail("%s: needs countries or regions", path)
		}
		for _, c := range h.Countries {
			if !v.refs.Countries[c] {
				v.fail("%s: unknown country %q", path, c)
			}
		}
		for _, r := range h.Regions {
			if !v.refs.Regions[r] {
				v.fail("%s: unknown region %q", path, r)
			}
		}
	}
}

// date checks a game-calendar date "MM-DD".
func (v *checker) date(path, s string) {
	if len(s) != 5 || s[2] != '-' {
		v.fail("%s = %q, want MM-DD", path, s)
		return
	}
	m, errM := strconv.Atoi(s[:2])
	d, errD := strconv.Atoi(s[3:])
	if errM != nil || errD != nil || m < 1 || m > 12 {
		v.fail("%s = %q, want MM-DD", path, s)
		return
	}
	v.integer(path+" day", int64(d), 1, int64(daysInMonth[m-1]))
}
