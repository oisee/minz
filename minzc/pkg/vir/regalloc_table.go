// regalloc_table.go — Precomputed register allocation lookup table.
//
// The GPU brute-force solver runs OFFLINE and produces a table of
// (constraint signature → optimal assignment). At compile time, the
// compiler hashes the function's constraints and looks up the answer.
//
// Signature format:
//   "{nVregs}v_{nOps}o_{patternHash}_{interferenceHash}"
//
// The pattern hash encodes: for each op, which patterns are available
// and their dst/src constraints. The interference hash encodes which
// vreg pairs conflict.
//
// This gives O(1) compile-time register allocation for functions that
// match a precomputed entry — no Z3, no GPU, instant and provably optimal.
package vir

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
)

// RegAllocEntry is one precomputed optimal assignment.
type RegAllocEntry struct {
	Signature  string `json:"sig"`
	NVregs     int    `json:"nVregs"`
	NOps       int    `json:"nOps"`
	Cost       int    `json:"cost"`
	Assignment []int  `json:"assignment"` // dense vreg index → physical loc
}

// RegAllocTable holds precomputed assignments indexed by signature.
type RegAllocTable struct {
	entries map[string]*RegAllocEntry
	mu      sync.RWMutex
}

var (
	globalTable     *RegAllocTable
	globalTableOnce sync.Once
)

// GetRegAllocTable returns the global precomputed table, loading it if needed.
func GetRegAllocTable() *RegAllocTable {
	globalTableOnce.Do(func() {
		globalTable = &RegAllocTable{entries: make(map[string]*RegAllocEntry)}
		globalTable.loadDefaults()
	})
	return globalTable
}

// Lookup checks if a precomputed assignment exists for the given VIR ops.
// Returns (assignment vreg→phys, cost, true) if found, (nil, 0, false) otherwise.
func (t *RegAllocTable) Lookup(ops []VIROp, desc *MachineDesc) (map[int]int, int, bool) {
	sig := ComputeSignature(ops, desc)

	t.mu.RLock()
	entry, ok := t.entries[sig]
	t.mu.RUnlock()

	if !ok {
		return nil, 0, false
	}

	// Map dense assignment back to original vregs
	prob := buildProblem(ops, desc)
	vregList := make([]int, 0, len(prob.vregs))
	for v := range prob.vregs {
		vregList = append(vregList, v)
	}
	sort.Ints(vregList)

	assignment := make(map[int]int)
	for i, loc := range entry.Assignment {
		if i < len(vregList) {
			assignment[vregList[i]] = loc
		}
	}

	return assignment, entry.Cost, true
}

// Add inserts a new entry into the table.
func (t *RegAllocTable) Add(sig string, entry *RegAllocEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Only keep the best (lowest cost) entry per signature
	if existing, ok := t.entries[sig]; ok && existing.Cost <= entry.Cost {
		return
	}
	t.entries[sig] = entry
}

// Size returns the number of entries.
func (t *RegAllocTable) Size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.entries)
}

// Save writes the table to a JSON file.
func (t *RegAllocTable) Save(path string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var entries []*RegAllocEntry
	for _, e := range t.entries {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Signature < entries[j].Signature
	})

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Load reads entries from a JSON file and merges them.
func (t *RegAllocTable) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var entries []*RegAllocEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	for _, e := range entries {
		t.Add(e.Signature, e)
	}
	return nil
}

// loadDefaults loads built-in entries (embedded at compile time from GPU runs).
func (t *RegAllocTable) loadDefaults() {
	// Try loading from well-known paths
	for _, p := range []string{
		os.ExpandEnv("$HOME/dev/z80-optimizer/regalloc_table.json"),
		"regalloc_table.json",
	} {
		if err := t.Load(p); err == nil {
			return
		}
	}
}

// ── Signature computation ───────────────────────────────────────────────────

// ComputeSignature creates a deterministic hash of a function's regalloc
// constraints. Two functions with the same signature will have the same
// optimal assignment.
//
// The signature captures:
//   - Number of vregs and ops
//   - Per-op: available patterns (dst/src loc sets, tied, cost)
//   - Interference pairs
//   - NOT: vreg IDs, function name, immediates, symbols
//
// This means different functions with isomorphic constraint graphs share
// the same entry — maximizing table reuse.
func ComputeSignature(ops []VIROp, desc *MachineDesc) string {
	prob := buildProblem(ops, desc)

	// Collect vregs and map to dense indices
	vregList := make([]int, 0, len(prob.vregs))
	for v := range prob.vregs {
		vregList = append(vregList, v)
	}
	sort.Ints(vregList)
	vregIdx := make(map[int]int)
	for i, v := range vregList {
		vregIdx[v] = i
	}

	h := sha256.New()

	// Encode nVregs, nOps
	fmt.Fprintf(h, "V%d:O%d:", len(vregList), len(ops))

	// Encode per-op pattern constraints (using dense vreg indices)
	for i, op := range ops {
		dst := -1
		if op.Dst > 0 {
			if idx, ok := vregIdx[op.Dst]; ok {
				dst = idx
			}
		}
		src0, src1 := -1, -1
		if op.Src[0] > 0 {
			if idx, ok := vregIdx[op.Src[0]]; ok {
				src0 = idx
			}
		}
		if op.Src[1] > 0 {
			if idx, ok := vregIdx[op.Src[1]]; ok {
				src1 = idx
			}
		}

		fmt.Fprintf(h, "op%d[d%d,s%d,%d]:", i, dst, src0, src1)

		// Pattern constraints
		for _, pi := range prob.patterns[i] {
			pat := &desc.Patterns[pi]
			fmt.Fprintf(h, "p%x,%x,%x,%d,%v;",
				pat.DstLocs, pat.SrcLocs[0], pat.SrcLocs[1],
				pat.Cost, pat.TiedDstSrc)
		}
	}

	// Encode interference (sorted pairs of dense indices)
	type pair struct{ a, b int }
	var pairs []pair
	emitted := make(map[pair]bool)
	for _, l := range prob.liveness {
		ids := make([]int, 0, len(l.live))
		for v := range l.live {
			if idx, ok := vregIdx[v]; ok {
				ids = append(ids, idx)
			}
		}
		sort.Ints(ids)
		for a := 0; a < len(ids); a++ {
			for b := a + 1; b < len(ids); b++ {
				p := pair{ids[a], ids[b]}
				if !emitted[p] {
					pairs = append(pairs, p)
					emitted[p] = true
				}
			}
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].a != pairs[j].a {
			return pairs[i].a < pairs[j].a
		}
		return pairs[i].b < pairs[j].b
	})
	for _, p := range pairs {
		fmt.Fprintf(h, "I%d,%d;", p.a, p.b)
	}

	sum := h.Sum(nil)
	return fmt.Sprintf("%dv_%do_%x", len(vregList), len(ops), sum[:8])
}
