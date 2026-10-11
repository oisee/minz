package vir

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80spec"
)

const checkedTableSchema = "vir-local-direct-v1"
const checkedTableCostModel = "expanded-pattern-tstates-v1"

// CheckedTableKey binds a local straight-line problem to its complete machine
// descriptor and cost model. Unlike historical shape keys, this conservative
// schema includes immediates, symbols, hints, scratch declarations and templates.
// Vreg numbers are normalized; ABI parameter constraints use the same mapping.
func CheckedTableKey(ops []VIROp, desc *MachineDesc, opts SolverOptions) (string, error) {
	if len(opts.FuncParamLocs) != 0 {
		return "", fmt.Errorf("resolve function ABI before checked lookup")
	}
	if desc == nil || desc.Name != "z80" {
		return "", fmt.Errorf("checked table requires Z80 descriptor")
	}
	canonical := append([]VIROp(nil), ops...)
	ids := make(map[int]int)
	for _, op := range ops {
		for _, v := range append(append([]int{op.Dst}, op.Src[:]...), append(append([]int(nil), op.AsmIns...), op.AsmOuts...)...) {
			if v > 0 {
				ids[v] = 0
			}
		}
	}
	var sorted []int
	for v := range ids {
		sorted = append(sorted, v)
	}
	sort.Ints(sorted)
	for i, v := range sorted {
		ids[v] = i + 1
	}
	norm := func(v int) int {
		if v > 0 {
			return ids[v]
		}
		return v
	}
	for i := range canonical {
		canonical[i].Dst = norm(canonical[i].Dst)
		for j, v := range canonical[i].Src {
			canonical[i].Src[j] = norm(v)
		}
		canonical[i].AsmIns = append([]int(nil), canonical[i].AsmIns...)
		canonical[i].AsmOuts = append([]int(nil), canonical[i].AsmOuts...)
		for j, v := range canonical[i].AsmIns {
			canonical[i].AsmIns[j] = norm(v)
		}
		for j, v := range canonical[i].AsmOuts {
			canonical[i].AsmOuts[j] = norm(v)
		}
	}
	params := make(map[int]int)
	for v, loc := range opts.ParamLocs {
		if _, ok := ids[v]; !ok {
			return "", fmt.Errorf("unknown ABI vreg %d", v)
		}
		params[ids[v]] = loc
	}
	// Include the canonical physical overlap relation independently of descriptor
	// aliases; changing z80spec must invalidate a cached allocation as well.
	var overlaps [][2]int
	for i, a := range desc.Locs {
		for j, b := range desc.Locs {
			if i < j && z80spec.Overlaps(a.Name, b.Name) {
				overlaps = append(overlaps, [2]int{i, j})
			}
		}
	}
	payload := struct {
		Schema, CostModel string
		Machine           *MachineDesc
		Ops               []VIROp
		Params            map[int]int
		Overlaps          [][2]int
		OptSize           bool
	}{checkedTableSchema, checkedTableCostModel, desc, canonical, params, overlaps, opts.OptSize}
	// pairAlias fields are private: encode their values explicitly below.
	pairs := make([][3]int, len(pairAliases))
	for i, p := range pairAliases {
		pairs[i] = [3]int{p.pair, p.hi, p.lo}
	}
	data, err := json.Marshal(struct {
		Problem any
		Pairs   [][3]int
	}{payload, pairs})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return checkedTableSchema + ":" + fmt.Sprintf("%x", sum), nil
}

func checkedLocalCost(ops []VIROp, assignment map[int]int, desc *MachineDesc, opts SolverOptions) (int, error) {
	// This certificate covers direct local emission, not opaque asm/calls or CFG.
	for v, loc := range assignment {
		if loc < 0 || loc >= len(desc.Locs) {
			return 0, fmt.Errorf("invalid location for vreg %d", v)
		}
		if desc.NonAllocatable.Has(loc) && desc.Locs[loc].Kind != LocFlag {
			return 0, fmt.Errorf("nonallocatable storage %s", desc.Locs[loc].Name)
		}
	}
	for _, op := range ops {
		if op.Op == OpRet && op.Src[0] > 0 {
			required := "A"
			if op.Width > 8 {
				required = "HL"
			}
			if op.Width == 1 {
				required = "F"
			}
			if assignment[op.Src[0]] != desc.LocByName(required) {
				return 0, fmt.Errorf("return value must be in %s", required)
			}
		}
	}
	for _, op := range ops {
		if op.Op == OpAsmBlock || op.Op == OpCall {
			return 0, fmt.Errorf("opaque operation cannot be certified locally")
		}
	}
	vf := &Func{Name: "checked_local", Blocks: []Block{{Ops: ops}}}
	if _, err := emitFromTable(&mir2.Func{Name: vf.Name}, vf, ops, assignment, desc, opts); err != nil {
		return 0, err
	}
	cost := 0
	for _, op := range ops {
		dst, src := -1, [2]int{-1, -1}
		if op.Dst > 0 {
			dst = assignment[op.Dst]
		}
		if op.Op == OpCmp || op.Op == OpCmpImm || op.Op == OpBitTest {
			dst = desc.LocByName("F")
		}
		for j, v := range op.Src {
			if v > 0 {
				src[j] = assignment[v]
			}
		}
		pat := findBestPattern(op, dst, src, desc)
		if pat == nil {
			return 0, fmt.Errorf("no direct pattern")
		}
		pir := PIROp{Pat: pat, DstPhys: dst, SrcPhys: src, Imm: op.Imm, Sym: op.Sym}
		if isSelfMove(pir.Emit(desc)) {
			continue
		}
		cost += pat.Cost
		for _, rule := range desc.Rules {
			if rule.OpMask != 0 && rule.OpMask != op.Op {
				continue
			}
			if dst >= 0 && rule.DstSet.Has(dst) && (src[0] >= 0 && rule.SrcSet.Has(src[0]) || src[1] >= 0 && rule.SrcSet.Has(src[1])) {
				if rule.Cost >= MaxCost {
					return 0, fmt.Errorf("forbidden rule %s", rule.Name)
				}
				cost += rule.Cost
			}
		}
	}
	return cost, nil
}

// AddChecked creates a local certificate from current admission, not a caller's
// claimed cost. Explicit moves (including PUSH/POP pair moves) are included.
// It makes no global optimality claim: only the best admitted entry is retained.
func (t *RegAllocTable) AddChecked(ops []VIROp, assignment map[int]int, desc *MachineDesc, opts SolverOptions) error {
	key, err := CheckedTableKey(ops, desc, opts)
	if err != nil {
		return err
	}
	cost, err := checkedLocalCost(ops, assignment, desc, opts)
	if err != nil {
		return err
	}
	var ids []int
	for v := range assignment {
		ids = append(ids, v)
	}
	sort.Ints(ids)
	dense := make([]int, len(ids))
	for i, v := range ids {
		dense[i] = assignment[v]
	}
	if len(ids) != len(buildProblem(ops, desc).vregs) {
		return fmt.Errorf("assignment contains extraneous vregs")
	}
	t.Add(key, &RegAllocEntry{Schema: checkedTableSchema, CostModel: checkedTableCostModel, Signature: key, NVregs: len(ids), NOps: len(ops), Cost: cost, Assignment: dense})
	return nil
}

// LookupChecked refuses legacy keys and revalidates the copied candidate under
// current admission and recomputed cost. Loaded JSON is never trusted by hash alone.
func (t *RegAllocTable) LookupChecked(ops []VIROp, desc *MachineDesc, opts SolverOptions) (map[int]int, int, bool) {
	key, err := CheckedTableKey(ops, desc, opts)
	if err != nil {
		return nil, 0, false
	}
	t.mu.RLock()
	entry, ok := t.entries[key]
	if !ok || entry.Schema != checkedTableSchema || entry.CostModel != checkedTableCostModel || entry.Signature != key || entry.NOps != len(ops) {
		t.mu.RUnlock()
		return nil, 0, false
	}
	copyEntry := *entry
	copyEntry.Assignment = append([]int(nil), entry.Assignment...)
	t.mu.RUnlock()
	var ids []int
	for v := range buildProblem(ops, desc).vregs {
		ids = append(ids, v)
	}
	sort.Ints(ids)
	if len(ids) != copyEntry.NVregs || len(ids) != len(copyEntry.Assignment) {
		return nil, 0, false
	}
	assignment := make(map[int]int, len(ids))
	for i, v := range ids {
		assignment[v] = copyEntry.Assignment[i]
	}
	cost, err := checkedLocalCost(ops, assignment, desc, opts)
	if err != nil || cost != copyEntry.Cost {
		return nil, 0, false
	}
	return assignment, cost, true
}
