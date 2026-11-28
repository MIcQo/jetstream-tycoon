// Package script runs scripted games for determinism checks (TECH_DESIGN §15):
// a seed, a number of days and timed commands, producing one state hash per day.
// The same embedded scripts run natively (amd64, arm64) and compiled to wasm.
package script

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/MIcQo/jetstream-tycoon/sim"
	"github.com/MIcQo/jetstream-tycoon/sim/command"
	"github.com/MIcQo/jetstream-tycoon/sim/content/bundle"
	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

//go:embed scripts/*.json scripts/*.golden
var files embed.FS

// Step queues a command just before the given tick runs.
type Step struct {
	AtTick  int64           `json:"atTick"`
	Command json.RawMessage `json:"command"`
}

// Script is a scripted game.
type Script struct {
	Name  string            `json:"name"`
	Game  sim.NewGameParams `json:"game"`
	Days  int               `json:"days"`
	Steps []Step            `json:"steps"`
}

// Parse decodes a script and checks that steps are in tick order.
func Parse(data []byte) (Script, error) {
	var s Script
	if err := json.Unmarshal(data, &s); err != nil {
		return Script{}, fmt.Errorf("parse script: %w", err)
	}
	if s.Days <= 0 {
		return Script{}, fmt.Errorf("script %q: days must be positive", s.Name)
	}
	for i := 1; i < len(s.Steps); i++ {
		if s.Steps[i].AtTick < s.Steps[i-1].AtTick {
			return Script{}, fmt.Errorf("script %q: steps out of tick order at step %d", s.Name, i)
		}
	}
	return s, nil
}

// Run plays the script with the embedded content and returns the state hash at
// the end of every day.
func Run(s Script) ([]string, error) {
	data, err := bundle.Load()
	if err != nil {
		return nil, err
	}
	e, err := sim.NewGame(s.Game, &data)
	if err != nil {
		return nil, fmt.Errorf("script %q: new game: %w", s.Name, err)
	}
	hashes := make([]string, 0, s.Days)
	next := 0
	for tick := int64(0); tick < int64(s.Days)*state.TicksPerDay; tick++ {
		for next < len(s.Steps) && s.Steps[next].AtTick == tick {
			c, err := command.Decode(s.Steps[next].Command)
			if err != nil {
				return nil, fmt.Errorf("script %q step %d: %w", s.Name, next, err)
			}
			e.Enqueue(c)
			next++
		}
		e.Advance(1)
		if e.State.Tick%state.TicksPerDay == 0 {
			hashes = append(hashes, e.State.Hash())
		}
	}
	return hashes, nil
}

// Embedded is a script shipped with the binary, with its golden day hashes.
type Embedded struct {
	Script Script
	Golden []string
}

// All returns the embedded scripts sorted by file name.
func All() ([]Embedded, error) {
	names, err := files.ReadDir("scripts")
	if err != nil {
		return nil, err
	}
	sort.Slice(names, func(i, j int) bool { return names[i].Name() < names[j].Name() })
	var out []Embedded
	for _, n := range names {
		if path.Ext(n.Name()) != ".json" {
			continue
		}
		data, err := files.ReadFile("scripts/" + n.Name())
		if err != nil {
			return nil, err
		}
		s, err := Parse(data)
		if err != nil {
			return nil, err
		}
		golden, err := files.ReadFile("scripts/" + strings.TrimSuffix(n.Name(), ".json") + ".golden")
		if err != nil {
			return nil, fmt.Errorf("script %q has no golden file: %w", s.Name, err)
		}
		out = append(out, Embedded{Script: s, Golden: strings.Fields(string(golden))})
	}
	return out, nil
}
