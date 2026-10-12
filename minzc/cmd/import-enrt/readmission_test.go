package main

import (
	"github.com/minz/minzc/pkg/vir"
	"os"
	"path/filepath"
	"strings"
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

func TestReadmissionRawDuplicateBudget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.nanz")
	pool := filepath.Join(dir, "pool.json")
	output, evidence := filepath.Join(dir, "out.json"), filepath.Join(dir, "evidence.json")
	if err := os.WriteFile(source, []byte("fun main()->u8{return 1}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pool, []byte(`[{"sig":"duplicate"},{"sig":"duplicate"}]`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := readmitFile(source, pool, output, evidence, "", 1); err == nil || !strings.Contains(err.Error(), "entry limit") {
		t.Fatal("raw duplicate budget bypass", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("budget failure published output", err)
	}
}

func TestReadmissionBoundsInputBytes(t *testing.T) {
	dir := t.TempDir()
	source, pool, output, evidence := filepath.Join(dir, "source"), filepath.Join(dir, "pool"), filepath.Join(dir, "out"), filepath.Join(dir, "evidence")
	if err := os.WriteFile(source, []byte("fun main()->u8{return 1}"), 0644); err != nil {
		t.Fatal(err)
	}
	// A sparse file exercises a large input without allocating its full size.
	f, err := os.Create(pool)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(64 * vir.MaxLegacyCandidateBytes); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := readmitFile(source, pool, output, evidence, "", 1); err == nil || !strings.Contains(err.Error(), "byte limit") {
		t.Fatal("oversized input admitted", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("oversize input published product", err)
	}
}
