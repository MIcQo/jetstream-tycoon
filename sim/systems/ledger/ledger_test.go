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
	e := engine.New(state.New(3), ledger.System{})
	start := e.State.CashCents
	out := e.Advance(48)
	gain := e.State.CashCents - start
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
