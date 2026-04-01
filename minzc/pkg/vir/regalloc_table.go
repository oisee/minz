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
	entries      map[string]*RegAllocEntry
	binaryTables []*EnrichedBinaryTable // Z80T binary tables (indexed by enumeration)
	mu           sync.RWMutex
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
// Tries three signature formats in order:
//   1. Original signature (legacy)
//   2. Exhaustive GPU signature (56 entries)
//   3. Enriched signature (shapeHash:opBagHash — O(1) from 37.6M entries)
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
	if !ok {
		// Try enriched signature (shapeHash:opBagHash)
		esig := ComputeEnrichedSignature(ops, desc)
		enrichedKey := fmt.Sprintf("%d:%d", esig.ShapeHash, esig.OpBagHash)
		entry, ok = t.entries[enrichedKey]
		gpuHit = ok // enriched entries also use GPU loc indices
	}
	// Try Z80T binary tables (enumeration index lookup)
	var binaryEntry *EnrichedEntry
	if !ok && len(t.binaryTables) > 0 {
		shape, _, shapeErr := VIRToEnrichedShape(ops, desc)
		if shapeErr == nil {
			idx, idxErr := EnrichedIndexOf(*shape, 6)
			if idxErr == nil {
				for _, bt := range t.binaryTables {
					if e := bt.Lookup(idx); e != nil && !e.Infeasible() {
						binaryEntry = e
						break
					}
				}
			}
		}
	}
	t.mu.RUnlock()

	// Handle binary table hit (different format: []byte assignment)
	if !ok && binaryEntry != nil {
		_, vregList, _ := VIRToEnrichedShape(ops, desc)
		assignment := make(map[int]int)
		for i, loc := range binaryEntry.Assignment {
			if i < len(vregList) {
				assignment[vregList[i]] = gpuLocToZ80(int(loc))
			}
		}
		return assignment, binaryEntry.Cost, true
	}

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

// LookupByKey looks up an entry by its raw signature key.
// Used by MZV host functions that compute the key externally.
func (t *RegAllocTable) LookupByKey(key string) (*RegAllocEntry, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	e, ok := t.entries[key]
	return e, ok
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

	// Try enriched table (z80-optimizer, O(1) lookup for ≤6v)
	for _, p := range []string{
		os.ExpandEnv("$HOME/dev/z80-optimizer/data/enriched_regalloc.jsonl"),
		os.ExpandEnv("$HOME/dev/minz-vir/data/enriched_regalloc.jsonl"),
		"enriched_regalloc.jsonl",
	} {
		if err := t.LoadEnriched(p); err == nil {
			break
		}
	}
	// Also check env var for custom path
	if p := os.Getenv("VIR_ENRICHED_TABLE"); p != "" {
		t.LoadEnriched(p)
	}

	// Try ENRT binary enriched tables. Only load 4v (4MB) by default.
	// 5v (383MB) loaded via VIR_ENRICHED_5V or VIR_ENRICHED_5V_AUTO=1; 6v via VIR_ENRICHED_6V.
	for _, p := range []string{
		os.ExpandEnv("$HOME/dev/minz-vir/data/enriched_4v.z80t"),
		os.ExpandEnv("$HOME/dev/z80-optimizer/data/enriched_4v.z80t"),
	} {
		bt, err := LoadEnrichedBinary(p)
		if err != nil {
			continue
		}
		total, feasible, _ := bt.Stats()
		t.binaryTables = append(t.binaryTables, bt)
		fmt.Fprintf(os.Stderr, "[regalloc] loaded %d enriched binary entries (%d feasible) from %s\n",
			total, feasible, p)
		break
	}
	// Load 5v: explicit path via VIR_ENRICHED_5V, or auto-detect merged_ix_5v.bin with VIR_ENRICHED_5V_AUTO=1.
	// merged_ix_5v.bin is Z80T v2 (includes IXH/IXL locs, +12% coverage vs enriched_5v).
	if p5 := os.Getenv("VIR_ENRICHED_5V"); p5 != "" {
		if bt, err := LoadEnrichedBinary(p5); err == nil {
			total, feasible, _ := bt.Stats()
			t.binaryTables = append(t.binaryTables, bt)
			fmt.Fprintf(os.Stderr, "[regalloc] loaded %d enriched binary entries (%d feasible) from %s\n",
				total, feasible, p5)
		}
	} else if os.Getenv("VIR_ENRICHED_5V_AUTO") == "1" {
		for _, p := range []string{
			os.ExpandEnv("$HOME/dev/minz-vir/data/merged_ix_5v.bin"),
			os.ExpandEnv("$HOME/dev/z80-optimizer/data/merged_ix_5v.bin"),
		} {
			if bt, err := LoadEnrichedBinary(p); err == nil {
				total, feasible, _ := bt.Stats()
				t.binaryTables = append(t.binaryTables, bt)
				fmt.Fprintf(os.Stderr, "[regalloc] loaded %d enriched binary entries (%d feasible) from %s\n",
					total, feasible, p)
				break
			}
		}
	}
	// Optionally load 6v via env var (large file, explicit opt-in).
	if p6 := os.Getenv("VIR_ENRICHED_6V"); p6 != "" {
		if bt, err := LoadEnrichedBinary(p6); err == nil {
			total, feasible, _ := bt.Stats()
			t.binaryTables = append(t.binaryTables, bt)
			fmt.Fprintf(os.Stderr, "[regalloc] loaded %d enriched binary entries (%d feasible) from %s\n",
				total, feasible, p6)
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

// LoadEnriched loads entries from z80-optimizer enriched table format.
// Format: JSON-per-line with shapeHash, opBagHash, assignment, cost, flags.
// Key: "shapeHash:opBagHash" → RegAllocEntry.
func (t *RegAllocTable) LoadEnriched(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	type enrichedEntry struct {
		ShapeHash uint64 `json:"shapeHash"`
		OpBagHash uint64 `json:"opBagHash"`
		NVregs    int    `json:"nVregs"`
		Cost      int    `json:"cost"`
		Assignment []int `json:"assignment"`
		Flags     struct {
			NoAccumulator bool `json:"no_accumulator"`
			NoHL          bool `json:"no_hl"`
			Mul8Safe      bool `json:"mul8_safe"`
			DjnzConflict  bool `json:"djnz_conflict"`
			U16Capable    bool `json:"u16_capable"`
		} `json:"flags"`
	}

	// Try JSON array first, then JSONL
	var entries []enrichedEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		// Try JSONL (one entry per line)
		for _, line := range splitLines(data) {
			if len(line) == 0 || line[0] != '{' {
				continue
			}
			var e enrichedEntry
			if err := json.Unmarshal(line, &e); err == nil {
				entries = append(entries, e)
			}
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	loaded := 0
	for _, e := range entries {
		key := fmt.Sprintf("%d:%d", e.ShapeHash, e.OpBagHash)
		if _, exists := t.entries[key]; !exists {
			t.entries[key] = &RegAllocEntry{
				Signature:  key,
				NVregs:     e.NVregs,
				Cost:       e.Cost,
				Assignment: e.Assignment,
			}
			loaded++
		}
	}
	if loaded > 0 {
		fmt.Fprintf(os.Stderr, "[regalloc] loaded %d enriched entries from %s\n", loaded, path)
	}
	return nil
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
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

// ── Graph Analysis for Decomposition ─────────────────────────────────────────
// Level 0: Cut vertices (Tarjan) → free decomposition, 87% corpus
// Level 2: EXX 2-coloring (BFS bipartite) → dual-bank for 7-12v

// FindCutVertices returns articulation points in the interference graph.
// Cutting at these vertices splits the graph into independent components
// with ZERO boundary move cost — optimal for enriched table lookup.
// Uses Tarjan's algorithm: O(V+E).
func (s InterferenceShape) FindCutVertices() []int {
	n := s.NVregs
	if n <= 1 {
		return nil
	}

	// Build adjacency list
	adj := make([][]int, n)
	for _, e := range s.Edges {
		adj[e[0]] = append(adj[e[0]], e[1])
		adj[e[1]] = append(adj[e[1]], e[0])
	}

	disc := make([]int, n)
	low := make([]int, n)
	parent := make([]int, n)
	isAP := make([]bool, n)
	visited := make([]bool, n)
	for i := range parent {
		parent[i] = -1
		disc[i] = -1
	}

	timer := 0
	var dfs func(u int)
	dfs = func(u int) {
		visited[u] = true
		disc[u] = timer
		low[u] = timer
		timer++
		childCount := 0
		for _, v := range adj[u] {
			if !visited[v] {
				childCount++
				parent[v] = u
				dfs(v)
				if low[v] < low[u] {
					low[u] = low[v]
				}
				// u is AP if: (1) root with 2+ children, or (2) non-root with low[v] >= disc[u]
				if parent[u] == -1 && childCount > 1 {
					isAP[u] = true
				}
				if parent[u] != -1 && low[v] >= disc[u] {
					isAP[u] = true
				}
			} else if v != parent[u] {
				if disc[v] < low[u] {
					low[u] = disc[v]
				}
			}
		}
	}

	for i := 0; i < n; i++ {
		if !visited[i] {
			dfs(i)
		}
	}

	var cuts []int
	for i, ap := range isAP {
		if ap {
			cuts = append(cuts, i)
		}
	}
	return cuts
}

// IsBipartite checks if the interference graph is 2-colorable (bipartite).
// If true, the graph can be split into two banks for EXX scheduling:
// main bank (A-L) and shadow bank (A'-L'), swapped with EXX (4T).
// Returns (true, coloring) where coloring[i] ∈ {0, 1}, or (false, nil).
func (s InterferenceShape) IsBipartite() (bool, []int) {
	n := s.NVregs
	if n == 0 {
		return true, nil
	}

	adj := make([][]int, n)
	for _, e := range s.Edges {
		adj[e[0]] = append(adj[e[0]], e[1])
		adj[e[1]] = append(adj[e[1]], e[0])
	}

	color := make([]int, n)
	for i := range color {
		color[i] = -1 // unvisited
	}

	for start := 0; start < n; start++ {
		if color[start] != -1 {
			continue
		}
		// BFS from start
		color[start] = 0
		queue := []int{start}
		for len(queue) > 0 {
			u := queue[0]
			queue = queue[1:]
			for _, v := range adj[u] {
				if color[v] == -1 {
					color[v] = 1 - color[u]
					queue = append(queue, v)
				} else if color[v] == color[u] {
					return false, nil // odd cycle → not bipartite
				}
			}
		}
	}
	return true, color
}

// DecomposeAtCutVertices splits the graph into components by removing cut vertices.
// Each component + its adjacent cut vertices forms an island ≤ original size.
// Returns list of vertex sets (canonical indices).
func (s InterferenceShape) DecomposeAtCutVertices() [][]int {
	cuts := s.FindCutVertices()
	if len(cuts) == 0 {
		// No cut vertices — return whole graph as single component
		all := make([]int, s.NVregs)
		for i := range all {
			all[i] = i
		}
		return [][]int{all}
	}

	cutSet := make(map[int]bool)
	for _, c := range cuts {
		cutSet[c] = true
	}

	// Build adjacency, find connected components excluding cut vertices
	adj := make([][]int, s.NVregs)
	for _, e := range s.Edges {
		adj[e[0]] = append(adj[e[0]], e[1])
		adj[e[1]] = append(adj[e[1]], e[0])
	}

	visited := make([]bool, s.NVregs)
	var components [][]int

	for i := 0; i < s.NVregs; i++ {
		if visited[i] || cutSet[i] {
			continue
		}
		// BFS from i, skipping cut vertices
		var component []int
		adjCuts := make(map[int]bool) // cut vertices adjacent to this component
		queue := []int{i}
		visited[i] = true
		for len(queue) > 0 {
			u := queue[0]
			queue = queue[1:]
			component = append(component, u)
			for _, v := range adj[u] {
				if cutSet[v] {
					adjCuts[v] = true
					continue
				}
				if !visited[v] {
					visited[v] = true
					queue = append(queue, v)
				}
			}
		}
		// Include adjacent cut vertices in the component
		for c := range adjCuts {
			component = append(component, c)
		}
		sort.Ints(component)
		components = append(components, component)
	}

	return components
}

// ── VIR → EnrichedShape converter ────────────────────────────────────────────
// Converts VIR ops to the EnrichedShape format used by Z80T binary tables.
// locSetIndex per vreg is determined by operation constraints (not hints):
//   ADD/SUB/AND/OR/XOR/CMP dst → must_A (locSets8[0])
//   OpCall dst → must_A (return in A for u8)
//   any GPR src/dst otherwise → any_gpr8 (locSets8[2])
// Intersection of all constraints per vreg → strictest wins.

// VIRToEnrichedShape converts VIR ops into an EnrichedShape for binary table lookup.
// Returns the shape + a vreg mapping (canonical index → original vreg).
// Only works for ≤6 vreg functions with 8-bit operations (first version).
func VIRToEnrichedShape(ops []VIROp, desc *MachineDesc) (*EnrichedShape, []int, error) {
	// Collect vregs in canonical order
	prob := buildProblem(ops, desc)
	vregList := make([]int, 0, len(prob.vregs))
	for v := range prob.vregs {
		vregList = append(vregList, v)
	}
	sort.Ints(vregList)

	nv := len(vregList)
	if nv < 2 || nv > 6 {
		return nil, nil, fmt.Errorf("nVregs=%d out of range [2,6]", nv)
	}

	vregIdx := make(map[int]int) // original vreg → canonical index
	for i, v := range vregList {
		vregIdx[v] = i
	}

	// Compute per-vreg constraints from operations.
	// Start with "any GPR8" and intersect with operation requirements.
	// locSets8: 0=must_A, 1=must_C, 2=any_gpr8, 3=any_gpr8_except_A
	type constraint struct {
		mustA bool // at least one op requires A
		noA   bool // at least one op forbids A (src of tied-dst-A op where dst≠this vreg)
		mustC bool // at least one op requires C
		w16   bool // any 16-bit usage
	}
	constraints := make([]constraint, nv)

	for _, op := range ops {
		switch op.Op {
		case OpAdd, OpSub, OpAnd, OpOr, OpXor, OpNeg:
			// Accumulator ALU: dst must be A
			if op.Dst > 0 {
				if ci, ok := vregIdx[op.Dst]; ok {
					constraints[ci].mustA = true
				}
			}
		case OpCmp, OpCmpImm:
			// CP: src0 must be A (comparison reads A)
			if op.Src[0] > 0 {
				if ci, ok := vregIdx[op.Src[0]]; ok {
					constraints[ci].mustA = true
				}
			}
		case OpAddImm, OpSubImm, OpAndImm, OpOrImm, OpXorImm:
			// Immediate ALU: dst must be A
			if op.Dst > 0 {
				if ci, ok := vregIdx[op.Dst]; ok {
					constraints[ci].mustA = true
				}
			}
		case OpCall:
			// Return value in A (for u8)
			if op.Dst > 0 && op.Width <= 8 {
				if ci, ok := vregIdx[op.Dst]; ok {
					constraints[ci].mustA = true
				}
			}
		}
	}

	// Build widths and locSetIndex
	widths := make([]int, nv)
	locSetIdx := make([]int, nv)
	for i := range widths {
		widths[i] = 8 // default 8-bit (first version)
		if constraints[i].mustA {
			locSetIdx[i] = 0 // must_A
		} else if constraints[i].mustC {
			locSetIdx[i] = 1 // must_C
		} else if constraints[i].noA {
			locSetIdx[i] = 3 // any_gpr8_except_A
		} else {
			locSetIdx[i] = 2 // any_gpr8
		}
	}

	// Build interference bitmask
	intfEdges := make([][2]int, 0)
	for i := range prob.liveness {
		liveVregs := make([]int, 0)
		for v := range prob.liveness[i].live {
			if ci, ok := vregIdx[v]; ok {
				liveVregs = append(liveVregs, ci)
			}
		}
		sort.Ints(liveVregs)
		for a := 0; a < len(liveVregs); a++ {
			for b := a + 1; b < len(liveVregs); b++ {
				intfEdges = append(intfEdges, [2]int{liveVregs[a], liveVregs[b]})
			}
		}
	}
	bitmask := InterferenceBitmask(nv, intfEdges)

	shape := &EnrichedShape{
		NVregs:       nv,
		Widths:       widths,
		LocSetIndex:  locSetIdx,
		Interference: bitmask,
	}
	return shape, vregList, nil
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
