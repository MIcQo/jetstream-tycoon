package sim

import (
	"strings"
	"unicode/utf8"

	"github.com/MIcQo/jetstream-tycoon/sim/command"
	"github.com/MIcQo/jetstream-tycoon/sim/content"
	"github.com/MIcQo/jetstream-tycoon/sim/content/bundle"
	"github.com/MIcQo/jetstream-tycoon/sim/detmath"
	"github.com/MIcQo/jetstream-tycoon/sim/engine"
	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

// NewGameParams are the player's choices for a new game.
type NewGameParams struct {
	Seed        uint64 `json:"seed"`
	AirlineName string `json:"airlineName"`
	HomeCountry string `json:"homeCountry"` // ISO2; must have at least one airport
	Difficulty  string `json:"difficulty"`  // easy | normal | hard
}

// MaxAirlineNameLen is the longest airline name, in characters.
const MaxAirlineNameLen = 40

// New-game validation errors (i18n keys, ADR-0016).
var (
	ErrAirlineName = &command.Error{Key: "errors.newgame.airline_name"}
	ErrHomeCountry = &command.Error{Key: "errors.newgame.home_country"}
	ErrDifficulty  = &command.Error{Key: "errors.newgame.difficulty"}
)

// PlayerColor is the player's airline colour on the map.
const PlayerColor = "#2563eb"

// NewGame validates the player's choices and creates the starting world
// (VISION "Game start"): the player owns the home country, its main airport is the
// hub, the airline has one starter plane, and starting cash is larger for smaller
// home countries. AI airlines are added in M8.5.
func NewGame(p NewGameParams, data *bundle.Bundle) (*engine.Engine, error) {
	name := strings.TrimSpace(p.AirlineName)
	if name == "" || utf8.RuneCountInString(name) > MaxAirlineNameLen {
		return nil, ErrAirlineName
	}
	switch p.Difficulty {
	case "easy", "normal", "hard":
	default:
		return nil, ErrDifficulty
	}
	w := &data.World
	home, ok := findCountry(w, p.HomeCountry)
	hub := mainAirport(w, p.HomeCountry)
	if !ok || hub == "" {
		return nil, ErrHomeCountry
	}
	c := &data.Content
	b := c.Balance

	s := state.New(p.Seed)
	s.Difficulty = p.Difficulty
	player := &state.Airline{
		ID:                 state.PlayerID,
		Name:               name,
		Color:              PlayerColor,
		Controller:         "human",
		Home:               home.ISO2,
		CashCents:          StartCash(w, home, b.Countries),
		Reputation:         b.Reputation.Start,
		CreditScore:        500,
		Status:             "ok",
		RestructuringsLeft: b.Finance.Restructurings,
		Hubs:               []string{hub},
	}
	s.Airlines[player.ID] = player
	s.Countries[home.ISO2] = &state.Country{ISO2: home.ISO2, OwnerID: player.ID}

	model := starterModel(c.Planes)
	planeID := s.NewID("pl")
	s.Planes[planeID] = &state.Plane{
		ID: planeID, AirlineID: player.ID, ModelID: model.ID, Level: 1,
		Condition: 1, DepFactor: 1, SeatsEco: model.SeatUnits, State: state.PlaneIdle,
	}
	s.Fuel = state.Fuel{PriceCents: b.Fuel.MeanCentsPerUnit, History: []int64{b.Fuel.MeanCentsPerUnit}}
	return engine.New(s, data, Systems()...), nil
}

func findCountry(w *content.World, iso2 string) (content.Country, bool) {
	for _, c := range w.Countries {
		if c.ISO2 == iso2 {
			return c, true
		}
	}
	return content.Country{}, false
}

// CountryPriceCents is the purchase price of a country: priceK · priceBase (TECH_DESIGN §7.0).
func CountryPriceCents(c content.Country, b content.CountryBalance) int64 {
	return detmath.RoundCents(b.PriceK * c.Stats.PriceBase)
}

// StartCash is startCash · (maxCountryPrice / homePrice)^exponent, clamped to
// [min, max] (TECH_DESIGN §7.0). A home country without a price gets the maximum.
func StartCash(w *content.World, home content.Country, b content.CountryBalance) int64 {
	homePrice := CountryPriceCents(home, b)
	if homePrice <= 0 {
		return b.StartCashMaxCents
	}
	var maxPrice int64
	for _, c := range w.Countries {
		maxPrice = max(maxPrice, CountryPriceCents(c, b))
	}
	ratio := float64(maxPrice) / float64(homePrice)
	cash := detmath.RoundCents(float64(b.StartCashCents) * detmath.Pow(ratio, b.StartCashExponent))
	return min(max(cash, b.StartCashMinCents), b.StartCashMaxCents)
}

// mainAirport is the home country's hub: the airport serving the largest city,
// then the longest runway, then the lowest IATA code. "" if the country has none.
func mainAirport(w *content.World, iso2 string) string {
	pop := map[string]int64{}
	for _, c := range w.Cities {
		pop[c.ID] = c.Pop
	}
	var best content.Airport
	for _, a := range w.Airports {
		if a.ISO2 != iso2 {
			continue
		}
		if best.IATA == "" || better(a, best, pop) {
			best = a
		}
	}
	return best.IATA
}

func better(a, b content.Airport, pop map[string]int64) bool {
	if pa, pb := pop[a.CityID], pop[b.CityID]; pa != pb {
		return pa > pb
	}
	if a.RunwayFt != b.RunwayFt {
		return a.RunwayFt > b.RunwayFt
	}
	return a.IATA < b.IATA
}

// starterModel is the cheapest plane of the lowest tier (ties: lowest ID).
func starterModel(planes []content.PlaneModel) content.PlaneModel {
	best := planes[0]
	for _, p := range planes[1:] {
		if p.Tier < best.Tier || (p.Tier == best.Tier && (p.PriceCents < best.PriceCents || (p.PriceCents == best.PriceCents && p.ID < best.ID))) {
			best = p
		}
	}
	return best
}
