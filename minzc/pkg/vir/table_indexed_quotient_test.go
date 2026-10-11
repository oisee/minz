package vir

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestIndexedQuotientForcedDomainsAndLifetime(t *testing.T) {
	ops := []VIROp{{Op: OpMove, Dst: 4, Src: [2]int{1}, Width: 8}, {Op: OpAdd, Dst: 2, Src: [2]int{1, 1}, Width: 8}, {Op: OpAdd, Dst: 3, Src: [2]int{2, 4}, Width: 8}, {Op: OpRet, Src: [2]int{3}, Width: 8}}
	p := &LocalTableProblem{Ops: ops, Options: SolverOptions{ParamLocs: map[int]int{1: 0}}}
	shape, groups, e := indexedQuotientShape(p)
	if e != nil || shape.NVregs != 2 || shape.Interference != 1 || len(groups[0]) != 3 {
		t.Fatal(e, shape, groups)
	}
	cases := map[string]func(*LocalTableProblem){
		"unbound ABI":                    func(p *LocalTableProblem) { p.Options.ParamLocs[999] = 0 },
		"internal live edge":             func(p *LocalTableProblem) { p.Ops[2].Src[1] = 1 },
		"conflicting instruction domain": func(p *LocalTableProblem) { p.Ops[1].DstHint = Singleton(Z80.LocByName("C")) },
		"pair alias ABI":                 func(p *LocalTableProblem) { p.Options.ParamLocs[1] = Z80.LocByName("HL") },
		"unrepresentable byte ABI":       func(p *LocalTableProblem) { p.Options.ParamLocs[1] = Z80.LocByName("B") },
		"non-singleton hints":            func(p *LocalTableProblem) { p.Ops[0].DstHint = Z80.LocSetByNames("A", "C") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			q := &LocalTableProblem{Ops: append([]VIROp(nil), ops...), Options: SolverOptions{ParamLocs: map[int]int{1: 0}}}
			mutate(q)
			if _, _, e := indexedQuotientShape(q); e == nil {
				t.Fatal("unsafe quotient accepted")
			}
		})
	}
	// Return ABI mutation is bound separately even when historical domains match.
	f, a := productionTableFunction(mir2.ClassAcc)
	bound, e := PrepareLocalTableProblem(f, a)
	if e != nil {
		t.Fatal(e)
	}
	f.Contract.Returns[0].Class = mir2.ClassRegC
	tab := &RegAllocTable{}
	tab.Init()
	if _, e := ImportIndexedENRTQuotient("missing", bound, tab); e == nil || tab.Size() != 0 {
		t.Fatal("stale ABI accepted")
	}
}
