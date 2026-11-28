package ledger_test

import (
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/engine"
	"github.com/MIcQo/jetstream-tycoon/sim/state"
	"github.com/MIcQo/jetstream-tycoon/sim/systems/ledger"
)

func TestLedger(t *testing.T) {
	if (ledger.System{}).Name() != "ledger" {
		t.Fatal("name")
	}
	s := state.New(3)
	s.Airlines[state.PlayerID] = &state.Airline{ID: state.PlayerID, CashCents: 1000}
	e := engine.New(s, nil, ledger.System{})
	out := e.Advance(48)
	gain := s.Player().CashCents - 1000
	if gain <= 0 || gain > 48*100 {
		t.Fatalf("cash gain over 48 ticks = %d, want in (0, 4800]", gain)
	}
	if len(out.Notifications) != 2 {
		t.Fatalf("notifications = %+v, want 2 day starts", out.Notifications)
	}
	for i, n := range out.Notifications {
		if n.Key != "notifications.day.started" || n.Params["day"] != []string{"1", "2"}[i] {
			t.Errorf("notification %d = %+v", i, n)
		}
	}
}

func TestLedgerWithoutPlayer(t *testing.T) {
	e := engine.New(state.New(3), nil, ledger.System{})
	e.Advance(5) // must not panic
	if e.State.Player() != nil {
		t.Fatal("player appeared")
	}
}
