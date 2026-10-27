package pipeline

import (
	"sort"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

func variantObjective(o Options) string {
	if o.OptSize {
		return "z80-encoded-bytes-v1"
	}
	return variantCostModel
}

func variantCodegenOptions(o Options) map[string]mir2.Z80FunctionOptions {
	p := map[string]mir2.Z80FunctionOptions{}
	for name, v := range o.FunctionVariants {
		p[name] = mir2.Z80FunctionOptions{CompactByteLoops: v.CompactByteLoops, BitTests: v.BitTests, DJNZTrampoline: v.DJNZTrampoline, CompactShifts: v.CompactShifts}
	}
	return p
}

// Eligibility only avoids provable no-ops; it never predicts profitability.
func sizeAxisRelevant(axis int, s Steps, name string, callers map[string]bool) bool {
	f := s.MIR2Module.FuncByName(name)
	if f == nil {
		return false
	}
	switch axis {
	case 5:
		return callers[name]
	case 6:
		return s.Traces[name] != nil && s.Traces[name].PtrThreading > 0
	case 7:
		// Turning carry fusion off can matter only if native code consumed
		// carry. A remaining BIT predicate already uses the unfused form.
		body := s.Assembly
		marker := "; fun " + name + "("
		if at := strings.Index(body, marker); at >= 0 {
			body = body[at+len(marker):]
		}
		if at := strings.Index(body, "; fun "); at >= 0 {
			body = body[:at]
		}
		if !strings.Contains(body, " C,") && !strings.Contains(body, " NC,") && !strings.Contains(body, "RET C") && !strings.Contains(body, "RET NC") {
			return false
		}
	case 9:
		constants := map[mir2.Reg]int64{}
		for _, b := range f.Blocks {
			for _, x := range b.Insts {
				if x.Op == mir2.OpConst {
					constants[x.Dst] = x.Imm
				}
			}
		}
		for _, b := range f.Blocks {
			for _, x := range b.Insts {
				if x.Op == mir2.OpShl || x.Op == mir2.OpShr || x.Op == mir2.OpSar {
					if n, ok := constants[x.Src[1]]; ok && n >= 2 && n <= 255 {
						return true
					}
				}
			}
		}
		return false
	case 8:
		for _, b := range f.Blocks {
			if _, ok := b.Term.(*mir2.TermDJNZ); ok {
				return true
			}
		}
		return false
	case 10:
		return len(f.Blocks) > 0
	}
	for _, b := range f.Blocks {
		for _, x := range b.Insts {
			if axis == 4 && strings.HasPrefix(x.Sym, "@mir.mem.") {
				return true
			}
			if axis == 7 && (x.Op == mir2.OpShl || x.Op == mir2.OpShr || x.Op == mir2.OpSar) {
				return true
			}
		}
	}
	return false
}

// Allocate the fixed final-axis budget to larger live instruction graphs first.
// Lowering/Grace axes retain their original ordering. Names only break ties;
// neither names nor expected benchmark outcomes estimate profitability.
func variantAxisOrder(axis int, m *mir2.Module, decisions []VariantDecision) []int {
	order := make([]int, len(decisions))
	for i := range order {
		order[i] = i
	}
	if axis != 10 {
		return order
	}
	weight := map[string]int{}
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			weight[f.Name] += len(b.Insts)
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		return weight[decisions[order[i]].Function] > weight[decisions[order[j]].Function]
	})
	return order
}
