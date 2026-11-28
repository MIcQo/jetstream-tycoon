// Package save encodes and decodes game saves (ADR-0009, ADR-0024): a short
// header, then gzip-compressed MessagePack holding metadata and the state. Old
// saves are upgraded by schema migrators when they load.
package save

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

// Magic identifies a save file; FormatVersion is the envelope layout (not the
// state schema, which is Meta.SchemaVersion).
var Magic = [4]byte{'J', 'T', 'S', 'V'}

// FormatVersion is the envelope layout version.
const FormatVersion = 1

// MaxDecodedBytes bounds decompression, so a corrupt or malicious file can't
// exhaust memory.
const MaxDecodedBytes = 64 << 20

// Meta describes a save without decoding the whole state (save lists, sync conflicts).
type Meta struct {
	SchemaVersion  int    `json:"schemaVersion"`
	ContentVersion string `json:"contentVersion"`
	Tick           int64  `json:"tick"`
	Day            int64  `json:"day"`
	AirlineName    string `json:"airlineName"`
	HomeCountry    string `json:"homeCountry"`
	CashCents      int64  `json:"cashCents"`
}

// Errors.
var (
	ErrNotASave   = errors.New("save: not a Jetstream Tycoon save")
	ErrFormat     = errors.New("save: unsupported save format")
	ErrNewerSave  = errors.New("save: made by a newer version of the game")
	ErrNoMigrator = errors.New("save: no migration for this schema version")
	ErrCorrupt    = errors.New("save: corrupt data")
)

type envelope struct {
	Meta  Meta `json:"meta"`
	State any  `json:"state"`
}

func newEncoder(w io.Writer) *msgpack.Encoder {
	enc := msgpack.NewEncoder(w)
	enc.SetSortMapKeys(true)       // sorts map[string]any and map[string]string only
	enc.SetCustomStructTag("json") // one set of field names for JSON and MessagePack
	enc.UseCompactInts(true)
	return enc
}

func newDecoder(r io.Reader) *msgpack.Decoder {
	dec := msgpack.NewDecoder(r)
	dec.SetCustomStructTag("json") // generic maps decode as map[string]any by default
	return dec
}

// canonical encodes v as MessagePack with every map's keys sorted. The library
// sorts only map[string]any and map[string]string, so v is encoded once, decoded
// into generic maps (map[string]any) and encoded again.
func canonical(v any) ([]byte, error) {
	var first bytes.Buffer
	if err := newEncoder(&first).Encode(v); err != nil {
		return nil, err
	}
	var generic any
	if err := newDecoder(&first).Decode(&generic); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := newEncoder(&out).Encode(generic); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// MetaOf builds the metadata for s.
func MetaOf(s *state.State, contentVersion string) Meta {
	m := Meta{SchemaVersion: s.Version, ContentVersion: contentVersion, Tick: s.Tick, Day: s.Day()}
	if p := s.Player(); p != nil {
		m.AirlineName, m.HomeCountry, m.CashCents = p.Name, p.Home, p.CashCents
	}
	return m
}

// Encode writes s as a save. The output is deterministic: equal states give equal bytes.
func Encode(s *state.State, contentVersion string) ([]byte, error) {
	raw, err := canonical(envelope{Meta: MetaOf(s, contentVersion), State: s})
	if err != nil {
		return nil, fmt.Errorf("save: encode: %w", err)
	}
	var out bytes.Buffer
	out.Write(Magic[:])
	out.WriteByte(FormatVersion)
	zw, err := gzip.NewWriterLevel(&out, gzip.BestCompression) // no timestamp or name: deterministic
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// decodeEnvelope checks the header, decompresses and decodes into generic maps.
func decodeEnvelope(b []byte) (map[string]any, error) {
	if len(b) < len(Magic)+1 || !bytes.Equal(b[:len(Magic)], Magic[:]) {
		return nil, ErrNotASave
	}
	if b[len(Magic)] != FormatVersion {
		return nil, fmt.Errorf("%w %d", ErrFormat, b[len(Magic)])
	}
	zr, err := gzip.NewReader(bytes.NewReader(b[len(Magic)+1:]))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err) //nolint:errorlint // ErrCorrupt is the error callers match
	}
	raw, err := io.ReadAll(io.LimitReader(zr, MaxDecodedBytes+1))
	if err != nil || len(raw) > MaxDecodedBytes {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err) //nolint:errorlint // ErrCorrupt is the error callers match
	}
	var env map[string]any
	if err := newDecoder(bytes.NewReader(raw)).Decode(&env); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err) //nolint:errorlint // ErrCorrupt is the error callers match
	}
	return env, nil
}

// Decode reads a save, migrating it to the current schema.
func Decode(b []byte) (*state.State, Meta, error) {
	env, err := decodeEnvelope(b)
	if err != nil {
		return nil, Meta{}, err
	}
	var meta Meta
	if err := convert(env["meta"], &meta); err != nil {
		return nil, Meta{}, fmt.Errorf("%w: meta: %v", ErrCorrupt, err) //nolint:errorlint // ErrCorrupt is the error callers match
	}
	st, ok := env["state"].(map[string]any)
	if !ok {
		return nil, Meta{}, fmt.Errorf("%w: no state", ErrCorrupt)
	}
	if err := Migrate(st, meta.SchemaVersion); err != nil {
		return nil, Meta{}, err
	}
	s := state.New(0)
	if err := convert(st, s); err != nil {
		return nil, Meta{}, fmt.Errorf("%w: state: %v", ErrCorrupt, err) //nolint:errorlint // ErrCorrupt is the error callers match
	}
	meta.SchemaVersion = s.Version
	return s, meta, nil
}

// convert re-encodes a generic value into a typed one through MessagePack.
func convert(from any, to any) error {
	var buf bytes.Buffer
	if err := newEncoder(&buf).Encode(from); err != nil {
		return err
	}
	return newDecoder(&buf).Decode(to)
}

// ExportJSON decodes a save (migrating it) and returns indented JSON of the meta
// and state, for debugging and bug reports.
func ExportJSON(b []byte) ([]byte, error) {
	s, meta, err := Decode(b)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(struct {
		Meta  Meta         `json:"meta"`
		State *state.State `json:"state"`
	}{meta, s}, "", "  ")
}
