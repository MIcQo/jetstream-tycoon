package save

import (
	"bytes"
	"compress/gzip"
	"os"
	"testing"
)

// TestWriteSchema1Fixture regenerates testdata/schema1.save, a save in the M0
// schema-1 shape, when WRITE_V1_FIXTURE=1 is set. It is a generator, not a check.
func TestWriteSchema1Fixture(t *testing.T) {
	if os.Getenv("WRITE_V1_FIXTURE") == "" {
		t.Skip("set WRITE_V1_FIXTURE=1 to regenerate testdata/schema1.save")
	}
	env := map[string]any{
		"meta":  map[string]any{"schemaVersion": 1, "contentVersion": "", "tick": 24, "day": 1},
		"state": map[string]any{"version": 1, "seed": uint64(42), "tick": int64(24), "speed": 1, "cashCents": int64(10_001_195)},
	}
	var raw bytes.Buffer
	if err := newEncoder(&raw).Encode(env); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	out.Write(Magic[:])
	out.WriteByte(FormatVersion)
	zw, _ := gzip.NewWriterLevel(&out, gzip.BestCompression)
	_, _ = zw.Write(raw.Bytes())
	_ = zw.Close()
	if err := os.WriteFile("testdata/schema1.save", out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
