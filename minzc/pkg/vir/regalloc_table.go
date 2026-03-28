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

// Init initializes an empty table (for external construction).
func (t *RegAllocTable) Init() {
	t.entries = make(map[string]*RegAllocEntry)
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
// Tries both the original signature format and the exhaustive GPU format.
// Returns (assignment vreg→phys, cost, true) if found, (nil, 0, false) otherwise.
func (t *RegAllocTable) Lookup(ops []VIROp, desc *MachineDesc) (map[int]int, int, bool) {
	sig := ComputeSignature(ops, desc)

	t.mu.RLock()
	entry, ok := t.entries[sig]
	gpuHit := false
	if !ok {
		// Try exhaustive table signature (GPU format)
		gpuSig := computeGPUSignature(ops, desc)
		entry, ok = t.entries[gpuSig]
		gpuHit = ok
	}
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
			if gpuHit {
				loc = gpuLocToZ80(loc)
			}
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

// loadDefaults loads built-in entries from well-known paths.
func (t *RegAllocTable) loadDefaults() {
	// Try original format (array of entries with sig field)
	for _, p := range []string{
		os.ExpandEnv("$HOME/dev/z80-optimizer/regalloc_table.json"),
		"regalloc_table.json",
	} {
		if err := t.Load(p); err == nil {
			break
		}
	}

	// Try exhaustive table format (map[hash] → {cost, assignment, nVregs, nOps})
	for _, p := range []string{
		os.ExpandEnv("$HOME/dev/minz-vir/minzc/pkg/vir/regalloc_exhaustive.json"),
		"regalloc_exhaustive.json",
	} {
		if err := t.LoadExhaustive(p); err == nil {
			break
		}
	}
}

// LoadExhaustive loads entries from the exhaustive enumeration format.
// Format: {"hash16": {"cost": N, "assignment": [...], "nVregs": N, "nOps": N}, ...}
func (t *RegAllocTable) LoadExhaustive(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw map[string]struct {
		Cost       int   `json:"cost"`
		Assignment []int `json:"assignment"`
		NVregs     int   `json:"nVregs"`
		NOps       int   `json:"nOps"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for sig, e := range raw {
		if _, exists := t.entries[sig]; !exists {
			t.entries[sig] = &RegAllocEntry{
				Signature:  sig,
				NVregs:     e.NVregs,
				NOps:       e.NOps,
				Cost:       e.Cost,
				Assignment: e.Assignment,
			}
		}
	}
	fmt.Fprintf(os.Stderr, "[regalloc] loaded %d exhaustive entries from %s\n", len(raw), path)
	return nil
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

// gpuLocToZ80 maps GPU regalloc location indices to Z80 descriptor indices.
// GPU: A=0..L=6, BC=7, DE=8, HL=9, IXH=10, IXL=11, IYH=12, IYL=13, MEM0=14
// Z80: A=0..L=6, BC=7, DE=8, HL=9, SP=10, IX=11, IY=12, F=13, IXH=14..IYL=17
var gpuToZ80Loc = [15]int{
	0, 1, 2, 3, 4, 5, 6, // A-L: same
	7, 8, 9,              // BC, DE, HL: same
	14, 15, 16, 17,       // IXH, IXL, IYH, IYL: shifted
	-1,                   // MEM0: no direct Z80 equivalent (spill)
}

func gpuLocToZ80(gpuLoc int) int {
	if gpuLoc >= 0 && gpuLoc < len(gpuToZ80Loc) {
		return gpuToZ80Loc[gpuLoc]
	}
	return gpuLoc // passthrough for unknown
}

// computeGPUSignature produces a signature matching the exhaustive enumerator's
// funcDescFP format. Uses GPU pattern desc ([]int loc lists) instead of LocSet hex.
func computeGPUSignature(ops []VIROp, desc *MachineDesc) string {
	gf, _, err := BuildGPUDesc(ops, desc, SolverOptions{})
	if err != nil {
		return ""
	}

	h := sha256.New()
	fmt.Fprintf(h, "V%d:", gf.NVregs)
	for _, op := range gf.Ops {
		fmt.Fprintf(h, "O%d,%d,%d:", op.Dst, op.Src0, op.Src1)
		for _, p := range op.Patterns {
			fmt.Fprintf(h, "P%v,%v,%v,%d,%v;", p.DstLocs, p.SrcLocs0, p.SrcLocs1, p.Cost, p.TiedDstSrc)
		}
	}
	for _, i := range gf.Interference {
		fmt.Fprintf(h, "I%d,%d;", i[0], i[1])
	}

	return fmt.Sprintf("%x", h.Sum(nil)[:8])
}

// ── O(1) Enriched Table Integration ──────────────────────────────────────────
// These functions compute the (interference_shape, op_bag) signature that
// indexes into z80-optimizer's precomputed enriched tables (37.6M entries).
//
// Architecture: hash(shape, op_bag) → enriched_Nv.enr → {assignment, flags, costs}
// See: z80-optimizer/data/ENRICHED_TABLES.md for binary format spec.

// OpBag is a multiset of abstract VIR operations. Order doesn't matter —
// only counts. This is the "operation bag" half of the enriched table key.
type OpBag struct {
	Add   int `json:"add"`   // OpAdd + OpAddImm
	Sub   int `json:"sub"`   // OpSub + OpSubImm
	Mul   int `json:"mul"`   // OpMul (runtime call)
	Cmp   int `json:"cmp"`   // OpCmp + OpCmpImm
	Logic int `json:"logic"` // OpAnd + OpOr + OpXor + *Imm variants
	Shift int `json:"shift"` // OpShl + OpShr
	Load  int `json:"load"`  // OpLoad + OpLoadGlobal + OpLoad16LE
	Store int `json:"store"` // OpStore + OpStoreGlobal
	Call  int `json:"call"`  // OpCall
	Move  int `json:"move"`  // OpMove
	Const int `json:"const"` // OpConst
	Neg   int `json:"neg"`   // OpNeg
}

// ComputeOpBag counts abstract VIR operations for the enriched table key.
func ComputeOpBag(ops []VIROp) OpBag {
	var bag OpBag
	for _, op := range ops {
		switch op.Op {
		case OpAdd, OpAddImm:
			bag.Add++
		case OpSub, OpSubImm:
			bag.Sub++
		case OpMul:
			bag.Mul++
		case OpCmp, OpCmpImm:
			bag.Cmp++
		case OpAnd, OpOr, OpXor, OpAndImm, OpOrImm, OpXorImm:
			bag.Logic++
		case OpShl, OpShr:
			bag.Shift++
		case OpLoad, OpLoadGlobal, OpLoad16LE:
			bag.Load++
		case OpStore, OpStoreGlobal:
			bag.Store++
		case OpCall:
			bag.Call++
		case OpMove:
			bag.Move++
		case OpConst:
			bag.Const++
		case OpNeg:
			bag.Neg++
		}
	}
	return bag
}

// opBagHash produces a compact hash of the operation bag for table lookup.
func (b OpBag) Hash() uint64 {
	// Pack counts into bytes (each 0-255), hash the result.
	// 12 categories × 8 bits = 96 bits → SHA256 → truncate to 64 bits.
	data := []byte{
		byte(b.Add), byte(b.Sub), byte(b.Mul), byte(b.Cmp),
		byte(b.Logic), byte(b.Shift), byte(b.Load), byte(b.Store),
		byte(b.Call), byte(b.Move), byte(b.Const), byte(b.Neg),
	}
	h := sha256.Sum256(data)
	var v uint64
	for i := 0; i < 8; i++ {
		v = (v << 8) | uint64(h[i])
	}
	return v
}

// InterferenceShape is a canonical representation of the interference graph.
// Two functions with the same shape and op_bag have the same optimal assignment.
type InterferenceShape struct {
	NVregs int      `json:"nVregs"`
	Edges  [][2]int `json:"edges"` // sorted pairs (lo, hi)
}

// ComputeInterferenceShape extracts the canonical interference graph from VIR ops.
func ComputeInterferenceShape(ops []VIROp, desc *MachineDesc) InterferenceShape {
	prob := buildProblem(ops, desc)

	// Collect unique interference edges
	edgeSet := make(map[[2]int]bool)
	for i := range prob.liveness {
		vregs := make([]int, 0, len(prob.liveness[i].live))
		for v := range prob.liveness[i].live {
			vregs = append(vregs, v)
		}
		sort.Ints(vregs)
		for a := 0; a < len(vregs); a++ {
			for b := a + 1; b < len(vregs); b++ {
				edgeSet[[2]int{vregs[a], vregs[b]}] = true
			}
		}
	}

	// Canonicalize: renumber vregs to 0..N-1 in order of first appearance
	vregOrder := make(map[int]int) // original vreg → canonical index
	for _, op := range ops {
		if op.Dst > 0 {
			if _, ok := vregOrder[op.Dst]; !ok {
				vregOrder[op.Dst] = len(vregOrder)
			}
		}
		for _, s := range op.Src {
			if s > 0 {
				if _, ok := vregOrder[s]; !ok {
					vregOrder[s] = len(vregOrder)
				}
			}
		}
	}

	var edges [][2]int
	for e := range edgeSet {
		ca, cb := vregOrder[e[0]], vregOrder[e[1]]
		if ca > cb {
			ca, cb = cb, ca
		}
		edges = append(edges, [2]int{ca, cb})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i][0] != edges[j][0] {
			return edges[i][0] < edges[j][0]
		}
		return edges[i][1] < edges[j][1]
	})

	return InterferenceShape{
		NVregs: len(vregOrder),
		Edges:  edges,
	}
}

// Hash produces a compact hash of the interference shape for table lookup.
func (s InterferenceShape) Hash() uint64 {
	h := sha256.New()
	fmt.Fprintf(h, "V%d:", s.NVregs)
	for _, e := range s.Edges {
		fmt.Fprintf(h, "%d-%d;", e[0], e[1])
	}
	sum := h.Sum(nil)
	var v uint64
	for i := 0; i < 8; i++ {
		v = (v << 8) | uint64(sum[i])
	}
	return v
}

// EnrichedSignature combines shape + op_bag into a single lookup key.
type EnrichedSignature struct {
	ShapeHash uint64 `json:"shapeHash"`
	OpBagHash uint64 `json:"opBagHash"`
	NVregs    int    `json:"nVregs"`
}

// ComputeEnrichedSignature computes the full enriched table lookup key.
func ComputeEnrichedSignature(ops []VIROp, desc *MachineDesc) EnrichedSignature {
	shape := ComputeInterferenceShape(ops, desc)
	bag := ComputeOpBag(ops)
	return EnrichedSignature{
		ShapeHash: shape.Hash(),
		OpBagHash: bag.Hash(),
		NVregs:    shape.NVregs,
	}
}
