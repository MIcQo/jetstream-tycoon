// Package bundle embeds the compiled game content (content.json.gz, built by
// `go run ./tools/contentc build`) into sim.wasm and the server (ADR-0004).
// The file is committed so builds stay hermetic; `make content-check` fails when
// it is stale.
package bundle

import (
	"bytes"
	"compress/gzip"
	_ "embed" // for the embedded content file
	"fmt"
	"io"
	"sync"

	"github.com/MIcQo/jetstream-tycoon/sim/content"
)

// FileName is the embedded artifact, relative to this package.
const FileName = "content.json.gz"

//go:embed content.json.gz
var compressed []byte

// Bundle is the compiled content plus the world data the simulation uses.
type Bundle struct {
	// ContentVersion identifies this content: "c" + 12 hex digits of the SHA-256 of
	// the content and world JSON. It is stored in saves and analytics.
	ContentVersion string          `json:"contentVersion"`
	Content        content.Content `json:"content"`
	World          content.World   `json:"world"`
}

var (
	once   sync.Once
	loaded Bundle
	errLd  error
)

// Load decodes the embedded bundle once and returns it.
func Load() (Bundle, error) {
	once.Do(func() { loaded, errLd = Decode(compressed) })
	return loaded, errLd
}

// Decode decompresses and strictly decodes a bundle.
func Decode(gz []byte) (Bundle, error) {
	r, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return Bundle{}, fmt.Errorf("bundle: %w", err)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return Bundle{}, fmt.Errorf("bundle: %w", err)
	}
	var b Bundle
	if err := content.DecodeInto(raw, &b); err != nil {
		return Bundle{}, fmt.Errorf("bundle: %w", err)
	}
	if b.ContentVersion == "" {
		return Bundle{}, fmt.Errorf("bundle: missing contentVersion")
	}
	return b, nil
}
