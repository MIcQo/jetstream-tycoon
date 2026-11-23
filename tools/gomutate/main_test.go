package main

import "testing"

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
