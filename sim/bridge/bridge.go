// Package bridge implements the simulation API that sim.wasm exposes to the Web
// Worker as `globalThis.jetstreamSim` (TECH_DESIGN §3). Requests and responses
// are JSON strings; cmd/simwasm only wires these methods to syscall/js.
//
// M0 returns a small full view per call; M2 moves state traffic to MessagePack
// deltas with tombstones.
package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/MIcQo/jetstream-tycoon/sim"
	"github.com/MIcQo/jetstream-tycoon/sim/command"
	"github.com/MIcQo/jetstream-tycoon/sim/engine"
)

// View is the UI state view.
type View struct {
	Tick      int64 `json:"tick"`
	Day       int64 `json:"day"`
	Speed     int   `json:"speed"`
	CashCents int64 `json:"cashCents"`
}

// AdvanceResult is the response of Advance.
type AdvanceResult struct {
	View          View                  `json:"view"`
	Notifications []engine.Notification `json:"notifications"`
	Rejected      []engine.Rejection    `json:"rejected"`
	// Reports are problems the simulation recovered from since the last Advance
	// (for example a non-finite formula result clamped in a release build). The UI
	// forwards them to Sentry; they never affect the game state.
	Reports []Report `json:"reports"`
}

// Report describes a recovered problem for error reporting.
type Report struct {
	Kind    string `json:"kind"`    // "nonFinite"
	Context string `json:"context"` // where it happened, e.g. the formula name
	Detail  string `json:"detail"`  // value or message
	Tick    int64  `json:"tick"`
}

// maxReports bounds the pending reports so a broken formula can't grow memory
// without limit between two Advance calls.
const maxReports = 50

// CommandResult is the response of Command.
type CommandResult struct {
	OK       bool   `json:"ok"`
	ErrorKey string `json:"errorKey,omitempty"`
}

// ErrNotStarted is reported when Advance or Command runs before NewGame.
var ErrNotStarted = &command.Error{Key: "errors.sim.not_started"}

// Session holds one running game.
type Session struct {
	engine  *engine.Engine
	reports []Report
}

// ReportNonFinite records a NaN/±Inf that detmath.Finite clamped (release builds).
// It is wired to detmath.NonFiniteHook by cmd/simwasm.
func (s *Session) ReportNonFinite(context string, x float64) {
	if len(s.reports) >= maxReports {
		return
	}
	var tick int64
	if s.engine != nil {
		tick = s.engine.State.Tick
	}
	s.reports = append(s.reports, Report{Kind: "nonFinite", Context: context, Detail: fmt.Sprint(x), Tick: tick})
}

// NewGame starts a new game from `{"seed": n}` and returns the initial view.
func (s *Session) NewGame(requestJSON string) (string, error) {
	var req struct {
		Seed uint64 `json:"seed"`
	}
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		return "", fmt.Errorf("newGame request: %w", err)
	}
	s.engine = sim.NewGame(req.Seed)
	return marshal(s.view())
}

// Advance applies queued commands and runs ticks.
func (s *Session) Advance(ticks int) (string, error) {
	if s.engine == nil {
		return "", ErrNotStarted
	}
	if ticks < 0 {
		return "", fmt.Errorf("advance: negative tick count %d", ticks)
	}
	out := s.engine.Advance(ticks)
	res := AdvanceResult{View: s.view(), Notifications: out.Notifications, Rejected: out.Rejected}
	if res.Notifications == nil {
		res.Notifications = []engine.Notification{}
	}
	if res.Rejected == nil {
		res.Rejected = []engine.Rejection{}
	}
	res.Reports = s.reports
	if res.Reports == nil {
		res.Reports = []Report{}
	}
	s.reports = nil
	return marshal(res)
}

// Command validates a command against the current state and queues it for the
// next tick boundary. It is validated again when applied (ADR-0020).
func (s *Session) Command(commandJSON string) string {
	if s.engine == nil {
		return mustMarshal(CommandResult{ErrorKey: ErrNotStarted.Key})
	}
	c, err := command.Decode([]byte(commandJSON))
	if err == nil {
		err = c.Validate(s.engine.State)
	}
	if err != nil {
		return mustMarshal(CommandResult{ErrorKey: errorKey(err)})
	}
	s.engine.Enqueue(c)
	return mustMarshal(CommandResult{OK: true})
}

func (s *Session) view() View {
	st := s.engine.State
	return View{Tick: st.Tick, Day: st.Day(), Speed: st.Speed, CashCents: st.CashCents}
}

// Guard runs a bridge call and always returns a JSON string: the call's result, or
// `{"error": "..."}` if it returned an error or panicked. A panic must never cross
// into JavaScript, because it would stop the Go runtime for the rest of the session.
func Guard(call func() (string, error)) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = errorJSON(fmt.Sprintf("panic: %v\n%s", r, truncate(debug.Stack(), maxStackBytes)))
		}
	}()
	res, err := call()
	if err != nil {
		return errorJSON(err.Error())
	}
	return res
}

// maxStackBytes keeps panic reports small enough for Sentry and postMessage.
const maxStackBytes = 4096

func truncate(b []byte, n int) []byte {
	if len(b) > n {
		return b[:n]
	}
	return b
}

func errorJSON(msg string) string {
	b, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{msg}) // a struct of one string always marshals
	return string(b)
}

func errorKey(err error) string {
	var ce *command.Error
	if errors.As(err, &ce) {
		return ce.Key
	}
	return "errors.cmd.unknown"
}

func marshal(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

func mustMarshal(v CommandResult) string {
	s, _ := marshal(v) // CommandResult always marshals
	return s
}
