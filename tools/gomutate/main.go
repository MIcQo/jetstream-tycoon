// Command gomutate runs AST mutation testing on Go packages using `go test -overlay`
// (without modifying working tree files) and enforces a minimum mutation kill score.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Mutant struct {
	File        string
	Line        int
	Col         int
	Description string
	MutatedSrc  []byte
}

var binarySwap = map[token.Token]token.Token{
	token.ADD:        token.SUB,
	token.SUB:        token.ADD,
	token.MUL:        token.QUO,
	token.LSS:        token.LEQ,
	token.LEQ:        token.LSS,
	token.GTR:        token.GEQ,
	token.GEQ:        token.GTR,
	token.EQL:        token.NEQ,
	token.NEQ:        token.EQL,
	token.LAND:       token.LOR,
	token.LOR:        token.LAND,
	token.ADD_ASSIGN: token.SUB_ASSIGN,
	token.SUB_ASSIGN: token.ADD_ASSIGN,
}

func main() {
	threshold := flag.Float64("threshold", 80.0, "minimum required mutation kill percentage (0-100)")
	flag.Parse()

	roots := flag.Args()
	if len(roots) == 0 {
		roots = []string{"sim", "server"}
	}

	var mutants []Mutant
	for _, root := range roots {
		ms, err := CollectMutantsInDir(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gomutate error collecting mutants in %s: %v\n", root, err)
			os.Exit(2)
		}
		mutants = append(mutants, ms...)
	}

	if len(mutants) == 0 {
		fmt.Println("gomutate: no mutable expressions found")
		return
	}

	tmpDir, err := os.MkdirTemp("", "gomutate-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "gomutate temp dir: %v\n", err)
		os.Exit(2)
	}
	defer os.RemoveAll(tmpDir)

	killed := 0
	for i, m := range mutants {
		ok, err := runMutantTest(tmpDir, i, m)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gomutate run error: %v\n", err)
			os.Exit(2)
		}
		if ok {
			killed++
			fmt.Printf("[KILLED]   %s:%d:%d (%s)\n", m.File, m.Line, m.Col, m.Description)
		} else {
			fmt.Printf("[SURVIVED] %s:%d:%d (%s)\n", m.File, m.Line, m.Col, m.Description)
		}
	}

	score := float64(killed) * 100.0 / float64(len(mutants))
	fmt.Printf("Mutation score: %.1f%% (%d / %d killed, threshold %.1f%%)\n", score, killed, len(mutants), *threshold)
	if score < *threshold {
		os.Exit(1)
	}
}

// CollectMutantsInDir discovers all mutants in non-test Go files under root.
func CollectMutantsInDir(root string) ([]Mutant, error) {
	var mutants []Mutant
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		ms, err := GenerateMutants(path, src)
		if err != nil {
			return err
		}
		mutants = append(mutants, ms...)
		return nil
	})
	return mutants, err
}

// GenerateMutants generates AST mutants for a single Go source file.
func GenerateMutants(filename string, src []byte) ([]Mutant, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}

	type site struct {
		pos  token.Pos
		from token.Token
		to   token.Token
		kind string
	}
	var sites []site

	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.BinaryExpr:
			if repl, ok := binarySwap[node.Op]; ok {
				sites = append(sites, site{pos: node.OpPos, from: node.Op, to: repl, kind: "binary"})
			}
		case *ast.AssignStmt:
			if repl, ok := binarySwap[node.Tok]; ok {
				sites = append(sites, site{pos: node.TokPos, from: node.Tok, to: repl, kind: "assign"})
			}
		}
		return true
	})

	var mutants []Mutant
	for idx, s := range sites {
		fsetClone := token.NewFileSet()
		fClone, err := parser.ParseFile(fsetClone, filename, src, 0)
		if err != nil {
			return nil, err
		}
		cur := 0
		ast.Inspect(fClone, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.BinaryExpr:
				if _, ok := binarySwap[node.Op]; ok {
					if cur == idx {
						node.Op = s.to
					}
					cur++
				}
			case *ast.AssignStmt:
				if _, ok := binarySwap[node.Tok]; ok {
					if cur == idx {
						node.Tok = s.to
					}
					cur++
				}
			}
			return true
		})

		var buf bytes.Buffer
		if err := format.Node(&buf, fsetClone, fClone); err != nil {
			return nil, err
		}
		pos := fset.Position(s.pos)
		mutants = append(mutants, Mutant{
			File:        filename,
			Line:        pos.Line,
			Col:         pos.Column,
			Description: fmt.Sprintf("%s -> %s", s.from, s.to),
			MutatedSrc:  buf.Bytes(),
		})
	}
	return mutants, nil
}

func runMutantTest(tmpDir string, idx int, m Mutant) (bool, error) {
	absTarget, err := filepath.Abs(m.File)
	if err != nil {
		return false, err
	}
	mutFile := filepath.Join(tmpDir, fmt.Sprintf("mutant_%d.go", idx))
	if err := os.WriteFile(mutFile, m.MutatedSrc, 0o600); err != nil {
		return false, err
	}
	overlayData, err := json.Marshal(struct {
		Replace map[string]string `json:"Replace"`
	}{
		Replace: map[string]string{absTarget: mutFile},
	})
	if err != nil {
		return false, err
	}
	overlayFile := filepath.Join(tmpDir, fmt.Sprintf("overlay_%d.json", idx))
	if err := os.WriteFile(overlayFile, overlayData, 0o600); err != nil {
		return false, err
	}

	pkgDir := "./" + filepath.ToSlash(filepath.Dir(m.File))
	cmd := exec.Command("go", "test", "-overlay="+overlayFile, pkgDir)
	if err := cmd.Run(); err != nil {
		return true, nil // test failed => mutant killed
	}
	return false, nil // test passed => mutant survived
}
