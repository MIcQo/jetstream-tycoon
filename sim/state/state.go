// Package state holds the serializable simulation state (TECH_DESIGN §4).
//
// The player is one airline among several (ADR-0020). Ownership lives on world
// entities (Country.OwnerID, Route.AirlineID, ...). Maps are for lookup only:
// game logic iterates sorted keys (ADR-0006), using SortedKeys.
//
// Money is int64 cents. Entity IDs are allocated deterministically by NewID.
package state

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"slices"
	"strconv"
)

// TicksPerDay is the number of ticks (game hours) in a game day.
const TicksPerDay = 24

// SchemaVersion is the current save schema version. Bump it with a migrator in
// sim/save when the shape of State changes.
const SchemaVersion = 2

// PlayerID is the human player's airline.
const PlayerID AirlineID = "P"

// AirlineID identifies an airline: "P" for the player, "A1".."A6" for AI airlines.
type AirlineID string

// ID identifies a plane, route, loan, offer or other created entity.
type ID string

// State is the complete game state.
type State struct {
	Version    int    `json:"version"`
	Seed       uint64 `json:"seed"`
	Tick       int64  `json:"tick"`
	Speed      int    `json:"speed"`
	Difficulty string `json:"difficulty"` // easy | normal | hard
	NextID     int64  `json:"nextId"`

	Airlines      map[AirlineID]*Airline   `json:"airlines"`
	Countries     map[string]*Country      `json:"countries"`     // ISO2 → ownership (only countries with state)
	Airports      map[string]*Airport      `json:"airports"`      // IATA → runtime state (only changed airports)
	LandingRights map[string]*LandingRight `json:"landingRights"` // LandingRightKey(airline, IATA)
	Planes        map[ID]*Plane            `json:"planes"`
	Routes        map[ID]*Route            `json:"routes"`
	Markets       map[string]*Market       `json:"markets"` // MarketKey(a, b)
	Loans         map[ID]*Loan             `json:"loans"`
	Offers        map[ID]*Offer            `json:"offers"`
	Missions      map[ID]*Mission          `json:"missions"`
	Pending       map[ID]*PendingEvent     `json:"pending"`
	Modifiers     map[string]*Modifier     `json:"modifiers"` // by Modifier.ID

	Fuel           Fuel              `json:"fuel"`
	UsedMarket     UsedMarket        `json:"usedMarket"`
	Milestones     map[string]int64  `json:"milestones"`     // milestone ID → tick achieved
	ConfigSnapshot map[string]string `json:"configSnapshot"` // simulation-affecting remote config (ADR-0006)
}

// Airline is one airline, human or AI.
type Airline struct {
	ID                 AirlineID `json:"id"`
	Name               string    `json:"name"`
	Color              string    `json:"color"`
	Controller         string    `json:"controller"`            // human | ai
	Personality        string    `json:"personality,omitempty"` // AI only, content ID
	Home               string    `json:"home"`                  // ISO2
	CashCents          int64     `json:"cashCents"`
	Reputation         float64   `json:"reputation"` // 0..100
	CreditScore        int       `json:"creditScore"`
	Status             string    `json:"status"` // ok | warning | distress | liquidation | bankrupt
	RestructuringsLeft int       `json:"restructuringsLeft"`
	Hubs               []string  `json:"hubs"` // IATA, sorted
}

// Country is the ownership state of a country. Countries absent from the map are unowned.
type Country struct {
	ISO2          string    `json:"iso2"`
	OwnerID       AirlineID `json:"ownerId"`
	PurchasedTick int64     `json:"purchasedTick"`
}

// Airport is the runtime state of an airport that differs from its content
// defaults (TECH_DESIGN §4: an untouched airport runs at its base level).
type Airport struct {
	IATA  string `json:"iata"`
	Level int    `json:"level"`
}

// LandingRight lets a non-owner fly from an airport.
type LandingRight struct {
	AirlineID        AirlineID `json:"airlineId"`
	IATA             string    `json:"iata"`
	YearlyLeaseCents int64     `json:"yearlyLeaseCents"`
	ExpiresTick      int64     `json:"expiresTick"`
	DeparturesCap    int       `json:"departuresCap"`
}

// Plane states.
const (
	PlaneIdle     = "idle"
	PlaneFlying   = "flying"
	PlaneOverhaul = "overhaul"
	PlaneReconfig = "reconfig"
)

// Plane is one owned aircraft.
type Plane struct {
	ID         ID        `json:"id"`
	AirlineID  AirlineID `json:"airlineId"`
	ModelID    string    `json:"modelId"`
	Level      int       `json:"level"`
	AgeDays    int       `json:"ageDays"`
	Condition  float64   `json:"condition"` // 0..1
	DepFactor  float64   `json:"depFactor"` // value depreciation, multiplied daily
	SeatsEco   int       `json:"seatsEco"`
	SeatsBiz   int       `json:"seatsBiz"`
	RouteID    ID        `json:"routeId,omitempty"`
	State      string    `json:"state"`
	BusyUntil  int64     `json:"busyUntil"` // tick when the current flight, overhaul or reconfiguration ends
	FlightFrom string    `json:"flightFrom,omitempty"`
	FlightTo   string    `json:"flightTo,omitempty"`
	DepartTick int64     `json:"departTick,omitempty"`
}

// Route is a scheduled connection operated by one airline.
type Route struct {
	ID            ID        `json:"id"`
	AirlineID     AirlineID `json:"airlineId"`
	From          string    `json:"from"`
	To            string    `json:"to"`
	PriceEcoCents int64     `json:"priceEcoCents"`
	PriceBizCents int64     `json:"priceBizCents"`
	PlaneIDs      []ID      `json:"planeIds"` // sorted
	OpenedTick    int64     `json:"openedTick"`
}

// Market is the competition on one airport pair.
type Market struct {
	Key        string                `json:"key"`
	Background []BackgroundCarrier   `json:"background"`
	LastShares map[AirlineID]float64 `json:"lastShares"`
}

// BackgroundCarrier stands in for an airline the game doesn't model (TECH_DESIGN §7.3).
type BackgroundCarrier struct {
	ID            string  `json:"id"`
	PriceEcoCents int64   `json:"priceEcoCents"`
	PriceBizCents int64   `json:"priceBizCents"`
	Quality       float64 `json:"quality"`
	SeatsPerDay   int     `json:"seatsPerDay"`
}

// Loan is borrowed money repaid weekly.
type Loan struct {
	ID              ID        `json:"id"`
	AirlineID       AirlineID `json:"airlineId"`
	PrincipalCents  int64     `json:"principalCents"`
	BalanceCents    int64     `json:"balanceCents"`
	AnnualRate      float64   `json:"annualRate"`
	WeeksLeft       int       `json:"weeksLeft"`
	InstalmentCents int64     `json:"instalmentCents"`
	MissedPayments  int       `json:"missedPayments"`
}

// Offer is a country buyout or airline takeover between airlines.
type Offer struct {
	ID          ID        `json:"id"`
	Kind        string    `json:"kind"` // countryBuyout | takeover
	From        AirlineID `json:"from"`
	To          AirlineID `json:"to"`
	Subject     string    `json:"subject"` // ISO2 or airline ID
	PriceCents  int64     `json:"priceCents"`
	ExpiresTick int64     `json:"expiresTick"`
	Status      string    `json:"status"`
}

// Mission is a player contract from the mission board.
type Mission struct {
	ID           ID                `json:"id"`
	TemplateID   string            `json:"templateId"`
	Params       map[string]string `json:"params"`
	DeadlineTick int64             `json:"deadlineTick"`
	Status       string            `json:"status"`
}

// PendingEvent is an event or investment waiting for the player's choice.
type PendingEvent struct {
	ID          ID     `json:"id"`
	EventID     string `json:"eventId"`
	ExpiresTick int64  `json:"expiresTick"`
	Chosen      string `json:"chosen,omitempty"`
	RevealTick  int64  `json:"revealTick,omitempty"`
}

// Modifier is a temporary or permanent change to a stat (ADR-0005, TECH_DESIGN §6).
type Modifier struct {
	ID          string  `json:"id"`
	Source      string  `json:"source"`
	Scope       string  `json:"scope"`  // global | airline | country | airport | route | plane
	Target      string  `json:"target"` // ID in the scope, "" for all
	Stat        string  `json:"stat"`
	Op          string  `json:"op"` // add | mul | set
	Value       float64 `json:"value"`
	Priority    int     `json:"priority"`
	ExpiresTick int64   `json:"expiresTick"` // 0 = permanent
}

// Fuel is the global fuel market.
type Fuel struct {
	PriceCents  int64   `json:"priceCents"` // per fuel unit
	History     []int64 `json:"history"`    // daily prices, oldest first, at most 90
	ShockLeft   int     `json:"shockLeft"`  // days of the current shock remaining
	ShockPerDay int64   `json:"shockPerDay"`
}

// UsedMarket lists second-hand planes for sale.
type UsedMarket struct {
	RefreshTick int64        `json:"refreshTick"`
	Offers      []PlaneOffer `json:"offers"`
}

// PlaneOffer is one used plane for sale.
type PlaneOffer struct {
	ID         ID      `json:"id"`
	ModelID    string  `json:"modelId"`
	Level      int     `json:"level"`
	AgeDays    int     `json:"ageDays"`
	Condition  float64 `json:"condition"`
	PriceCents int64   `json:"priceCents"`
}

// New returns an empty world for a seed. sim.NewGame fills in the airlines.
func New(seed uint64) *State {
	return &State{
		Version:        SchemaVersion,
		Seed:           seed,
		Speed:          1,
		Difficulty:     "normal",
		Airlines:       map[AirlineID]*Airline{},
		Countries:      map[string]*Country{},
		Airports:       map[string]*Airport{},
		LandingRights:  map[string]*LandingRight{},
		Planes:         map[ID]*Plane{},
		Routes:         map[ID]*Route{},
		Markets:        map[string]*Market{},
		Loans:          map[ID]*Loan{},
		Offers:         map[ID]*Offer{},
		Missions:       map[ID]*Mission{},
		Pending:        map[ID]*PendingEvent{},
		Modifiers:      map[string]*Modifier{},
		Milestones:     map[string]int64{},
		ConfigSnapshot: map[string]string{},
	}
}

// Day returns the current game day (0-based).
func (s *State) Day() int64 { return s.Tick / TicksPerDay }

// NewID allocates the next entity ID with a prefix, e.g. "pl1", "rt2". IDs are
// sequential, so the same commands always produce the same IDs.
func (s *State) NewID(prefix string) ID {
	s.NextID++
	return ID(prefix + strconv.FormatInt(s.NextID, 10))
}

// Player returns the human player's airline, or nil before a game is set up.
func (s *State) Player() *Airline { return s.Airlines[PlayerID] }

// SortedKeys returns a map's keys in ascending order. Use it whenever game logic
// walks a map (ADR-0006: never range over a map).
func SortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	return slices.Sorted(maps.Keys(m))
}

// LandingRightKey is the LandingRights map key for an airline at an airport.
func LandingRightKey(airline AirlineID, iata string) string {
	return string(airline) + ":" + iata
}

// MarketKey is the Markets map key for an airport pair, independent of direction.
func MarketKey(a, b string) string {
	if b < a {
		a, b = b, a
	}
	return a + "-" + b
}

// Hash returns a SHA-256 hex digest of the canonical JSON encoding. encoding/json
// writes struct fields in declaration order and map keys sorted, so the digest is
// identical on every platform.
func (s *State) Hash() string {
	b, err := json.Marshal(s)
	if err != nil {
		panic("state: marshal for hash: " + err.Error()) // State contains only marshalable fields
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
