package main

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestInspectFileDetectsViolations(t *testing.T) {
	src := `package badsim

import (
	"math"
	"math/rand"
	"time"
)

func bad() {
	_ = time.Now()
	_ = rand.Int()
	_ = math.Pow(2, 3)
	_ = math.Sqrt(4)
	m := map[string]int{"a": 1}
	for k := range m {
		_ = k
	}
	go func() {}()
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "bad.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{Importer: importer.Default(), Error: func(error) {}}
	_, _ = conf.Check("badsim", fset, []*ast.File{f}, info)

	issues := InspectFile(fset, f, info)
	joined := strings.Join(issues, "\n")

	wantSubstrings := []string{
		"time is forbidden in sim/",
		"math/rand is forbidden in sim/",
		"math.Pow is forbidden in sim/",
		"ranging over a map is forbidden in sim/",
		"goroutines (go statement) are forbidden in sim/",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(joined, want) {
			t.Errorf("expected violation containing %q, got:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "math.Sqrt") {
		t.Errorf("math.Sqrt should be allowed, got:\n%s", joined)
	}
}
