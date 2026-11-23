// Command migrationcheck validates Postgres and ClickHouse golang-migrate files (ADR-0007, ADR-0011):
//   - Sequential 4-digit version numbers starting at 0001 (0001_<name>.up.sql / .down.sql)
//   - Every migration has both an .up.sql and a matching .down.sql file
//   - Non-empty SQL statements (after stripping comments)
//   - ClickHouse migrations do not use Nullable(...) columns
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var filePattern = regexp.MustCompile(`^([0-9]{4})_([a-z0-9_]+)\.(up|down)\.sql$`)
var nullablePattern = regexp.MustCompile(`(?i)\bNullable\s*\(`)

func main() {
	dirs := []struct {
		path         string
		isClickHouse bool
	}{
		{path: "migrations/postgres", isClickHouse: false},
		{path: "migrations/clickhouse", isClickHouse: true},
	}

	var allIssues []string
	for _, d := range dirs {
		issues, err := ValidateDir(d.path, d.isClickHouse)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrationcheck error on %s: %v\n", d.path, err)
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
	fmt.Println("migrationcheck: postgres and clickhouse migrations valid")
}

type pair struct {
	version int
	name    string
	hasUp   bool
	hasDown bool
}

// ValidateDir checks all migration files in dir.
func ValidateDir(dir string, isClickHouse bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var issues []string
	byVersion := make(map[int]*pair)

	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		fullPath := filepath.Join(dir, e.Name())
		m := filePattern.FindStringSubmatch(e.Name())
		if m == nil {
			issues = append(issues, fmt.Sprintf("%s: filename must match NNNN_<name>.(up|down).sql", fullPath))
			continue
		}

		ver, _ := strconv.Atoi(m[1])
		name := m[2]
		direction := m[3]

		p, exists := byVersion[ver]
		if !exists {
			p = &pair{version: ver, name: name}
			byVersion[ver] = p
		} else if p.name != name {
			issues = append(issues, fmt.Sprintf("%s: mismatched migration name %q for version %04d (expected %q)", fullPath, name, ver, p.name))
		}

		if direction == "up" {
			if p.hasUp {
				issues = append(issues, fmt.Sprintf("%s: duplicate up migration for version %04d", fullPath, ver))
			}
			p.hasUp = true
		} else {
			if p.hasDown {
				issues = append(issues, fmt.Sprintf("%s: duplicate down migration for version %04d", fullPath, ver))
			}
			p.hasDown = true
		}

		raw, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		sqlText := stripSQLComments(string(raw))
		if strings.TrimSpace(sqlText) == "" {
			issues = append(issues, fmt.Sprintf("%s: migration file contains no SQL statements", fullPath))
		}
		if isClickHouse && nullablePattern.MatchString(sqlText) {
			issues = append(issues, fmt.Sprintf("%s: ClickHouse migrations must not use Nullable(...) (ADR-0011)", fullPath))
		}
	}

	if len(byVersion) == 0 {
		issues = append(issues, fmt.Sprintf("%s: no migrations found", dir))
		return issues, nil
	}

	var versions []int
	for v := range byVersion {
		versions = append(versions, v)
	}
	sort.Ints(versions)

	for idx, v := range versions {
		expected := idx + 1
		if v != expected {
			issues = append(issues, fmt.Sprintf("%s: non-contiguous migration version %04d (expected %04d)", dir, v, expected))
		}
		p := byVersion[v]
		if !p.hasUp {
			issues = append(issues, fmt.Sprintf("%s: version %04d_%s is missing .up.sql", dir, v, p.name))
		}
		if !p.hasDown {
			issues = append(issues, fmt.Sprintf("%s: version %04d_%s is missing .down.sql", dir, v, p.name))
		}
	}

	return issues, nil
}

func stripSQLComments(src string) string {
	var out strings.Builder
	lines := strings.Split(src, "\n")
	for _, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}
