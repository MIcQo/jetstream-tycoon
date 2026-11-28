package bundle_test

import (
	"bytes"
	"compress/gzip"
	"regexp"
	"strings"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/content/bundle"
)

func TestEmbeddedBundleLoads(t *testing.T) {
	b, err := bundle.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^c[0-9a-f]{12}$`).MatchString(b.ContentVersion) {
		t.Errorf("contentVersion = %q", b.ContentVersion)
	}
	w := b.World
	if len(b.Content.Planes) < 3 || len(b.Content.AirportLevels) != 5 || len(w.Countries) < 200 || len(w.Airports) < 900 || len(w.Cities) < 900 {
		t.Errorf("bundle too small: %d planes, %d levels, %d countries, %d airports, %d cities",
			len(b.Content.Planes), len(b.Content.AirportLevels), len(w.Countries), len(w.Airports), len(w.Cities))
	}
	if c := w.Candidates["LHR"]; len(c) != 30 || c[0].Km <= 0 || c[0].DistFactor <= 0 {
		t.Errorf("LHR candidates = %+v", c)
	}
	again, _ := bundle.Load()
	if again.ContentVersion != b.ContentVersion {
		t.Error("Load is not stable")
	}
}

func gz(s string) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write([]byte(s))
	_ = w.Close()
	return buf.Bytes()
}

func TestDecode(t *testing.T) {
	b, err := bundle.Decode(gz(`{"contentVersion":"c1","content":{"planes":[]},"world":{"candidates":{"AAA":[["BBB",100,39.8]]}}}`))
	if err != nil || b.ContentVersion != "c1" || b.World.Candidates["AAA"][0].To != "BBB" {
		t.Fatalf("Decode = %+v, %v", b, err)
	}
	cases := map[string][]byte{
		"not gzip":        []byte("plain"),
		"truncated gzip":  gz(`{"contentVersion":"c1"}`)[:12],
		"not json":        gz(`nope`),
		"unknown field":   gz(`{"contentVersion":"c1","extra":1}`),
		"missing version": gz(`{"content":{}}`),
	}
	for name, in := range cases {
		if _, err := bundle.Decode(in); err == nil || !strings.HasPrefix(err.Error(), "bundle:") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
