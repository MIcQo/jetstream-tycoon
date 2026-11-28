package sim_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim"
	"github.com/MIcQo/jetstream-tycoon/sim/content"
	"github.com/MIcQo/jetstream-tycoon/sim/content/bundle"
	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

// testBundle is a tiny world: country BIG (expensive) with two airports, SML
// (cheap) with one, and NON with no airport.
func testBundle() *bundle.Bundle {
	b := &bundle.Bundle{ContentVersion: "ctest"}
	b.Content.Balance.Countries = content.CountryBalance{
		PriceK: 1, StartCashCents: 1_000_00, StartCashMinCents: 500_00, StartCashMaxCents: 5_000_00, StartCashExponent: 0.5,
	}
	b.Content.Balance.Reputation.Start = 50
	b.Content.Balance.Finance.Restructurings = 1
	b.Content.Balance.Fuel.MeanCentsPerUnit = 80
	b.Content.Planes = []content.PlaneModel{
		{ID: "big_jet", Tier: 2, SeatUnits: 150, PriceCents: 50},
		{ID: "prop_b", Tier: 1, SeatUnits: 40, PriceCents: 20},
		{ID: "prop_a", Tier: 1, SeatUnits: 50, PriceCents: 20},
		{ID: "prop_dear", Tier: 1, SeatUnits: 60, PriceCents: 30},
	}
	b.World.Countries = []content.Country{
		{ISO2: "BIG", Stats: content.CountryStats{PriceBase: 40_000_00}},
		{ISO2: "SML", Stats: content.CountryStats{PriceBase: 10_000_00}},
		{ISO2: "NON", Stats: content.CountryStats{PriceBase: 1}},
		{ISO2: "ZRO", Stats: content.CountryStats{PriceBase: 0}},
	}
	b.World.Cities = []content.City{{ID: "big_city", Pop: 5_000_000}, {ID: "small_city", Pop: 100_000}, {ID: "sml_city", Pop: 200_000}}
	b.World.Airports = []content.Airport{
		{IATA: "BBB", ISO2: "BIG", CityID: "small_city", RunwayFt: 12_000},
		{IATA: "BAA", ISO2: "BIG", CityID: "big_city", RunwayFt: 9_000},
		{IATA: "SSS", ISO2: "SML", CityID: "sml_city", RunwayFt: 8_000},
		{IATA: "ZZZ", ISO2: "ZRO", CityID: "sml_city", RunwayFt: 8_000},
	}
	return b
}

func params(home string) sim.NewGameParams {
	return sim.NewGameParams{Seed: 7, AirlineName: "  Blue Skies  ", HomeCountry: home, Difficulty: "hard"}
}

func TestNewGameSetup(t *testing.T) {
	e, err := sim.NewGame(params("BIG"), testBundle())
	if err != nil {
		t.Fatal(err)
	}
	s := e.State
	p := s.Player()
	if p == nil || p.Name != "Blue Skies" || p.Home != "BIG" || p.Controller != "human" || p.Color != sim.PlayerColor ||
		p.Reputation != 50 || p.CreditScore != 500 || p.Status != "ok" || p.RestructuringsLeft != 1 {
		t.Fatalf("player = %+v", p)
	}
	// The hub serves the biggest city, even though BBB has the longer runway.
	if len(p.Hubs) != 1 || p.Hubs[0] != "BAA" {
		t.Errorf("hubs = %v, want [BAA]", p.Hubs)
	}
	if c := s.Countries["BIG"]; c == nil || c.OwnerID != state.PlayerID || len(s.Countries) != 1 {
		t.Errorf("countries = %v", s.Countries)
	}
	// Starter plane: lowest tier, cheapest, then lowest ID → prop_a.
	if len(s.Planes) != 1 {
		t.Fatalf("planes = %v", s.Planes)
	}
	pl := s.Planes["pl1"]
	if pl == nil || pl.ModelID != "prop_a" || pl.AirlineID != state.PlayerID || pl.Level != 1 || pl.Condition != 1 ||
		pl.DepFactor != 1 || pl.SeatsEco != 50 || pl.SeatsBiz != 0 || pl.State != state.PlaneIdle || s.NextID != 1 {
		t.Errorf("plane = %+v", pl)
	}
	if s.Seed != 7 || s.Difficulty != "hard" || s.Fuel.PriceCents != 80 || len(s.Fuel.History) != 1 || s.Fuel.History[0] != 80 {
		t.Errorf("world = seed %d, difficulty %s, fuel %+v", s.Seed, s.Difficulty, s.Fuel)
	}
	if e.Data == nil || e.Data.ContentVersion != "ctest" {
		t.Error("engine does not carry the content")
	}
	// BIG is the most expensive country: ratio 1 → base start cash.
	if p.CashCents != 1_000_00 {
		t.Errorf("BIG start cash = %d, want 100000", p.CashCents)
	}
}

func TestStartCash(t *testing.T) {
	b := testBundle()
	cash := func(home string) int64 {
		e, err := sim.NewGame(params(home), b)
		if err != nil {
			t.Fatal(err)
		}
		return e.State.Player().CashCents
	}
	// SML costs a quarter of BIG: 1000 · 4^0.5 = 2000 dollars.
	if got := cash("SML"); got != 2_000_00 {
		t.Errorf("SML start cash = %d, want 200000", got)
	}
	// Clamped to the maximum: exponent 2 → 1000 · 16 = 16000 > 5000.
	b.Content.Balance.Countries.StartCashExponent = 2
	if got := cash("SML"); got != 5_000_00 {
		t.Errorf("clamped to max = %d", got)
	}
	// Clamped to the minimum.
	b.Content.Balance.Countries.StartCashExponent = 0.5
	b.Content.Balance.Countries.StartCashCents = 100_00
	if got := cash("BIG"); got != 500_00 {
		t.Errorf("clamped to min = %d", got)
	}
	// A home country without a price gets the maximum.
	if got := cash("ZRO"); got != 5_000_00 {
		t.Errorf("priceless home = %d", got)
	}
}

func TestCountryPrice(t *testing.T) {
	c := content.Country{Stats: content.CountryStats{PriceBase: 1234.4}}
	if got := sim.CountryPriceCents(c, content.CountryBalance{PriceK: 10}); got != 12344 {
		t.Errorf("price = %d", got)
	}
}

func TestMainAirportTieBreaks(t *testing.T) {
	b := testBundle()
	// Same city population: the longer runway wins.
	b.World.Airports[0].CityID = "big_city"
	if e, _ := sim.NewGame(params("BIG"), b); e.State.Player().Hubs[0] != "BBB" {
		t.Errorf("runway tie-break: hub %v", e.State.Player().Hubs)
	}
	// Same population and runway: the lowest IATA code wins.
	b.World.Airports[0].RunwayFt = 9_000
	if e, _ := sim.NewGame(params("BIG"), b); e.State.Player().Hubs[0] != "BAA" {
		t.Errorf("IATA tie-break: hub %v", e.State.Player().Hubs)
	}
}

func TestStarterModelTieBreaks(t *testing.T) {
	b := testBundle()
	b.Content.Planes = []content.PlaneModel{{ID: "z", Tier: 1, PriceCents: 5}, {ID: "y", Tier: 1, PriceCents: 9}, {ID: "x", Tier: 3, PriceCents: 1}}
	if e, _ := sim.NewGame(params("BIG"), b); e.State.Planes["pl1"].ModelID != "z" {
		t.Errorf("cheapest of the lowest tier: %s", e.State.Planes["pl1"].ModelID)
	}
	b.Content.Planes = []content.PlaneModel{{ID: "x", Tier: 2, PriceCents: 1}, {ID: "w", Tier: 1, PriceCents: 99}}
	if e, _ := sim.NewGame(params("BIG"), b); e.State.Planes["pl1"].ModelID != "w" {
		t.Errorf("lowest tier first: %s", e.State.Planes["pl1"].ModelID)
	}
}

func TestNewGameValidation(t *testing.T) {
	cases := []struct {
		name   string
		change func(*sim.NewGameParams)
		want   error
	}{
		{"empty name", func(p *sim.NewGameParams) { p.AirlineName = "" }, sim.ErrAirlineName},
		{"blank name", func(p *sim.NewGameParams) { p.AirlineName = "   " }, sim.ErrAirlineName},
		{"long name", func(p *sim.NewGameParams) { p.AirlineName = strings.Repeat("a", 41) }, sim.ErrAirlineName},
		{"bad difficulty", func(p *sim.NewGameParams) { p.Difficulty = "nightmare" }, sim.ErrDifficulty},
		{"unknown country", func(p *sim.NewGameParams) { p.HomeCountry = "XX" }, sim.ErrHomeCountry},
		{"country without airports", func(p *sim.NewGameParams) { p.HomeCountry = "NON" }, sim.ErrHomeCountry},
	}
	for _, tc := range cases {
		p := params("BIG")
		tc.change(&p)
		if _, err := sim.NewGame(p, testBundle()); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
	// 40 characters is allowed, counted in characters, not bytes.
	p := params("BIG")
	p.AirlineName = strings.Repeat("é", sim.MaxAirlineNameLen)
	if _, err := sim.NewGame(p, testBundle()); err != nil {
		t.Errorf("40 multi-byte characters rejected: %v", err)
	}
	for _, d := range []string{"easy", "normal", "hard"} {
		p := params("BIG")
		p.Difficulty = d
		if _, err := sim.NewGame(p, testBundle()); err != nil {
			t.Errorf("difficulty %s rejected: %v", d, err)
		}
	}
	if sim.ErrHomeCountry.Error() != "errors.newgame.home_country" {
		t.Error("error keys")
	}
}

func TestNewGameWithRealContent(t *testing.T) {
	b := realBundle(t)
	small, err := sim.NewGame(sim.NewGameParams{Seed: 1, AirlineName: "Island Air", HomeCountry: "MT", Difficulty: "normal"}, b)
	if err != nil {
		t.Fatal(err)
	}
	big, err := sim.NewGame(sim.NewGameParams{Seed: 1, AirlineName: "Big Air", HomeCountry: "US", Difficulty: "normal"}, b)
	if err != nil {
		t.Fatal(err)
	}
	if small.State.Player().CashCents <= big.State.Player().CashCents {
		t.Errorf("a small home country should start with more cash: MT %d, US %d", small.State.Player().CashCents, big.State.Player().CashCents)
	}
	if hub := big.State.Player().Hubs[0]; hub != "JFK" {
		t.Errorf("US hub = %s, want JFK (New York, longest runway)", hub)
	}
	if m := big.State.Planes["pl1"].ModelID; m != "skylark_50" {
		t.Errorf("starter plane = %s", m)
	}
}
