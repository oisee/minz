// mulopt.go — GPU-optimal constant multiplication table.
//
// Loads precomputed optimal multiply-by-constant sequences from
// z80-optimizer's mulopt8_clobber.json. Each entry: A × K → A
// with clobber annotation (which registers are destroyed).
//
// Two clobber classes:
//   B-preserving: {A, F, carry} — 14 solutions (powers of 2 + NEG-based)
//   B-clobbering: {A, B, F, carry} — 150 solutions (B used as temp)
package vir

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type MulOpt struct {
	K        int      `json:"k"`
	Ops      []string `json:"ops"`
	TStates  int      `json:"tstates"`
	Clobber  []string `json:"clobber"`
}

type MulOptTable struct {
	entries map[int]*MulOpt // K → optimal sequence
}

var (
	globalMulOpt     *MulOptTable
	globalMulOptOnce sync.Once
)

func GetMulOptTable() *MulOptTable {
	globalMulOptOnce.Do(func() {
		globalMulOpt = loadMulOptTable()
	})
	return globalMulOpt
}

func loadMulOptTable() *MulOptTable {
	t := &MulOptTable{entries: make(map[int]*MulOpt)}

	// Try multiple paths
	paths := []string{
		os.ExpandEnv("$HOME/dev/z80-optimizer/data/mulopt8_clobber.json"),
	}
	if p := os.Getenv("MULOPT8_PATH"); p != "" {
		paths = []string{p}
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var entries []MulOpt
		if err := json.Unmarshal(data, &entries); err != nil {
			fmt.Fprintf(os.Stderr, "[mulopt] parse error: %v\n", err)
			continue
		}
		for i := range entries {
			t.entries[entries[i].K] = &entries[i]
		}
		fmt.Fprintf(os.Stderr, "[mulopt] loaded %d mul8 entries from %s\n", len(entries), path)
		return t
	}

	return t
}

// Lookup returns the optimal multiply sequence for A × K → A.
// bPreserve=true returns only B-preserving solutions.
func (t *MulOptTable) Lookup(k int, bPreserve bool) *MulOpt {
	if t == nil {
		return nil
	}
	e, ok := t.entries[k]
	if !ok {
		return nil
	}
	if bPreserve {
		for _, c := range e.Clobber {
			if c == "B" {
				return nil // B is clobbered, caller needs B-preserving
			}
		}
	}
	return e
}

func (t *MulOptTable) Size() int {
	if t == nil {
		return 0
	}
	return len(t.entries)
}

// ── Division table ──────────────────────────────────────────────────────────

type DivOpt struct {
	K              int      `json:"k"`
	Ops            []string `json:"ops"`
	Length         int      `json:"length"`
	TStates        int      `json:"tstates"`
	Clobber        []string `json:"clobber"`
	Preamble       []string `json:"preamble"`
	PreambleTStates int     `json:"preamble_tstates"`
}

type DivOptTable struct {
	entries map[int]*DivOpt
}

var (
	globalDivOpt     *DivOptTable
	globalDivOptOnce sync.Once
)

func GetDivOptTable() *DivOptTable {
	globalDivOptOnce.Do(func() {
		globalDivOpt = loadDivOptTable()
	})
	return globalDivOpt
}

func loadDivOptTable() *DivOptTable {
	t := &DivOptTable{entries: make(map[int]*DivOpt)}

	paths := []string{
		os.ExpandEnv("$HOME/dev/z80-optimizer/data/div8_optimal.json"),
	}
	if p := os.Getenv("DIV8_PATH"); p != "" {
		paths = []string{p}
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var entries []DivOpt
		if err := json.Unmarshal(data, &entries); err != nil {
			fmt.Fprintf(os.Stderr, "[divopt] parse error: %v\n", err)
			continue
		}
		for i := range entries {
			t.entries[entries[i].K] = &entries[i]
		}
		fmt.Fprintf(os.Stderr, "[divopt] loaded %d div8 entries from %s\n", len(entries), path)
		return t
	}

	return t
}

func (t *DivOptTable) Lookup(k int) *DivOpt {
	if t == nil {
		return nil
	}
	return t.entries[k]
}

func (t *DivOptTable) Size() int {
	if t == nil {
		return 0
	}
	return len(t.entries)
}
