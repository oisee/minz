// z3solve.go — SMT-based optimal register allocation via Z3.
//
// Encodes the register allocation problem as an SMT-LIB2 optimization
// problem and calls Z3 to find the provably optimal assignment.
//
// For Z80 with ~37 locations and functions of 20-200 instructions,
// Z3 solves in <100ms — fast enough for compile-time on a PC.
//
// Architecture:
//
//	LIR Prog (after isel)
//	  → EncodeSMT (Go → SMT-LIB2 text)
//	  → z3 subprocess (SMT-LIB2 → model)
//	  → ParseZ3Model (model → register assignments)
//	  → Apply to LIR Prog
//
// The SMT encoding captures:
//   - LocSet constraints (allowed physical locations per operand)
//   - Interference (live vregs can't share same physical location)
//   - Instruction pattern constraints (ADD requires A, etc.)
//   - Cost minimization (total T-states)
package lir

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Z3Path is the path to the Z3 binary. Set by init or caller.
var Z3Path = "/home/alice/miniconda3/bin/z3"

// Z3Result holds the result of SMT-based register allocation.
type Z3Result struct {
	// VRegPhys maps virtual register → physical location index.
	VRegPhys map[int]int
	// TotalCost is the total T-states cost of the assignment.
	TotalCost int
	// Optimal is true if Z3 proved this is the minimum cost.
	Optimal bool
	// Stats holds solver statistics.
	Stats Z3Stats
}

type Z3Stats struct {
	NumVRegs       int
	NumConstraints int
	NumLiveRanges  int
	SolveTimeMs    int
	RawOutput      string // Z3 stdout for debugging
}

// SolveOptimal runs Z3 to find the optimal register assignment for a LIR program.
// Returns the assignment or an error if unsatisfiable.
// SolveOptimalWithHints runs Z3 with PBQP hints as soft preferences.
// Vregs with hints get a cost bonus when assigned to the hinted location.
func SolveOptimalWithHints(prog *Prog, hints map[int]int) (*Z3Result, error) {
	return solveOptimalImpl(prog, hints)
}

func SolveOptimal(prog *Prog) (*Z3Result, error) {
	return solveOptimalImpl(prog, nil)
}

func solveOptimalImpl(prog *Prog, hints map[int]int) (*Z3Result, error) {
	if len(prog.Blocks) == 0 {
		return &Z3Result{VRegPhys: map[int]int{}}, nil
	}

	// Build liveness information.
	live := computeLiveness(prog)

	// Encode as SMT-LIB2.
	smt, enc := encodeSMT(prog, live, hints)

	// Call Z3.
	cmd := exec.Command(Z3Path, "-in", "-smt2")
	cmd.Stdin = strings.NewReader(smt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("z3 failed: %v\nstderr: %s\nSMT:\n%s", err, stderr.String(), smt)
	}

	// Parse result.
	result, err := parseZ3Output(stdout.String(), enc, prog.Desc)
	if err != nil {
		return nil, err
	}
	result.Stats.RawOutput = stdout.String()
	return result, nil
}

// === Liveness Analysis ===

type liveInfo struct {
	// defAt[vreg] = instruction index where vreg is defined
	defAt map[int]int
	// lastUse[vreg] = last instruction index where vreg is used
	lastUse map[int]int
	// liveAt[instIdx] = set of vregs live at this instruction
	liveAt []map[int]bool
	// interferes[vreg] = set of vregs that interfere (overlapping live ranges)
	interferes map[int]map[int]bool
	// tied[{va,vb}] = true if va and vb MUST be in the same register (tied operand)
	tied map[[2]int]bool
}

func computeLiveness(prog *Prog) *liveInfo {
	li := &liveInfo{
		defAt:      make(map[int]int),
		lastUse:    make(map[int]int),
		interferes: make(map[int]map[int]bool),
	}

	// Flatten all blocks into instruction sequence for now.
	// TODO: proper inter-block liveness for multi-block functions.
	var allInsts []Inst
	for _, b := range prog.Blocks {
		allInsts = append(allInsts, b.Insts...)
	}

	// Pass 1: find def and last use for each vreg.
	for i, inst := range allInsts {
		if inst.Dst.VReg > 0 {
			if _, ok := li.defAt[inst.Dst.VReg]; !ok {
				li.defAt[inst.Dst.VReg] = i
			}
		}
		for s := 0; s < 2; s++ {
			if inst.Srcs[s].VReg > 0 {
				li.lastUse[inst.Srcs[s].VReg] = i
			}
		}
		if inst.Dst.VReg > 0 {
			// dst is also a "use" for liveness purposes at def point
			if cur, ok := li.lastUse[inst.Dst.VReg]; !ok || i > cur {
				li.lastUse[inst.Dst.VReg] = i
			}
		}
	}

	// Pass 2: compute liveAt for each instruction.
	li.liveAt = make([]map[int]bool, len(allInsts))
	for i := range allInsts {
		li.liveAt[i] = make(map[int]bool)
	}
	for vreg, def := range li.defAt {
		last, ok := li.lastUse[vreg]
		if !ok {
			last = def
		}
		for i := def; i <= last && i < len(allInsts); i++ {
			li.liveAt[i][vreg] = true
		}
	}

	// Pass 3: build interference graph (standard Chaitin-style).
	// Two vregs interfere if one is live at the definition point of the other.
	for va := range li.defAt {
		for vb := range li.defAt {
			if va >= vb {
				continue
			}
			defA, lastA := li.defAt[va], li.lastUse[va]
			defB, lastB := li.defAt[vb], li.lastUse[vb]
			// Overlap: [defA, lastA] ∩ [defB, lastB] != ∅
			if defA <= lastB && defB <= lastA {
				if li.interferes[va] == nil {
					li.interferes[va] = make(map[int]bool)
				}
				if li.interferes[vb] == nil {
					li.interferes[vb] = make(map[int]bool)
				}
				li.interferes[va][vb] = true
				li.interferes[vb][va] = true
			}
		}
	}

	// Pass 4: find tied operands — vregs that MUST be in the same register.
	// e.g. ADD A,r: dst=v3 and src0=v1 both constrained to {A} → v1 = v3.
	li.tied = make(map[[2]int]bool)
	for _, inst := range allInsts {
		if inst.Dst.VReg <= 0 || inst.Dst.Allowed.IsEmpty() {
			continue
		}
		for s := 0; s < 2; s++ {
			sv := inst.Srcs[s].VReg
			if sv <= 0 || sv == inst.Dst.VReg {
				continue
			}
			// If dst and src have identical singleton constraint → tied.
			if inst.Dst.Allowed == inst.Srcs[s].Allowed && inst.Dst.Allowed.Count() == 1 {
				pair := [2]int{min(inst.Dst.VReg, sv), max(inst.Dst.VReg, sv)}
				li.tied[pair] = true
				// Remove interference between tied pair — they MUST share.
				delete(li.interferes[pair[0]], pair[1])
				delete(li.interferes[pair[1]], pair[0])
			}
		}
	}

	return li
}

// === SMT Encoding ===

type smtEncoding struct {
	// vregVar maps vreg → SMT variable name
	vregVar map[int]string
	// vregs is the list of all vregs in order
	vregs []int
	// locCount is the number of physical locations
	locCount int
}

func encodeSMT(prog *Prog, live *liveInfo, hints map[int]int) (string, *smtEncoding) {
	desc := prog.Desc
	enc := &smtEncoding{
		vregVar:  make(map[int]string),
		locCount: len(desc.Locs),
	}

	var b strings.Builder
	b.WriteString("; Z3 optimal register allocation for LIR\n")
	b.WriteString("; Generated by MinZ compiler\n")
	b.WriteString("(set-option :produce-models true)\n")
	b.WriteString("(set-logic QF_LIA)\n") // quantifier-free linear integer arithmetic
	b.WriteString("\n")

	// Collect all vregs.
	vregSet := make(map[int]bool)
	for _, blk := range prog.Blocks {
		for _, inst := range blk.Insts {
			if inst.Dst.VReg > 0 {
				vregSet[inst.Dst.VReg] = true
			}
			for s := 0; s < 2; s++ {
				if inst.Srcs[s].VReg > 0 {
					vregSet[inst.Srcs[s].VReg] = true
				}
			}
		}
	}
	for v := range vregSet {
		enc.vregs = append(enc.vregs, v)
	}

	// Sort for deterministic output.
	sortInts(enc.vregs)

	// Declare one integer variable per vreg: value = physical location index.
	b.WriteString("; --- Virtual register variables ---\n")
	for _, v := range enc.vregs {
		name := fmt.Sprintf("v%d", v)
		enc.vregVar[v] = name
		b.WriteString(fmt.Sprintf("(declare-const %s Int)\n", name))
	}
	b.WriteString("\n")

	// Declare cost variable per vreg.
	b.WriteString("; --- Cost variables ---\n")
	for _, v := range enc.vregs {
		b.WriteString(fmt.Sprintf("(declare-const cost_%s Int)\n", enc.vregVar[v]))
	}
	b.WriteString("(declare-const total_cost Int)\n")
	b.WriteString("\n")

	// Constraint 1: Domain — each vreg must be in its allowed LocSet.
	b.WriteString("; --- Domain constraints (LocSet) ---\n")
	allowedSets := collectAllowedSets(prog)
	for _, v := range enc.vregs {
		allowed, ok := allowedSets[v]
		if !ok || allowed.IsEmpty() {
			// Unconstrained — allow all non-flag, non-stack locs.
			allowed = desc.LocsOfWidth(8).Or(desc.LocsOfWidth(16))
		}

		var opts []string
		for i := 0; i < len(desc.Locs); i++ {
			if allowed.Has(i) {
				opts = append(opts, fmt.Sprintf("(= %s %d)", enc.vregVar[v], i))
			}
		}
		if len(opts) == 1 {
			b.WriteString(fmt.Sprintf("(assert %s)\n", opts[0]))
		} else if len(opts) > 1 {
			b.WriteString(fmt.Sprintf("(assert (or %s))\n", strings.Join(opts, " ")))
		}
	}
	b.WriteString("\n")

	// Constraint 2: Interference — live vregs can't share location.
	b.WriteString("; --- Interference constraints ---\n")
	seen := make(map[[2]int]bool)
	for va, neighbors := range live.interferes {
		for vb := range neighbors {
			pair := [2]int{min(va, vb), max(va, vb)}
			if seen[pair] {
				continue
			}
			seen[pair] = true
			nameA, okA := enc.vregVar[va]
			nameB, okB := enc.vregVar[vb]
			if okA && okB {
				b.WriteString(fmt.Sprintf("(assert (not (= %s %s)))\n", nameA, nameB))
			}
		}
	}
	b.WriteString("\n")

	// Constraint 3: Tied operands — must be same register.
	b.WriteString("; --- Tied operand constraints ---\n")
	for pair := range live.tied {
		nameA, okA := enc.vregVar[pair[0]]
		nameB, okB := enc.vregVar[pair[1]]
		if okA && okB {
			b.WriteString(fmt.Sprintf("(assert (= %s %s))\n", nameA, nameB))
		}
	}
	b.WriteString("\n")

	// Constraint 4: Alias — sub-register conflicts.
	// If vreg A is in H and vreg B is in HL, they conflict.
	b.WriteString("; --- Alias constraints ---\n")
	for i, loc := range desc.Locs {
		if loc.Alias.IsEmpty() {
			continue
		}
		for j := 0; j < len(desc.Locs); j++ {
			if !loc.Alias.Has(j) || i == j {
				continue
			}
			// For each pair of interfering vregs:
			// if one is in loc i, the other can't be in loc j
			for va, neighbors := range live.interferes {
				for vb := range neighbors {
					if va >= vb {
						continue
					}
					nameA, okA := enc.vregVar[va]
					nameB, okB := enc.vregVar[vb]
					if !okA || !okB {
						continue
					}
					b.WriteString(fmt.Sprintf("(assert (not (and (= %s %d) (= %s %d))))\n",
						nameA, i, nameB, j))
				}
			}
		}
	}
	b.WriteString("\n")

	// Constraint 4: Cost model — assign cost per vreg based on location.
	b.WriteString("; --- Cost model ---\n")
	for _, v := range enc.vregs {
		name := enc.vregVar[v]
		costName := fmt.Sprintf("cost_%s", name)

		// Build ITE chain: cost = if loc=0 then cost[0] elif loc=1 then cost[1] ...
		if len(desc.LocCost) > 0 {
			expr := buildCostITE(name, desc.LocCost, len(desc.Locs))
			b.WriteString(fmt.Sprintf("(assert (= %s %s))\n", costName, expr))
		} else {
			b.WriteString(fmt.Sprintf("(assert (= %s 0))\n", costName))
		}
	}

	// Hint bonus: if PBQP hints suggest a register, add cost for NOT using it.
	// This makes Z3 prefer the hinted register while still allowing alternatives.
	if hints != nil {
		b.WriteString("; --- PBQP hint preferences ---\n")
		for _, v := range enc.vregs {
			if hinted, ok := hints[v]; ok {
				name := enc.vregVar[v]
				// Add cost 10 if NOT matching hint. This biases Z3 toward PBQP's choice
				// without making it mandatory (hard constraint would be too strict).
				b.WriteString(fmt.Sprintf("(declare-const hint_%s Int)\n", name))
				b.WriteString(fmt.Sprintf("(assert (= hint_%s (ite (= %s %d) 0 10)))\n",
					name, name, hinted))
			}
		}
		b.WriteString("\n")
	}

	// Total cost = sum of all vreg costs + hint penalties.
	var costTerms []string
	for _, v := range enc.vregs {
		costTerms = append(costTerms, fmt.Sprintf("cost_%s", enc.vregVar[v]))
	}
	if hints != nil {
		for _, v := range enc.vregs {
			if _, ok := hints[v]; ok {
				costTerms = append(costTerms, fmt.Sprintf("hint_%s", enc.vregVar[v]))
			}
		}
	}
	if len(costTerms) > 1 {
		b.WriteString(fmt.Sprintf("(assert (= total_cost (+ %s)))\n", strings.Join(costTerms, " ")))
	} else if len(costTerms) == 1 {
		b.WriteString(fmt.Sprintf("(assert (= total_cost %s))\n", costTerms[0]))
	} else {
		b.WriteString("(assert (= total_cost 0))\n")
	}
	b.WriteString("\n")

	// Optimize: minimize total cost.
	b.WriteString("; --- Optimization objective ---\n")
	b.WriteString("(minimize total_cost)\n")
	b.WriteString("\n")

	// Solve.
	b.WriteString("(check-sat)\n")
	b.WriteString("(get-model)\n")

	return b.String(), enc
}

// collectAllowedSets gathers the intersection of all LocSet constraints
// on each vreg across all instructions.
func collectAllowedSets(prog *Prog) map[int]LocSet {
	allowed := make(map[int]LocSet)
	for _, blk := range prog.Blocks {
		for _, inst := range blk.Insts {
			if inst.Dst.VReg > 0 && !inst.Dst.Allowed.IsEmpty() {
				if prev, ok := allowed[inst.Dst.VReg]; ok {
					allowed[inst.Dst.VReg] = prev.And(inst.Dst.Allowed)
				} else {
					allowed[inst.Dst.VReg] = inst.Dst.Allowed
				}
			}
			for s := 0; s < 2; s++ {
				v := inst.Srcs[s].VReg
				if v > 0 && !inst.Srcs[s].Allowed.IsEmpty() {
					if prev, ok := allowed[v]; ok {
						allowed[v] = prev.And(inst.Srcs[s].Allowed)
					} else {
						allowed[v] = inst.Srcs[s].Allowed
					}
				}
			}
		}
	}
	return allowed
}

func buildCostITE(varName string, costs []int, numLocs int) string {
	// (ite (= v 0) cost0 (ite (= v 1) cost1 ... default))
	n := numLocs
	if n > len(costs) {
		n = len(costs)
	}
	if n == 0 {
		return "0"
	}
	// Build from the inside out.
	result := fmt.Sprintf("%d", costs[n-1])
	for i := n - 2; i >= 0; i-- {
		result = fmt.Sprintf("(ite (= %s %d) %d %s)", varName, i, costs[i], result)
	}
	return result
}

// === Z3 Output Parsing ===

func parseZ3Output(output string, enc *smtEncoding, desc *MachineDesc) (*Z3Result, error) {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty z3 output")
	}

	// First line should be "sat".
	status := strings.TrimSpace(lines[0])
	if status == "unsat" {
		return nil, fmt.Errorf("z3: unsatisfiable — no valid register assignment exists")
	}
	if status != "sat" {
		return nil, fmt.Errorf("z3: unexpected status %q\nfull output:\n%s", status, output)
	}

	// Parse model: (define-fun varname () Int value)
	result := &Z3Result{
		VRegPhys: make(map[int]int),
		Optimal:  true,
		Stats: Z3Stats{
			NumVRegs: len(enc.vregs),
		},
	}

	// Z3 model format is multi-line:
	//   (define-fun v1 () Int
	//     0)
	// Join all lines, then use regex-free parsing.
	model := strings.Join(lines[1:], " ") // skip "sat" line

	// Find each "(define-fun NAME () Int VALUE)"
	// The key insight: after "() Int" there's a value before the matching ")".
	for {
		idx := strings.Index(model, "(define-fun ")
		if idx < 0 {
			break
		}
		model = model[idx+len("(define-fun "):]

		// Get name (first token).
		spaceIdx := strings.IndexByte(model, ' ')
		if spaceIdx < 0 {
			break
		}
		name := model[:spaceIdx]

		// Find "() Int" marker.
		intIdx := strings.Index(model, "() Int")
		if intIdx < 0 {
			break
		}
		after := model[intIdx+len("() Int"):]

		// Find the closing paren for this define-fun.
		closeIdx := strings.IndexByte(after, ')')
		if closeIdx < 0 {
			break
		}
		valStr := strings.TrimSpace(after[:closeIdx])
		model = after[closeIdx+1:]

		val, err := strconv.Atoi(valStr)
		if err != nil {
			// Could be negative: (- N)
			valStr = strings.TrimPrefix(valStr, "(- ")
			valStr = strings.TrimSuffix(valStr, ")")
			val, err = strconv.Atoi(valStr)
			if err != nil {
				continue
			}
			val = -val
		}

		if name == "total_cost" {
			result.TotalCost = val
			continue
		}
		if strings.HasPrefix(name, "cost_") {
			continue
		}

		for vreg, vname := range enc.vregVar {
			if vname == name {
				result.VRegPhys[vreg] = val
				break
			}
		}
	}

	return result, nil
}

// === Utility ===

func sortInts(a []int) {
	// Simple insertion sort (vregs list is small).
	for i := 1; i < len(a); i++ {
		key := a[i]
		j := i - 1
		for j >= 0 && a[j] > key {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = key
	}
}

// ApplyZ3Result applies the Z3 register assignment back to the LIR program.
func ApplyZ3Result(prog *Prog, result *Z3Result) {
	for bi := range prog.Blocks {
		for ii := range prog.Blocks[bi].Insts {
			inst := &prog.Blocks[bi].Insts[ii]
			if inst.Dst.VReg > 0 {
				if phys, ok := result.VRegPhys[inst.Dst.VReg]; ok {
					inst.Dst.Phys = phys
					inst.Dst.Allowed = Singleton(phys)
				}
			}
			for s := 0; s < 2; s++ {
				if inst.Srcs[s].VReg > 0 {
					if phys, ok := result.VRegPhys[inst.Srcs[s].VReg]; ok {
						inst.Srcs[s].Phys = phys
						inst.Srcs[s].Allowed = Singleton(phys)
					}
				}
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
