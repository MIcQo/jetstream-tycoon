package content_test

import (
	"encoding/json"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/content"
)

func TestCandidateJSON(t *testing.T) {
	c := content.Candidate{To: "JFK", Km: 5540, DistFactor: 980.5}
	b, err := json.Marshal(c)
	if err != nil || string(b) != `["JFK",5540,980.5]` {
		t.Fatalf("marshal = %s, %v", b, err)
	}
	var back content.Candidate
	if err := json.Unmarshal(b, &back); err != nil || back != c {
		t.Fatalf("unmarshal = %+v, %v", back, err)
	}
	for _, bad := range []string{`{}`, `["JFK",1]`, `["JFK",1,2,3]`, `[1,2,3]`, `["JFK","x",3]`, `["JFK",1,"x"]`} {
		if err := json.Unmarshal([]byte(bad), &back); err == nil {
			t.Errorf("unmarshal(%s) succeeded", bad)
		}
	}
}

func TestDecodeInto(t *testing.T) {
	var w content.World
	if err := content.DecodeInto([]byte(`{"airports":[{"iata":"AAA","baseLevel":3}]}`), &w); err != nil || w.Airports[0].BaseLevel != 3 {
		t.Fatalf("DecodeInto = %+v, %v", w, err)
	}
	if err := content.DecodeInto([]byte(`{"airports":[{"iata":"AAA","name":"x"}]}`), &w); err == nil {
		t.Error("unknown field accepted")
	}
}
