package vir

import (
	"fmt"
	"sort"
)

// ImportIndexedENRTQuotient optionally merges allocation variables that are
// independently forced to the same byte register and never simultaneously live.
// No original operation is rewritten: the historical assignment is lifted and
// independently certified for the complete current problem, including moves.
func ImportIndexedENRTQuotient(path string, p *LocalTableProblem, t *RegAllocTable) (*IndexedLegacyImportEvidence, error) {
	if p == nil || t == nil {
		return nil, fmt.Errorf("missing quotient input")
	}
	context, e := localFunctionContext(p.Function)
	if e != nil || context != p.Options.CheckedContext {
		return nil, fmt.Errorf("stale quotient function binding")
	}
	shape, groups, e := indexedQuotientShape(p)
	if e != nil {
		return nil, e
	}
	return importIndexedENRTMapped(path, shape, p, t, groups)
}

func indexedQuotientShape(p *LocalTableProblem) (*EnrichedShape, [][]int, error) {
	ids := []int{}
	for v := range buildProblem(p.Ops, Z80).vregs {
		ids = append(ids, v)
	}
	sort.Ints(ids)
	idSet := map[int]bool{}
	for _, v := range ids {
		idSet[v] = true
	}
	forced := map[int]int{}
	force := func(v, loc int) error {
		if v <= 0 {
			return nil
		}
		if !idSet[v] {
			return fmt.Errorf("unbound quotient ABI/hint variable")
		}
		if old, ok := forced[v]; ok && old != loc {
			return fmt.Errorf("conflicting quotient singleton domains")
		}
		if loc < 0 || loc >= len(Z80.Locs) || Z80.Locs[loc].Width != 8 {
			return fmt.Errorf("unsupported quotient singleton width/alias")
		}
		forced[v] = loc
		return nil
	}
	hint := func(v int, h LocSet) error {
		if v <= 0 || h.IsEmpty() {
			return nil
		}
		if h.Count() != 1 {
			return fmt.Errorf("non-singleton quotient hint unsupported")
		}
		for loc := range Z80.Locs {
			if h.Has(loc) {
				return force(v, loc)
			}
		}
		return fmt.Errorf("invalid quotient hint")
	}
	for _, op := range p.Ops {
		if op.Width != 8 {
			return nil, nil, fmt.Errorf("quotient currently byte-only")
		}
		if e := hint(op.Dst, op.DstHint); e != nil {
			return nil, nil, e
		}
		for j, v := range op.Src {
			if e := hint(v, op.SrcHint[j]); e != nil {
				return nil, nil, e
			}
		}
		switch op.Op {
		case OpAdd, OpSub, OpAnd, OpOr, OpXor, OpAddImm, OpSubImm, OpAndImm, OpOrImm, OpXorImm:
			if e := force(op.Dst, Z80.LocByName("A")); e != nil {
				return nil, nil, e
			}
		case OpMove, OpConst, OpRet:
		default:
			return nil, nil, fmt.Errorf("unsupported quotient operation")
		}
	}
	for v, loc := range p.Options.ParamLocs {
		if e := force(v, loc); e != nil {
			return nil, nil, e
		}
	}
	groups := [][]int{}
	membership := map[int]int{}
	singletonGroup := map[int]int{}
	for _, v := range ids {
		loc, fixed := forced[v]
		i, seen := singletonGroup[loc]
		if !fixed || !seen {
			i = len(groups)
			groups = append(groups, nil)
			if fixed {
				singletonGroup[loc] = i
			}
		}
		groups[i] = append(groups[i], v)
		membership[v] = i
	}
	if len(groups) < 2 || len(groups) > 6 {
		return nil, nil, fmt.Errorf("quotient has %d variables outside[2,6]", len(groups))
	}
	shape := &EnrichedShape{NVregs: len(groups)}
	for _, g := range groups {
		shape.Widths = append(shape.Widths, 8)
		ls := 2
		if loc, ok := forced[g[0]]; ok {
			switch Z80.Locs[loc].Name {
			case "A":
				ls = 0
			case "C":
				ls = 1
			default:
				return nil, nil, fmt.Errorf("quotient singleton ABI not representable")
			}
		}
		shape.LocSetIndex = append(shape.LocSetIndex, ls)
	}
	edges := [][2]int{}
	conflict := false
	pair := func(a, b int) {
		if a <= 0 || b <= 0 || a == b {
			return
		}
		ga, gb := membership[a], membership[b]
		if ga == gb {
			conflict = true
		} else {
			edges = append(edges, [2]int{ga, gb})
		}
	}
	live := map[int]bool{}
	for i := len(p.Ops) - 1; i >= 0; i-- {
		op := p.Ops[i]
		for v := range live {
			if op.Op != OpMove || v != op.Src[0] {
				pair(op.Dst, v)
			}
		}
		delete(live, op.Dst)
		for _, v := range op.Src {
			if v > 0 {
				live[v] = true
			}
		}
		for a := range live {
			for b := range live {
				pair(a, b)
			}
		}
	}
	if conflict {
		return nil, nil, fmt.Errorf("quotient forced-location internal live edge")
	}
	shape.Interference = InterferenceBitmask(len(groups), edges)
	return shape, groups, nil
}
