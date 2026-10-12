package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadmissionRefusesExistingAndAliasedOutputs(t *testing.T) {
	dir := t.TempDir()
	protected := filepath.Join(dir, "input.json")
	if err := os.WriteFile(protected, []byte("retained"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, paths := range [][2]string{{protected, filepath.Join(dir, "evidence")}, {filepath.Join(dir, "output"), protected}, {filepath.Join(dir, "same"), filepath.Join(dir, "same")}} {
		if err := readmitFile("source", protected, paths[0], paths[1], "", 1); err == nil {
			t.Fatal("unsafe outputs accepted")
		}
	}
	data, err := os.ReadFile(protected)
	if err != nil || string(data) != "retained" {
		t.Fatal("input altered", err)
	}
}
