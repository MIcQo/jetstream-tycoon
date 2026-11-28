// Command contentc loads, validates and compiles the game content (ADR-0004).
//
//	go run ./tools/contentc check   # strict YAML load + validation
//	go run ./tools/contentc build   # also write sim/content/bundle/content.json.gz
//	go run ./tools/contentc verify  # check + fail if the committed bundle is stale (make content-check)
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/MIcQo/jetstream-tycoon/sim/content"
	"github.com/MIcQo/jetstream-tycoon/sim/content/bundle"
)

func main() {
	cmd := "check"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	if err := run(cmd, "content", bundlePath); err != nil {
		fmt.Fprintln(os.Stderr, "contentc:", err)
		os.Exit(1)
	}
}

// bundlePath is where the compiled bundle is embedded from.
const bundlePath = "sim/content/bundle/" + bundle.FileName

func run(cmd, dir, out string) error {
	c, refs, err := Load(dir)
	if err != nil {
		return err
	}
	if err := content.Validate(c, refs); err != nil {
		return err
	}
	fmt.Printf("contentc check: %d planes, %d airport levels, %d seasons, %d holidays OK\n",
		len(c.Planes), len(c.AirportLevels), len(c.Seasons), len(c.Holidays))
	if cmd == "check" {
		return nil
	}
	world, err := LoadWorld(filepath.Join(dir, "world"))
	if err != nil {
		return err
	}
	gz, version, err := Build(c, world)
	if err != nil {
		return err
	}
	switch cmd {
	case "build":
		if err := os.WriteFile(out, gz, 0o644); err != nil {
			return err
		}
		fmt.Printf("contentc build: %s (%d bytes), contentVersion %s\n", out, len(gz), version)
		return nil
	case "verify":
		committed, err := os.ReadFile(out)
		if err != nil {
			return err
		}
		if !bytes.Equal(committed, gz) {
			return fmt.Errorf("%s is stale: run `make content-build` and commit it", out)
		}
		fmt.Printf("contentc verify: %s is up to date (contentVersion %s)\n", out, version)
		return nil
	}
	return fmt.Errorf("unknown command %q (want check, build or verify)", cmd)
}

// LoadWorld reads the simulation subset of content/world. Extra fields in the
// world files (names, labels, sources) are ignored.
func LoadWorld(dir string) (content.World, error) {
	var w content.World
	for _, f := range []struct {
		name string
		into any
	}{
		{"countries.json", &w.Countries},
		{"airports.json", &w.Airports},
		{"cities.json", &w.Cities},
		{"candidates.json", &w.Candidates},
	} {
		if err := readJSON(filepath.Join(dir, f.name), f.into); err != nil {
			return content.World{}, err
		}
	}
	return w, nil
}

// Build compiles content and world into the gzipped bundle. The content version is
// derived from the JSON, and gzip carries no timestamp or name, so the output is
// byte-identical for identical input.
func Build(c content.Content, w content.World) (gz []byte, version string, err error) {
	payload, err := json.Marshal(struct {
		Content content.Content `json:"content"`
		World   content.World   `json:"world"`
	}{c, w})
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(payload)
	version = "c" + hex.EncodeToString(sum[:])[:12]
	data, err := json.Marshal(bundle.Bundle{ContentVersion: version, Content: c, World: w})
	if err != nil {
		return nil, "", err
	}
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, "", err
	}
	if _, err := zw.Write(data); err != nil {
		return nil, "", err
	}
	if err := zw.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), version, nil
}

// Load reads the YAML content and the references it may use from dir.
func Load(dir string) (content.Content, content.Refs, error) {
	var c content.Content
	files := []struct {
		name string
		into any
	}{
		{"planes.yaml", &c.Planes},
		{"airport_levels.yaml", &c.AirportLevels},
		{"balance.yaml", &c.Balance},
		{"seasons.yaml", &c.Seasons},
		{"holidays.yaml", &c.Holidays},
	}
	for _, f := range files {
		if err := readYAML(filepath.Join(dir, f.name), f.into); err != nil {
			return content.Content{}, content.Refs{}, err
		}
	}
	refs, err := loadRefs(dir)
	return c, refs, err
}

// readYAML decodes strictly: unknown or duplicate fields are errors.
func readYAML(path string, into any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.UnmarshalStrict(data, into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func loadRefs(dir string) (content.Refs, error) {
	refs := content.Refs{Countries: map[string]bool{}, Regions: map[string]bool{}, I18nKeys: map[string]bool{}}
	var countries []struct {
		ISO2   string `json:"iso2"`
		Region string `json:"region"`
	}
	if err := readJSON(filepath.Join(dir, "world", "countries.json"), &countries); err != nil {
		return refs, err
	}
	for _, c := range countries {
		refs.Countries[c.ISO2] = true
		if c.Region != "" {
			refs.Regions[c.Region] = true
		}
	}
	var catalog map[string]string
	if err := readJSON(filepath.Join(dir, "i18n", "en.json"), &catalog); err != nil {
		return refs, err
	}
	for k := range catalog {
		refs.I18nKeys[k] = true
	}
	return refs, nil
}

func readJSON(path string, into any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
