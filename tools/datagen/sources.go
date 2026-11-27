package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Source is a pinned dataset. The checksum makes builds reproducible and detects
// changed or tampered downloads.
type Source struct {
	File    string // name in the cache directory
	URL     string
	SHA256  string
	License string
}

// sources lists every dataset datagen reads. Bump URL and checksum together.
var sources = []Source{
	{
		File:    "ne_110m_admin_0_countries.geojson",
		URL:     "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/v5.1.2/geojson/ne_110m_admin_0_countries.geojson",
		SHA256:  "6866c877d39cba9c357620878839b336d569f8c662d3cfab4cb1dbe2d39c977f",
		License: "Natural Earth — public domain",
	},
	{
		File:    "ne_50m_admin_0_countries.geojson",
		URL:     "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/v5.1.2/geojson/ne_50m_admin_0_countries.geojson",
		SHA256:  "3e458fc036ad0a66411f2c1e6cac49c5d7bfb81cb1123bc513b22511a2b7fdeb",
		License: "Natural Earth — public domain",
	},
	{
		File:    "ne_10m_populated_places.geojson",
		URL:     "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/v5.1.2/geojson/ne_10m_populated_places.geojson",
		SHA256:  "9b8e3de09048ef00dfc70357dbb9fa324493f214b5e0ae4daf1aa79a8d10116b",
		License: "Natural Earth — public domain",
	},
	{
		// Pinned to a commit of the OurAirports data repository (the website file changes nightly).
		File:    "airports.csv",
		URL:     "https://raw.githubusercontent.com/davidmegginson/ourairports-data/a9426f7711df70678dfb745ceaa3d8891bf98328/airports.csv",
		SHA256:  "b6ae8329691c9fc7b89d840e386fe65a9986e125c808cf885ae73e1ecb18aea8",
		License: "OurAirports — public domain",
	},
	{
		File:    "runways.csv",
		URL:     "https://raw.githubusercontent.com/davidmegginson/ourairports-data/a9426f7711df70678dfb745ceaa3d8891bf98328/runways.csv",
		SHA256:  "5948888ea0b0c55a6b39a31d059f9e4a2fdb8fb1ae0ebd72b43ddd210893b9cd",
		License: "OurAirports — public domain",
	},
}

// fetchAll downloads missing sources and verifies every cached file's checksum.
func fetchAll(dir string, srcs []Source) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	for _, s := range srcs {
		path := filepath.Join(dir, s.File)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := download(client, s.URL, path); err != nil {
				return fmt.Errorf("download %s: %w", s.File, err)
			}
		}
		if err := verify(path, s.SHA256); err != nil {
			return err
		}
	}
	return nil
}

func download(client *http.Client, url, path string) error {
	resp, err := client.Get(url) //nolint:noctx // CLI tool with a client timeout
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func verify(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("%s: checksum %s, want %s (delete the cached file to re-download)", path, got, want)
	}
	return nil
}
