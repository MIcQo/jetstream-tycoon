package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGenerateMutants(t *testing.T) {
	src := []byte(`package p

func Add(a, b int) int {
	if a < 0 {
		return a - b
	}
	return a + b
}
`)
	ms, err := GenerateMutants("add.go", src)
	if err != nil {
		t.Fatalf("GenerateMutants error: %v", err)
	}
	if len(ms) != 3 {
		t.Fatalf("len(mutants) = %d, want 3", len(ms))
	}
}

func TestRunAllKeepsOrderAndUsesWorkers(t *testing.T) {
	mutants := make([]Mutant, 20)
	for i := range mutants {
		mutants[i] = Mutant{Line: i}
	}
	var mu sync.Mutex
	active, peak := 0, 0
	var progress []int
	results := RunAll(mutants, 4, func(i int, m Mutant) (bool, error) {
		mu.Lock()
		active++
		if active > peak {
			peak = active
		}
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		if m.Line == 7 {
			return false, errors.New("boom")
		}
		return i%2 == 0, nil
	}, func(done, total int) {
		if total != 20 {
			t.Errorf("total = %d", total)
		}
		progress = append(progress, done)
	})

	if peak < 2 || peak > 4 {
		t.Errorf("peak concurrency = %d, want 2..4", peak)
	}
	for i, r := range results {
		if i == 7 {
			if r.Err == nil {
				t.Error("error for mutant 7 not recorded")
			}
			continue
		}
		if r.Killed != (i%2 == 0) || r.Err != nil {
			t.Errorf("result %d = %+v", i, r)
		}
	}
	if len(progress) != 20 || progress[19] != 20 {
		t.Errorf("progress = %v", progress)
	}
}

func TestRunAllSingleWorkerFallback(t *testing.T) {
	calls := 0
	res := RunAll([]Mutant{{}, {}}, 0, func(int, Mutant) (bool, error) { calls++; return true, nil }, func(int, int) {})
	if calls != 2 || !res[0].Killed || !res[1].Killed {
		t.Fatalf("calls=%d res=%+v", calls, res)
	}
}

func TestRunMutantTestTimeoutCountsAsKilled(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "loop")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, src string) {
		if err := os.WriteFile(filepath.Join(pkg, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module looptest\n\ngo 1.22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	write("loop.go", "package loop\n\nfunc Halve(n int) int {\n\tc := 0\n\tfor n > 0 {\n\t\tn >>= 1\n\t\tc++\n\t}\n\treturn c\n}\n")
	write("loop_test.go", "package loop\n\nimport \"testing\"\n\nfunc TestHalve(t *testing.T) {\n\tif Halve(8) != 4 {\n\t\tt.Fatal(Halve(8))\n\t}\n}\n")

	src, _ := os.ReadFile(filepath.Join(pkg, "loop.go"))
	ms, err := GenerateMutants(filepath.Join(pkg, "loop.go"), src)
	if err != nil {
		t.Fatal(err)
	}
	var loopMutant *Mutant
	for i := range ms {
		if strings.Contains(ms[i].Description, "> -> >=") {
			loopMutant = &ms[i]
		}
	}
	if loopMutant == nil {
		t.Fatalf("no > -> >= mutant among %v", ms)
	}

	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// The unmutated package passes, so a kill below can only come from the mutant.
	orig := Mutant{File: filepath.Join("loop", "loop.go"), MutatedSrc: src}
	if killed, err := runMutantTest(t.TempDir(), 0, orig, 30*time.Second); err != nil || killed {
		t.Fatalf("original package: killed=%v err=%v, want passing tests", killed, err)
	}

	loopMutant.File = filepath.Join("loop", "loop.go")
	start := time.Now()
	killed, err := runMutantTest(t.TempDir(), 0, *loopMutant, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !killed {
		t.Fatal("infinite-loop mutant was not counted as killed")
	}
	// It must have been stopped by the 2 s test timeout, not by a build error.
	if d := time.Since(start); d < 2*time.Second || d > 40*time.Second {
		t.Fatalf("mutant run took %s, want just over the 2s timeout", d)
	}
}
