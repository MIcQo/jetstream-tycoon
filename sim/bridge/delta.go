package bridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"reflect"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/MIcQo/jetstream-tycoon/sim/save"
	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

// Delta is the MessagePack state traffic to the UI (TECH_DESIGN §3). A full
// snapshot (`init`) has Full set and every entity in Changed; an update holds
// only the entities that changed since the previous Delta, sent whole, and
// tombstones for removed ones.
type Delta struct {
	Full    bool                      `json:"full"`
	Tick    int64                     `json:"tick"`
	Changed map[string]map[string]any `json:"changed"` // collection → ID → entity
	Deleted map[string][]string       `json:"deleted"` // collection → IDs, sorted
	Flights []Flight                  `json:"flights"` // planes in the air, for interpolation
}

// Flight is a plane in the air (sent for planes that changed).
type Flight struct {
	PlaneID    state.ID `json:"planeId"`
	From       string   `json:"from"`
	To         string   `json:"to"`
	DepartTick int64    `json:"departTick"`
	ArriveTick int64    `json:"arriveTick"`
}

// collection names one entity map of the state for the UI.
type collection struct {
	name  string
	items func(*state.State) ([]string, func(string) any)
}

func coll[K ~string, V any](name string, pick func(*state.State) map[K]V) collection {
	return collection{name: name, items: func(s *state.State) ([]string, func(string) any) {
		m := pick(s)
		keys := state.SortedKeys(m)
		ids := make([]string, len(keys))
		for i, k := range keys {
			ids[i] = string(k)
		}
		return ids, func(id string) any { return m[K(id)] }
	}}
}

func single[V any](name string, pick func(*state.State) V) collection {
	return collection{name: name, items: func(s *state.State) ([]string, func(string) any) {
		return []string{name}, func(string) any { return pick(s) }
	}}
}

// collections is the UI state view: every entity the UI renders. The order is
// fixed; the per-collection key order comes from SortedKeys.
var collections = []collection{
	coll("airlines", func(s *state.State) map[state.AirlineID]*state.Airline { return s.Airlines }),
	coll("countries", func(s *state.State) map[string]*state.Country { return s.Countries }),
	coll("airports", func(s *state.State) map[string]*state.Airport { return s.Airports }),
	coll("landingRights", func(s *state.State) map[string]*state.LandingRight { return s.LandingRights }),
	coll("planes", func(s *state.State) map[state.ID]*state.Plane { return s.Planes }),
	coll("routes", func(s *state.State) map[state.ID]*state.Route { return s.Routes }),
	coll("markets", func(s *state.State) map[string]*state.Market { return s.Markets }),
	coll("loans", func(s *state.State) map[state.ID]*state.Loan { return s.Loans }),
	coll("offers", func(s *state.State) map[state.ID]*state.Offer { return s.Offers }),
	coll("missions", func(s *state.State) map[state.ID]*state.Mission { return s.Missions }),
	coll("pending", func(s *state.State) map[state.ID]*state.PendingEvent { return s.Pending }),
	coll("modifiers", func(s *state.State) map[string]*state.Modifier { return s.Modifiers }),
	coll("milestones", func(s *state.State) map[string]int64 { return s.Milestones }),
	single("world", func(s *state.State) worldView { return worldView{s.Fuel, s.UsedMarket} }),
}

// worldView holds the global markets (one entity with ID "world").
type worldView struct {
	Fuel       state.Fuel       `json:"fuel"`
	UsedMarket state.UsedMarket `json:"usedMarket"`
}

// Tracker finds the entities that changed since the last Delta by comparing a
// SHA-256 of each entity's encoding. It lives in the bridge, outside the
// deterministic core, so systems need no dirty flags and cannot forget one.
type Tracker struct {
	sent map[string]map[string][sha256.Size]byte // collection → ID → fingerprint
	buf  bytes.Buffer
	enc  *msgpack.Encoder
	// hasMap caches, per type, whether a value contains a map. The MessagePack
	// encoder writes typed maps in random order, so those types are
	// fingerprinted with encoding/json, which sorts map keys.
	hasMap map[reflect.Type]bool
}

func containsMap(t reflect.Type, seen map[reflect.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	switch t.Kind() {
	case reflect.Map:
		return true
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return containsMap(t.Elem(), seen)
	case reflect.Struct:
		for i := range t.NumField() {
			if containsMap(t.Field(i).Type, seen) {
				return true
			}
		}
	}
	return false
}

func (t *Tracker) fingerprint(v any) ([sha256.Size]byte, error) {
	typ := reflect.TypeOf(v)
	withMap, ok := t.hasMap[typ]
	if !ok {
		if t.hasMap == nil {
			t.hasMap = map[reflect.Type]bool{}
		}
		withMap = containsMap(typ, map[reflect.Type]bool{})
		t.hasMap[typ] = withMap
	}
	if withMap {
		b, err := json.Marshal(v)
		return sha256.Sum256(b), err
	}
	if t.enc == nil {
		t.enc = save.NewEncoder(&t.buf)
	}
	t.buf.Reset()
	err := t.enc.Encode(v)
	return sha256.Sum256(t.buf.Bytes()), err
}

// Snapshot returns a full Delta and makes it the new baseline.
func (t *Tracker) Snapshot(s *state.State) ([]byte, error) {
	t.sent = nil
	return t.diff(s, true)
}

// Next returns the changes since the previous Snapshot or Next.
func (t *Tracker) Next(s *state.State) ([]byte, error) {
	return t.diff(s, t.sent == nil)
}

func (t *Tracker) diff(s *state.State, full bool) ([]byte, error) {
	d := Delta{Full: full, Tick: s.Tick, Changed: map[string]map[string]any{}, Deleted: map[string][]string{}, Flights: []Flight{}}
	next := make(map[string]map[string][sha256.Size]byte, len(collections))
	for _, c := range collections {
		ids, get := c.items(s)
		prev := t.sent[c.name]
		cur := make(map[string][sha256.Size]byte, len(ids))
		for _, id := range ids {
			v := get(id)
			fp, err := t.fingerprint(v)
			if err != nil {
				return nil, err
			}
			cur[id] = fp
			if old, ok := prev[id]; ok && old == fp {
				continue
			}
			if d.Changed[c.name] == nil {
				d.Changed[c.name] = map[string]any{}
			}
			d.Changed[c.name][id] = v
			if p, ok := v.(*state.Plane); ok && p.State == state.PlaneFlying {
				d.Flights = append(d.Flights, Flight{PlaneID: p.ID, From: p.FlightFrom, To: p.FlightTo, DepartTick: p.DepartTick, ArriveTick: p.BusyUntil})
			}
		}
		for _, id := range state.SortedKeys(prev) {
			if _, ok := cur[id]; !ok {
				d.Deleted[c.name] = append(d.Deleted[c.name], id)
			}
		}
		next[c.name] = cur
	}
	b, err := save.Marshal(d)
	if err != nil {
		return nil, err
	}
	t.sent = next
	return b, nil
}
