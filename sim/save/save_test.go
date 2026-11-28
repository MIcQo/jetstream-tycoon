package save_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim"
	"github.com/MIcQo/jetstream-tycoon/sim/command"
	"github.com/MIcQo/jetstream-tycoon/sim/content/bundle"
	"github.com/MIcQo/jetstream-tycoon/sim/effects"
	"github.com/MIcQo/jetstream-tycoon/sim/save"
	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

var update = flag.Bool("update", false, "rewrite the current-schema fixture in testdata/")

// played returns a game a few days in, with a modifier, so most state fields are set.
func played(t testing.TB) *state.State {
	t.Helper()
	b, err := bundle.Load()
	if err != nil {
		t.Fatal(err)
	}
	e, err := sim.NewGame(sim.NewGameParams{Seed: 99, AirlineName: "Save Air", HomeCountry: "FR", Difficulty: "hard"}, &b)
	if err != nil {
		t.Fatal(err)
	}
	e.Enqueue(command.SetSpeed{Speed: 2})
	e.Advance(24*3 + 5)
	if err := effects.Add(e.State, state.Modifier{Source: "test", Scope: effects.ScopeCountry, Target: "FR", Stat: effects.StatDemand, Op: effects.OpMul, Value: 1.25, ExpiresTick: 500}); err != nil {
		t.Fatal(err)
	}
	return e.State
}

func TestRoundTripKeepsTheStateHash(t *testing.T) {
	for name, s := range map[string]*state.State{"empty": state.New(5), "played": played(t)} {
		data, err := save.Encode(s, "c123")
		if err != nil {
			t.Fatal(err)
		}
		back, meta, err := save.Decode(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if back.Hash() != s.Hash() {
			a, _ := json.Marshal(s)
			b, _ := json.Marshal(back)
			t.Fatalf("%s: hash changed after a round trip\nbefore %s\nafter  %s", name, a, b)
		}
		if meta.ContentVersion != "c123" || meta.SchemaVersion != state.SchemaVersion || meta.Tick != s.Tick {
			t.Errorf("%s: meta = %+v", name, meta)
		}
	}
}

func TestEncodeIsDeterministicAndCompact(t *testing.T) {
	s := played(t)
	// Many entries in typed maps: Go randomises map order on every walk.
	for _, id := range []string{"e", "b", "d", "a", "c", "f", "h", "g"} {
		s.Milestones[id] = 1
		s.Countries[id] = &state.Country{ISO2: id}
		s.Markets[id] = &state.Market{Key: id, LastShares: map[state.AirlineID]float64{"P": 0.5, "A1": 0.3, "A2": 0.2, "A3": 0.1}}
	}
	a, _ := save.Encode(s, "c1")
	for range 30 {
		if b, _ := save.Encode(s, "c1"); !bytes.Equal(a, b) {
			t.Fatal("encoding the same state twice gave different bytes")
		}
	}
	j, _ := json.Marshal(s)
	if len(a) >= len(j) {
		t.Errorf("save is %d bytes, JSON state is %d: expected smaller", len(a), len(j))
	}
	if !bytes.HasPrefix(a, []byte("JTSV\x01")) {
		t.Errorf("header = %q", a[:5])
	}
}

func TestMeta(t *testing.T) {
	s := played(t)
	m := save.MetaOf(s, "cv")
	if m.AirlineName != "Save Air" || m.HomeCountry != "FR" || m.CashCents != s.Player().CashCents || m.Day != 3 || m.Tick != 77 {
		t.Errorf("meta = %+v", m)
	}
	if empty := save.MetaOf(state.New(1), ""); empty.AirlineName != "" || empty.CashCents != 0 {
		t.Errorf("meta without player = %+v", empty)
	}
}

func TestExportJSON(t *testing.T) {
	data, _ := save.Encode(played(t), "cjson")
	out, err := save.ExportJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Meta  save.Meta   `json:"meta"`
		State state.State `json:"state"`
	}
	if err := json.Unmarshal(out, &doc); err != nil || doc.Meta.ContentVersion != "cjson" || doc.State.Player().Name != "Save Air" {
		t.Fatalf("export = %.200s, %v", out, err)
	}
	if _, err := save.ExportJSON([]byte("nope")); !errors.Is(err, save.ErrNotASave) {
		t.Errorf("export of garbage: %v", err)
	}
}

func TestDecodeErrors(t *testing.T) {
	good, _ := save.Encode(state.New(1), "c")
	cases := map[string]struct {
		data []byte
		want error
	}{
		"empty":       {nil, save.ErrNotASave},
		"short":       {[]byte("JTS"), save.ErrNotASave},
		"wrong magic": {append([]byte("XXXX"), good[4:]...), save.ErrNotASave},
		"format":      {append([]byte("JTSV\x09"), good[5:]...), save.ErrFormat},
		"not gzip":    {[]byte("JTSV\x01not compressed"), save.ErrCorrupt},
		"truncated":   {good[:len(good)-4], save.ErrCorrupt},
	}
	for name, tc := range cases {
		if _, _, err := save.Decode(tc.data); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
}

func TestMigrate(t *testing.T) {
	if err := save.Migrate(map[string]any{}, state.SchemaVersion+1); !errors.Is(err, save.ErrNewerSave) {
		t.Errorf("newer schema: %v", err)
	}
	if err := save.Migrate(map[string]any{}, 0); !errors.Is(err, save.ErrNoMigrator) {
		t.Errorf("schema 0 has no migrator: %v", err)
	}
	st := map[string]any{"cashCents": "lots"}
	if err := save.Migrate(st, 1); err == nil || !strings.Contains(err.Error(), "migrate schema 1 → 2") {
		t.Errorf("bad v1 cash: %v", err)
	}
	current := map[string]any{"version": state.SchemaVersion}
	if err := save.Migrate(current, state.SchemaVersion); err != nil || current["version"] != state.SchemaVersion {
		t.Errorf("current schema: %v %v", err, current)
	}
	// Every integer type MessagePack may produce is accepted.
	for _, v := range []any{int8(5), int16(5), int32(5), int64(5), uint8(5), uint16(5), uint32(5), uint64(5)} {
		st := map[string]any{"cashCents": v}
		if err := save.Migrate(st, 1); err != nil {
			t.Fatalf("%T: %v", v, err)
		}
		if cash := st["airlines"].(map[string]any)["P"].(map[string]any)["cashCents"]; cash != int64(5) {
			t.Errorf("%T: migrated cash %v", v, cash)
		}
	}
	st = map[string]any{}
	if err := save.Migrate(st, 1); err != nil || st["version"] != 2 {
		t.Errorf("v1 without cash: %v %v", err, st)
	}
}

// TestFixtureV1 loads a real schema-1 save (M0 shape: a bare cashCents balance)
// and checks the migration result (TECH_DESIGN §9, #29).
func TestFixtureV1(t *testing.T) {
	data, err := os.ReadFile("testdata/schema1.save")
	if err != nil {
		t.Fatal(err)
	}
	s, meta, err := save.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	p := s.Player()
	if s.Version != state.SchemaVersion || meta.SchemaVersion != state.SchemaVersion || s.Seed != 42 || s.Tick != 24 ||
		p == nil || p.CashCents != 10_001_195 || p.Controller != "human" || s.Difficulty != "normal" {
		t.Fatalf("migrated v1 = %+v, player %+v", s, p)
	}
	if s.Planes == nil || s.Modifiers == nil {
		t.Error("maps missing in the migrated state must be initialised")
	}
}

// TestFixtureCurrentSchema loads a committed save of the current schema. When a
// later version bumps the schema, this file must keep loading (through the new
// migrator): keep it, and add a new fixture for the new schema.
func TestFixtureCurrentSchema(t *testing.T) {
	path := "testdata/schema2.save"
	if *update {
		data, err := save.Encode(played(t), "cfixture")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s, meta, err := save.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if meta.ContentVersion != "cfixture" || s.Player().Name != "Save Air" || s.Tick != 77 || len(s.Modifiers) != 1 {
		t.Errorf("fixture = %+v", meta)
	}
}
