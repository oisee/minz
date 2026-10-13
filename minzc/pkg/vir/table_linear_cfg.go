package vir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
)

// PrepareLocalTableProblem binds the original CFG while representing a bounded
// unconditional chain as straight-line operations with explicit SSA edge moves.
func PrepareLocalTableProblem(f *mir2.Func, allocation *mir2.AllocResult) (*LocalTableProblem, error) {
	if f == nil || len(f.Blocks) <= 1 {
		return prepareSingleBlockTableProblem(f, allocation)
	}
	flat, err := flattenLocalLinearCFG(f)
	if err != nil {
		return nil, err
	}
	p, err := prepareSingleBlockTableProblem(flat, allocation)
	if err != nil {
		return nil, err
	}
	context, err := localFunctionContext(f)
	if err != nil {
		return nil, err
	}
	p.Function = f
	p.Options.CheckedContext = context
	return p, nil
}

func flattenLocalLinearCFG(f *mir2.Func) (*mir2.Func, error) {
	if len(f.Blocks) > 8 {
		return nil, fmt.Errorf("checked linear CFG exceeds eight blocks")
	}
	labels := map[string]*mir2.Block{}
	for _, b := range f.Blocks {
		if b == nil || b.Term == nil || b.Label == "" || labels[b.Label] != nil {
			return nil, fmt.Errorf("invalid checked linear CFG block")
		}
		labels[b.Label] = b
	}
	defined := map[mir2.Reg]mir2.Ty{}
	for _, p := range f.Contract.Params {
		if p.Reg <= 0 || defined[p.Reg] != nil {
			return nil, fmt.Errorf("invalid linear CFG input")
		}
		defined[p.Reg] = p.Ty
	}
	flat := &mir2.Block{Label: f.Blocks[0].Label, Params: append([]mir2.BlockParam(nil), f.Blocks[0].Params...)}
	visited := map[string]bool{}
	b := f.Blocks[0]
	for {
		if visited[b.Label] {
			return nil, fmt.Errorf("checked linear CFG cycle")
		}
		visited[b.Label] = true
		for _, inst := range b.Insts {
			if inst == nil || inst.Dst <= 0 || defined[inst.Dst] != nil {
				return nil, fmt.Errorf("invalid linear CFG definition")
			}
			// Full type/opcode admission runs on the resulting unchanged instruction.
			copy := *inst
			flat.Insts = append(flat.Insts, &copy)
			defined[inst.Dst] = inst.Ty
		}
		switch term := b.Term.(type) {
		case *mir2.TermRet:
			if len(visited) != len(f.Blocks) {
				return nil, fmt.Errorf("checked linear CFG has unreachable blocks")
			}
			flat.Term = &mir2.TermRet{Vals: append([]mir2.Reg(nil), term.Vals...)}
			result := *f
			result.Blocks = []*mir2.Block{flat}
			return &result, nil
		case *mir2.TermJmp:
			next := labels[term.Target]
			if next == nil || visited[term.Target] || len(term.Args) != len(next.Params) {
				return nil, fmt.Errorf("invalid checked linear CFG edge")
			}
			// Validate every source before defining any destination: edge args are
			// simultaneous, not sequential assignments that may read new values.
			destinations := map[mir2.Reg]bool{}
			for i, p := range next.Params {
				if p.Dst <= 0 || defined[p.Dst] != nil || destinations[p.Dst] || !localIntegerType(p.Ty) || defined[term.Args[i]] != p.Ty {
					return nil, fmt.Errorf("invalid checked linear CFG parameter")
				}
				destinations[p.Dst] = true
			}
			for i, p := range next.Params {
				flat.Insts = append(flat.Insts, &mir2.Inst{Op: mir2.OpMove, Dst: p.Dst, Src: [2]mir2.Reg{term.Args[i], 0}, Ty: p.Ty, Cls: p.Class})
				defined[p.Dst] = p.Ty
			}
			b = next
		default:
			return nil, fmt.Errorf("checked linear CFG requires unconditional edges")
		}
	}
}
