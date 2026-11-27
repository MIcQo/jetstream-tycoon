package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// worldBankSnapshot is the committed copy of the World Bank indicators datagen
// uses. The API has no versioned snapshots and values are revised over time, so a
// checksum on live responses would break builds. The data is CC BY 4.0, which
// allows redistribution with attribution. Refresh with `datagen worldbank`.
const worldBankSnapshot = "tools/datagen/data/worldbank.json"

// Indicator codes.
const (
	indPopulation  = "SP.POP.TOTL"
	indGDPPerCap   = "NY.GDP.PCAP.CD"
	indTourismArrv = "ST.INT.ARVL"
	// tourismMaxYear avoids the pandemic collapse in 2020+ tourism data.
	tourismMaxYear = 2019
)

// Observation is one indicator value for one country.
type Observation struct {
	Value float64 `json:"value"`
	Year  int     `json:"year"`
}

// WorldBank is the snapshot: indicator → ISO2 → latest usable observation.
type WorldBank struct {
	Source     string                            `json:"source"`
	License    string                            `json:"license"`
	Fetched    string                            `json:"fetched"`
	Indicators map[string]map[string]Observation `json:"indicators"`
}

type wbRow struct {
	Country struct {
		ID string `json:"id"` // ISO2 for countries (e.g. XK)
	} `json:"country"`
	ISO3  string   `json:"countryiso3code"`
	Date  string   `json:"date"`
	Value *float64 `json:"value"`
}

// fetchWorldBank downloads the indicators for the given countries from baseURL
// (https://api.worldbank.org/v2) and returns a snapshot.
func fetchWorldBank(baseURL string, countries []Country, now time.Time) (WorldBank, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	wb := WorldBank{
		Source:     "World Bank World Development Indicators (api.worldbank.org/v2)",
		License:    "CC BY 4.0",
		Fetched:    now.UTC().Format("2006-01-02"),
		Indicators: map[string]map[string]Observation{},
	}
	queries := map[string]string{
		indPopulation:  "mrnev=1",
		indGDPPerCap:   "mrnev=1",
		indTourismArrv: "date=2010:" + strconv.Itoa(tourismMaxYear),
	}
	for _, ind := range []string{indPopulation, indGDPPerCap, indTourismArrv} {
		url := fmt.Sprintf("%s/country/all/indicator/%s?format=json&per_page=20000&%s", baseURL, ind, queries[ind])
		rows, err := getWorldBank(client, url)
		if err != nil {
			return WorldBank{}, fmt.Errorf("%s: %w", ind, err)
		}
		wb.Indicators[ind] = latestByCountry(rows, countries)
	}
	return wb, nil
}

func getWorldBank(client *http.Client, url string) ([]wbRow, error) {
	resp, err := client.Get(url) //nolint:noctx // CLI tool with a client timeout
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var page []json.RawMessage // [metadata, rows]
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, err
	}
	if len(page) != 2 {
		return nil, fmt.Errorf("unexpected response shape (%d parts)", len(page))
	}
	var rows []wbRow
	if err := json.Unmarshal(page[1], &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// latestByCountry keeps, per playable country, the most recent non-null value.
// Rows match by ISO2 first, then ISO3 (the World Bank uses XKX for Kosovo).
func latestByCountry(rows []wbRow, countries []Country) map[string]Observation {
	byISO3 := map[string]string{}
	known := map[string]bool{}
	for _, c := range countries {
		byISO3[c.ISO3] = c.ISO2
		known[c.ISO2] = true
	}
	out := map[string]Observation{}
	for _, r := range rows {
		if r.Value == nil {
			continue
		}
		iso := r.Country.ID
		if !known[iso] {
			iso = byISO3[r.ISO3]
		}
		if iso == "" {
			continue // aggregates such as "Africa Eastern and Southern"
		}
		year, err := strconv.Atoi(r.Date)
		if err != nil {
			continue
		}
		if prev, ok := out[iso]; !ok || year > prev.Year {
			out[iso] = Observation{Value: *r.Value, Year: year}
		}
	}
	return out
}

// sortedKeys returns map keys in order (for deterministic iteration).
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
