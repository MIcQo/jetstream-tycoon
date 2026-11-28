package save

import (
	"fmt"

	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

// Migrator upgrades a decoded state from schema version N to N+1, in place.
// It works on generic maps because the old shape no longer matches state.State.
type Migrator func(st map[string]any) error

// migrators[N] upgrades schema N to N+1. Add one whenever state.SchemaVersion is
// bumped, with a fixture test of a real save of version N (testdata/).
var migrators = map[int]Migrator{
	1: migrateV1toV2,
}

// Migrate upgrades st from version to state.SchemaVersion.
func Migrate(st map[string]any, version int) error {
	if version > state.SchemaVersion {
		return fmt.Errorf("%w (schema %d, this build supports %d)", ErrNewerSave, version, state.SchemaVersion)
	}
	for v := version; v < state.SchemaVersion; v++ {
		m, ok := migrators[v]
		if !ok {
			return fmt.Errorf("%w %d", ErrNoMigrator, v)
		}
		if err := m(st); err != nil {
			return fmt.Errorf("save: migrate schema %d → %d: %w", v, v+1, err)
		}
		st["version"] = v + 1
	}
	return nil
}

// migrateV1toV2: schema 1 (M0) kept a single cashCents balance on the state. Schema
// 2 has airlines; the balance moves to a player airline. Early test saves have no
// home country or name, so those stay empty.
func migrateV1toV2(st map[string]any) error {
	cash, err := toInt64(st["cashCents"])
	if err != nil {
		return fmt.Errorf("cashCents: %w", err)
	}
	delete(st, "cashCents")
	st["difficulty"] = "normal"
	st["airlines"] = map[string]any{
		string(state.PlayerID): map[string]any{
			"id": string(state.PlayerID), "controller": "human", "cashCents": cash,
			"reputation": 50.0, "creditScore": 500, "status": "ok", "hubs": []any{},
		},
	}
	return nil
}

// toInt64 accepts the integer types MessagePack decodes into.
func toInt64(v any) (int64, error) {
	switch n := v.(type) {
	case int8:
		return int64(n), nil
	case int16:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case uint8:
		return int64(n), nil
	case uint16:
		return int64(n), nil
	case uint32:
		return int64(n), nil
	case uint64:
		return int64(n), nil //nolint:gosec // save values fit in int64
	case nil:
		return 0, nil
	}
	return 0, fmt.Errorf("want an integer, got %T", v)
}
