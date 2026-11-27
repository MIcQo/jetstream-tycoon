// Command contentc loads and validates the game content (ADR-0004).
//
//	go run ./tools/contentc check   # strict YAML load + validation (make content-check)
//
// The build into content.json (embedded in sim.wasm and the server) follows in M1.8.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/MIcQo/jetstream-tycoon/sim/content"
)

func main() {
	cmd := "check"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	if cmd != "check" {
		fmt.Fprintf(os.Stderr, "contentc: unknown command %q (want check)\n", cmd)
		os.Exit(2)
	}
	c, refs, err := Load("content")
	if err == nil {
		err = content.Validate(c, refs)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "contentc:", err)
		os.Exit(1)
	}
	fmt.Printf("contentc check: %d planes, %d airport levels, %d seasons, %d holidays OK\n",
		len(c.Planes), len(c.AirportLevels), len(c.Seasons), len(c.Holidays))
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
