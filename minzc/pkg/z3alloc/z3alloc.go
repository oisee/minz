// Package z3alloc implements optimal register allocation via Z3 SMT solver.
//
// Formulation: register allocation as constraint satisfaction + optimization.
//
//   Variables:  reg_v  ∈ {0..N-1}  for each virtual register v
//   Hard:       reg_v ≠ reg_w      when v and w are simultaneously live
//   Soft:       minimize Σ cost(class_v, phys(reg_v))
//
// Z3 finds a provably optimal assignment or reports unsatisfiable.
// Falls back to PBQP if Z3 is unavailable or times out.
//
// Usage:
//
//   result, err := z3alloc.Allocate(func, liveness, costTable, z3alloc.Options{Timeout: 5*time.Second})
//   if err != nil { /* fallback to PBQP */ }
package z3alloc

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/minz/minzc/pkg/mir2"
)

// Options controls the Z3 allocator.
type Options struct {
	Timeout time.Duration // Z3 timeout (default 5s)
	Z3Path  string        // path to z3 binary (default: "z3")
	Verbose bool          // print SMT-LIB2 and model
}

// Allocate runs Z3-optimal register allocation for a single function.
// Returns nil, error if Z3 is unavailable or times out — caller should
// fallback to PBQP.
func Allocate(f *mir2.Func, lr *mir2.LivenessResult, ct mir2.CostTable, opts Options) (*mir2.AllocResult, error) {
	if opts.Z3Path == "" {
		opts.Z3Path = "z3"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Second
	}

	// Check z3 exists
	if _, err := exec.LookPath(opts.Z3Path); err != nil {
		return nil, fmt.Errorf("z3 not found: %w", err)
	}

	locs := ct.Locs()
	regs := collectRegs(f, lr)
	interferences := computeInterferences(f, regs, lr)

	smt := generateSMT(f, regs, locs, interferences, ct)

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "[z3alloc] SMT-LIB2 (%d vars, %d constraints):\n%s\n", len(regs), len(interferences), smt)
	}

	model, err := runZ3(smt, opts)
	if err != nil {
		return nil, err
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "[z3alloc] model: %s\n", model)
	}

	return parseModel(model, regs, locs)
}

// regInfo tracks a virtual register for allocation.
type regInfo struct {
	reg   mir2.Reg
	class mir2.RegClass
	width int // bits
}

// interference is a pair of vregs that cannot share a physical location.
type interference struct {
	a, b mir2.Reg
}

func collectRegs(f *mir2.Func, lr *mir2.LivenessResult) []regInfo {
	seen := map[mir2.Reg]bool{}
	var regs []regInfo
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			if !seen[p.Dst] {
				seen[p.Dst] = true
				regs = append(regs, regInfo{reg: p.Dst, class: p.Class, width: p.Ty.Width() * 8})
			}
		}
		for _, inst := range b.Insts {
			if inst.Dst != mir2.NoReg && !seen[inst.Dst] {
				seen[inst.Dst] = true
				regs = append(regs, regInfo{reg: inst.Dst, class: inst.Cls, width: inst.Ty.Width() * 8})
			}
		}
	}
	return regs
}

func computeInterferences(f *mir2.Func, regs []regInfo, lr *mir2.LivenessResult) []interference {
	ig := mir2.BuildInterferenceGraph(f, lr)
	var result []interference
	for i := 0; i < len(regs); i++ {
		for j := i + 1; j < len(regs); j++ {
			neighbors := ig.Neighbors(regs[i].reg)
			if neighbors != nil && neighbors.Has(regs[j].reg) {
				result = append(result, interference{regs[i].reg, regs[j].reg})
			}
		}
	}
	return result
}

// generateSMT produces SMT-LIB2 for the allocation problem.
func generateSMT(f *mir2.Func, regs []regInfo, locs []mir2.PhysLoc, intf []interference, ct mir2.CostTable) string {
	var sb strings.Builder

	sb.WriteString("; Z3 register allocation for " + f.Name + "\n")
	sb.WriteString("(set-logic QF_LIA)\n")

	nlocs := len(locs)

	// Variable per vreg: which physical location (0..nlocs-1)
	for _, r := range regs {
		sb.WriteString(fmt.Sprintf("(declare-const r%d Int)\n", r.reg))
		sb.WriteString(fmt.Sprintf("(assert (>= r%d 0))\n", r.reg))
		sb.WriteString(fmt.Sprintf("(assert (< r%d %d))\n", r.reg, nlocs))
	}

	// Domain restriction: only locations compatible with this reg's class
	for _, r := range regs {
		var allowed []int
		for i, loc := range locs {
			if ct.Cost(r.class, loc) < mir2.InfCost {
				allowed = append(allowed, i)
			}
		}
		if len(allowed) < nlocs && len(allowed) > 0 {
			var parts []string
			for _, a := range allowed {
				parts = append(parts, fmt.Sprintf("(= r%d %d)", r.reg, a))
			}
			sb.WriteString(fmt.Sprintf("(assert (or %s))\n", strings.Join(parts, " ")))
		}
	}

	// Interference constraints: live-at-same-point → different location
	for _, pair := range intf {
		sb.WriteString(fmt.Sprintf("(assert (not (= r%d r%d)))\n", pair.a, pair.b))
	}

	// Objective: minimize total cost
	// cost_v = ite(r_v = 0, cost(cls, loc[0]), ite(r_v = 1, cost(cls, loc[1]), ...))
	sb.WriteString("\n; Cost function\n")
	sb.WriteString("(declare-const total_cost Int)\n")
	var costTerms []string
	for _, r := range regs {
		costVar := fmt.Sprintf("cost_%d", r.reg)
		sb.WriteString(fmt.Sprintf("(declare-const %s Int)\n", costVar))
		// Build nested ite for cost lookup
		expr := "0"
		for i := len(locs) - 1; i >= 0; i-- {
			c := ct.Cost(r.class, locs[i])
			if c >= mir2.InfCost {
				c = 9999
			}
			expr = fmt.Sprintf("(ite (= r%d %d) %d %s)", r.reg, i, c, expr)
		}
		sb.WriteString(fmt.Sprintf("(assert (= %s %s))\n", costVar, expr))
		costTerms = append(costTerms, costVar)
	}

	if len(costTerms) > 0 {
		sum := costTerms[0]
		for _, t := range costTerms[1:] {
			sum = fmt.Sprintf("(+ %s %s)", sum, t)
		}
		sb.WriteString(fmt.Sprintf("(assert (= total_cost %s))\n", sum))
	}

	sb.WriteString("\n(minimize total_cost)\n")
	sb.WriteString("(check-sat)\n")
	sb.WriteString("(get-model)\n")

	return sb.String()
}

func runZ3(smt string, opts Options) (string, error) {
	tmpFile, err := os.CreateTemp("", "z3alloc_*.smt2")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(smt); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	cmd := exec.Command(opts.Z3Path, "-T:"+strconv.Itoa(int(opts.Timeout.Seconds())), tmpFile.Name())
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("z3 failed: %w\noutput: %s", err, string(out))
	}

	result := string(out)
	if !strings.Contains(result, "sat") {
		return "", fmt.Errorf("z3: unsat or unknown: %s", result[:min(len(result), 200)])
	}

	return result, nil
}

func parseModel(model string, regs []regInfo, locs []mir2.PhysLoc) (*mir2.AllocResult, error) {
	ar := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}

	for _, r := range regs {
		varName := fmt.Sprintf("r%d", r.reg)
		// Find (define-fun r123 () Int 5)
		idx := strings.Index(model, "(define-fun "+varName+" ")
		if idx < 0 {
			continue
		}
		// Extract the integer value
		rest := model[idx:]
		// Find last token before closing paren
		parts := strings.Fields(rest)
		for i, p := range parts {
			if p == "Int" && i+1 < len(parts) {
				valStr := strings.TrimRight(parts[i+1], ")")
				val, err := strconv.Atoi(valStr)
				if err == nil && val >= 0 && val < len(locs) {
					ar.Locs[r.reg] = locs[val]
				}
				break
			}
		}
	}

	return ar, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
