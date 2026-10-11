package vir

import (
	"fmt"
	"github.com/minz/minzc/pkg/z80spec"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Generate uses the existing global VIR SMT encoding without transformation
// passes that change operation/vreg identities after the certificate is bound.
// Parameter locations and independent linear interference are hard constraints.
// The solved model is only a candidate: AddChecked independently admits it.
func (p *LocalTableProblem) Generate(table *RegAllocTable, solver SolverOptions) error {
	if p == nil || table == nil {
		return fmt.Errorf("nil local generation input")
	}
	if solver.Z3Path == "" {
		solver.Z3Path = "z3"
	}
	if solver.Timeout == 0 {
		solver.Timeout = 30 * time.Second
	}
	if solver.Timeout <= 0 || solver.Timeout > 5*time.Minute {
		return fmt.Errorf("local generation timeout must be in (0,5m]")
	}
	if _, err := exec.LookPath(solver.Z3Path); err != nil {
		return fmt.Errorf("local solver unavailable: %w", err)
	}
	prob := buildProblem(p.Ops, Z80)
	for i, pats := range prob.patterns {
		if len(pats) == 0 {
			return fmt.Errorf("no pattern for local op %d", i)
		}
	}
	var constraints strings.Builder
	var params []int
	for v := range p.Options.ParamLocs {
		params = append(params, v)
	}
	sort.Ints(params)
	for _, v := range params {
		fmt.Fprintf(&constraints, "(assert (= loc_v%d %d))\n", v, p.Options.ParamLocs[v])
	}
	// Build actual simultaneous live values independently of the global solver's
	// approximation and unconditional 'tied' exclusions.
	pairs := map[[2]int]bool{}
	addPair := func(a, b int) {
		if a > b {
			a, b = b, a
		}
		if a > 0 && a != b {
			pairs[[2]int{a, b}] = true
		}
	}
	live := map[int]bool{}
	for i := len(p.Ops) - 1; i >= 0; i-- {
		op := p.Ops[i]
		if op.Dst > 0 {
			for v := range live {
				if op.Op != OpMove || v != op.Src[0] {
					addPair(op.Dst, v)
				}
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
				addPair(a, b)
			}
		}
	}
	var ordered [][2]int
	for pair := range pairs {
		ordered = append(ordered, pair)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i][0] != ordered[j][0] {
			return ordered[i][0] < ordered[j][0]
		}
		return ordered[i][1] < ordered[j][1]
	})
	for _, pair := range ordered {
		fmt.Fprintf(&constraints, "(assert (not (= loc_v%d loc_v%d)))\n", pair[0], pair[1])
		for a := range Z80.Locs {
			for b := range Z80.Locs {
				if a != b && (Z80.Locs[a].Alias.Has(b) || Z80.Locs[b].Alias.Has(a) || z80spec.Overlaps(Z80.Locs[a].Name, Z80.Locs[b].Name)) {
					fmt.Fprintf(&constraints, "(assert (not (and (= loc_v%d %d) (= loc_v%d %d))))\n", pair[0], a, pair[1], b)
				}
			}
		}
	}
	smt := strings.Replace(prob.generateSMT(), "(check-sat)", constraints.String()+"(check-sat)", 1)
	model, err := runZ3(smt, solver)
	if err != nil {
		return err
	}
	values := parseZ3Model(model)
	assignment := map[int]int{}
	for v := range prob.vregs {
		loc, ok := values[fmt.Sprintf("loc_v%d", v)]
		if !ok {
			return fmt.Errorf("solver omitted vreg %d", v)
		}
		assignment[v] = loc
	}
	return table.AddChecked(p.Ops, assignment, Z80, p.Options)
}
