package script_test

import (
	"flag"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/script"
)

var update = flag.Bool("update", false, "rewrite scripts/*.golden from the current build")

// TestDeterministicScripts replays every embedded script and compares the state
// hash at the end of each day with its golden file. CI runs it on amd64, arm64
// and wasm (ADR-0006). Regenerate goldens only for intentional changes:
//
//	go test ./sim/script -run TestDeterministicScripts -update
func TestDeterministicScripts(t *testing.T) {
	all, err := script.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 3 {
		t.Fatalf("found %d scripts, want at least 3", len(all))
	}
	for _, e := range all {
		got, err := script.Run(e.Script)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != e.Script.Days {
			t.Fatalf("%s: %d day hashes, want %d", e.Script.Name, len(got), e.Script.Days)
		}
		if *update {
			name := goldenPath(t, e.Script.Name)
			if err := os.WriteFile(name, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		for day := range got {
			if day >= len(e.Golden) || got[day] != e.Golden[day] {
				t.Fatalf("%s: first divergence on day %d: got %s", e.Script.Name, day+1, got[day])
			}
		}
		if len(e.Golden) != len(got) {
			t.Fatalf("%s: golden has %d days, run has %d", e.Script.Name, len(e.Golden), len(got))
		}
	}
}

func goldenPath(t *testing.T, name string) string {
	t.Helper()
	entries, err := os.ReadDir("scripts")
	if err != nil {
		t.Fatal(err)
	}
	for _, en := range entries {
		data, err := os.ReadFile("scripts/" + en.Name())
		if err == nil && strings.HasSuffix(en.Name(), ".json") && strings.Contains(string(data), `"name": "`+name+`"`) {
			return "scripts/" + strings.TrimSuffix(en.Name(), ".json") + ".golden"
		}
	}
	t.Fatalf("no script file for %q", name)
	return ""
}

func TestRunIsDeterministicAndSensitive(t *testing.T) {
	s, err := script.Parse([]byte(`{"name":"x","game":{"seed":1,"airlineName":"T","homeCountry":"GB","difficulty":"normal"},"days":2,"steps":[{"atTick":3,"command":{"type":"setSpeed","speed":4}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	a, err := script.Run(s)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := script.Run(s)
	if !reflect.DeepEqual(a, b) || len(a) != 2 {
		t.Fatalf("runs differ or wrong length: %v vs %v", a, b)
	}
	s.Steps[0].AtTick = 30 // same command a day later changes day 1's state
	c, _ := script.Run(s)
	if c[0] == a[0] {
		t.Error("moving a command to another day did not change day 1's hash")
	}
	if c[1] != a[1] {
		t.Error("speed is the same by the end of day 2, so the hash must match")
	}
}

func TestParseErrors(t *testing.T) {
	for _, bad := range []string{
		`not json`,
		`{"name":"x","seed":1,"days":0}`,
		`{"name":"x","seed":1,"days":-1}`,
		`{"name":"x","seed":1,"days":1,"steps":[{"atTick":5,"command":{}},{"atTick":4,"command":{}}]}`,
	} {
		if _, err := script.Parse([]byte(bad)); err == nil {
			t.Errorf("Parse(%s) succeeded", bad)
		}
	}
	if _, err := script.Parse([]byte(`{"name":"x","seed":1,"days":1,"steps":[{"atTick":4,"command":{}},{"atTick":4,"command":{}}]}`)); err != nil {
		t.Errorf("equal ticks must be allowed: %v", err)
	}
}

func TestRunRejectsUndecodableCommand(t *testing.T) {
	s, _ := script.Parse([]byte(`{"name":"x","game":{"seed":1,"airlineName":"T","homeCountry":"GB","difficulty":"normal"},"days":1,"steps":[{"atTick":0,"command":{"type":"nope"}}]}`))
	if _, err := script.Run(s); err == nil {
		t.Fatal("unknown command type did not fail the run")
	}
}

func TestRunRejectsInvalidGame(t *testing.T) {
	s, _ := script.Parse([]byte(`{"name":"x","game":{"seed":1,"airlineName":"","homeCountry":"GB","difficulty":"normal"},"days":1}`))
	if _, err := script.Run(s); err == nil || !strings.Contains(err.Error(), "new game") {
		t.Errorf("invalid game params: %v", err)
	}
}
