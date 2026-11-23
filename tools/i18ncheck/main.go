// Command i18ncheck validates localization files in web/src/i18n and content/i18n (ADR-0016):
//   - Valid JSON string maps with `area.object.detail` keys and non-empty values
//   - Balanced ICU MessageFormat `{...}` braces
//   - Key parity across all locale files against en.json
//   - Pseudo-locale (en-XA) generation check preserving ICU placeholders
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var keyPattern = regexp.MustCompile(`^[a-z0-9_]+(\.[a-z0-9_]+)+$`)

var accentMap = map[rune]rune{
	'a': 'á', 'e': 'é', 'i': 'í', 'o': 'ó', 'u': 'ú',
	'A': 'Á', 'E': 'É', 'I': 'Í', 'O': 'Ó', 'U': 'Ú',
}

func main() {
	dirs := []string{"web/src/i18n", "content/i18n"}
	var allIssues []string
	for _, dir := range dirs {
		issues, err := ValidateI18nDir(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "i18ncheck error on %s: %v\n", dir, err)
			os.Exit(2)
		}
		allIssues = append(allIssues, issues...)
	}

	if len(allIssues) > 0 {
		for _, issue := range allIssues {
			fmt.Fprintln(os.Stderr, issue)
		}
		os.Exit(1)
	}
	fmt.Println("i18ncheck: all locale catalogs and en-XA pseudo-locale checks passed")
}

// ValidateI18nDir checks en.json and any additional locale JSON files in dir.
func ValidateI18nDir(dir string) ([]string, error) {
	enPath := filepath.Join(dir, "en.json")
	enMap, issues, err := loadCatalog(enPath)
	if err != nil {
		return nil, err
	}
	if len(enMap) == 0 {
		issues = append(issues, fmt.Sprintf("%s: catalog must contain at least one key", enPath))
	}

	for k, v := range enMap {
		pseudo := ToPseudoLocale(v)
		if !hasBalancedBraces(pseudo) {
			issues = append(issues, fmt.Sprintf("%s: key %q failed en-XA pseudo-locale brace check", enPath, k))
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "en.json" || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		locPath := filepath.Join(dir, e.Name())
		locMap, locIssues, err := loadCatalog(locPath)
		if err != nil {
			return nil, err
		}
		issues = append(issues, locIssues...)

		var keys []string
		for k := range enMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if _, ok := locMap[k]; !ok {
				issues = append(issues, fmt.Sprintf("%s: missing key %q present in en.json", locPath, k))
			}
		}
	}

	return issues, nil
}

func loadCatalog(path string) (map[string]string, []string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, []string{fmt.Sprintf("%s: invalid JSON string map: %v", path, err)}, nil
	}

	var issues []string
	for k, v := range m {
		if !keyPattern.MatchString(k) {
			issues = append(issues, fmt.Sprintf("%s: invalid key %q (must match area.object.detail)", path, k))
		}
		if strings.TrimSpace(v) == "" {
			issues = append(issues, fmt.Sprintf("%s: key %q has empty translation", path, k))
		}
		if !hasBalancedBraces(v) {
			issues = append(issues, fmt.Sprintf("%s: key %q has unbalanced ICU braces in %q", path, k, v))
		}
	}
	return m, issues, nil
}

func hasBalancedBraces(s string) bool {
	depth := 0
	for _, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// ToPseudoLocale converts an English ICU string to en-XA pseudo-locale without corrupting `{...}` blocks.
func ToPseudoLocale(s string) string {
	var b strings.Builder
	b.WriteRune('[')
	depth := 0
	for _, r := range s {
		switch r {
		case '{':
			depth++
			b.WriteRune(r)
		case '}':
			if depth > 0 {
				depth--
			}
			b.WriteRune(r)
		default:
			if depth == 0 {
				if acc, ok := accentMap[r]; ok {
					b.WriteRune(acc)
					b.WriteRune(acc)
					continue
				}
			}
			b.WriteRune(r)
		}
	}
	b.WriteString(" ⋯]")
	return b.String()
}
