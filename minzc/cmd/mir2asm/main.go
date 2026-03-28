// mir2asm — Stage 3+4 of self-hosting pipeline.
//
// Reads MIR2 S-expr text from stdin, builds interference graph,
// looks up register assignment in enriched tables (O(1)),
// emits Z80 assembly with peephole optimization.
//
// Usage:
//   echo '(module (func add ...))' | mir2asm > output.a80
//   mir2asm < program.mir2 > program.a80
//
// This is the Go proof-of-concept. Will be ported to Nanz for self-hosting.
package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	vir "github.com/minz/minzc/pkg/vir"
)

// ── MIR2 S-expr AST ─────────────────────────────────────────────────────────

type MIR2Module struct {
	Funcs []*MIR2Func
}

type MIR2Func struct {
	Name   string
	Params []MIR2Param
	RetTy  string
	RetCls string // "acc", "gen", "flag"
	Blocks []*MIR2Block
}

type MIR2Param struct {
	Name  string
	Vreg  int
	Ty    string
	Class string // "acc", "gen", "counter", etc.
}

type MIR2Block struct {
	Label string
	Insts []*MIR2Inst
	Term  *MIR2Term
}

type MIR2Inst struct {
	Dst int    // vreg number
	Op  string // "add", "sub", "cmp", "move", "const", etc.
	Src [2]int // source vregs (-1 = unused)
	Imm int64  // immediate value
}

type MIR2Term struct {
	Kind string // "ret", "jmp", "brif"
	Vals []int  // return values (vregs)
	Cond int    // condition vreg
	Then string // then label
	Else string // else label
}

// ── S-expr parser ────────────────────────────────────────────────────────────

type sexpr struct {
	atom     string
	children []*sexpr
}

func (s *sexpr) isAtom() bool { return len(s.children) == 0 }
func (s *sexpr) str() string  { return s.atom }
func (s *sexpr) nth(i int) *sexpr {
	if i < len(s.children) {
		return s.children[i]
	}
	return &sexpr{atom: ""}
}
func (s *sexpr) len() int { return len(s.children) }

func parseSexpr(input string) (*sexpr, string) {
	input = strings.TrimLeft(input, " \t\n\r")
	if len(input) == 0 {
		return nil, ""
	}
	if input[0] == '(' {
		input = input[1:] // skip (
		node := &sexpr{}
		for {
			input = strings.TrimLeft(input, " \t\n\r")
			if len(input) == 0 || input[0] == ')' {
				if len(input) > 0 {
					input = input[1:] // skip )
				}
				return node, input
			}
			child, rest := parseSexpr(input)
			if child != nil {
				node.children = append(node.children, child)
			}
			input = rest
		}
	}
	// Atom
	end := 0
	for end < len(input) && input[end] != ' ' && input[end] != '\t' &&
		input[end] != '\n' && input[end] != '\r' &&
		input[end] != '(' && input[end] != ')' {
		end++
	}
	return &sexpr{atom: input[:end]}, input[end:]
}

// ── MIR2 S-expr → AST ───────────────────────────────────────────────────────

func parseMIR2Module(s *sexpr) *MIR2Module {
	m := &MIR2Module{}
	for i := 1; i < s.len(); i++ { // skip "module"
		child := s.nth(i)
		if !child.isAtom() && child.nth(0).str() == "func" {
			m.Funcs = append(m.Funcs, parseMIR2Func(child))
		}
	}
	return m
}

func parseMIR2Func(s *sexpr) *MIR2Func {
	f := &MIR2Func{Name: s.nth(1).str()}
	vregCounter := 1

	for i := 2; i < s.len(); i++ {
		child := s.nth(i)
		if child.isAtom() {
			continue
		}
		switch child.nth(0).str() {
		case "params":
			for j := 1; j < child.len(); j++ {
				p := child.nth(j)
				param := MIR2Param{
					Name:  p.nth(0).str(),
					Vreg:  vregCounter,
					Ty:    p.nth(1).str(),
					Class: "gen",
				}
				if p.len() > 2 {
					param.Class = p.nth(2).str()
				}
				f.Params = append(f.Params, param)
				vregCounter++
			}
		case "ret":
			f.RetTy = child.nth(1).str()
			if child.len() > 2 {
				f.RetCls = child.nth(2).str()
			}
		case "block":
			f.Blocks = append(f.Blocks, parseMIR2Block(child, &vregCounter))
		}
	}
	return f
}

func parseMIR2Block(s *sexpr, vregCounter *int) *MIR2Block {
	b := &MIR2Block{Label: s.nth(1).str()}
	for i := 2; i < s.len(); i++ {
		child := s.nth(i)
		if child.isAtom() {
			continue
		}
		// Check if it's a terminator or instruction
		first := child.nth(0).str()
		switch first {
		case "ret":
			t := &MIR2Term{Kind: "ret"}
			for j := 1; j < child.len(); j++ {
				if v, err := strconv.Atoi(strings.TrimPrefix(child.nth(j).str(), "r")); err == nil {
					t.Vals = append(t.Vals, v)
				}
			}
			b.Term = t
		case "jmp":
			b.Term = &MIR2Term{Kind: "jmp", Then: child.nth(1).str()}
		case "brif":
			t := &MIR2Term{Kind: "brif"}
			if v, err := strconv.Atoi(strings.TrimPrefix(child.nth(1).str(), "r")); err == nil {
				t.Cond = v
			}
			// (brif r3 cond then else)
			if child.len() > 3 {
				t.Then = child.nth(3).str()
			}
			if child.len() > 4 {
				t.Else = child.nth(4).str()
			}
			b.Term = t
		default:
			// Instruction: (rN = op ...)
			if child.len() >= 3 && child.nth(1).str() == "=" {
				inst := parseMIR2Inst(child)
				b.Insts = append(b.Insts, inst)
			}
		}
	}
	return b
}

func parseMIR2Inst(s *sexpr) *MIR2Inst {
	dst := parseVreg(s.nth(0).str())
	op := s.nth(2).str()
	inst := &MIR2Inst{Dst: dst, Op: op, Src: [2]int{-1, -1}}
	if s.len() > 3 {
		inst.Src[0] = parseVreg(s.nth(3).str())
		if inst.Src[0] == -1 {
			// Try as immediate
			if v, err := strconv.ParseInt(s.nth(3).str(), 0, 64); err == nil {
				inst.Imm = v
			}
		}
	}
	if s.len() > 4 {
		inst.Src[1] = parseVreg(s.nth(4).str())
		if inst.Src[1] == -1 {
			if v, err := strconv.ParseInt(s.nth(4).str(), 0, 64); err == nil {
				inst.Imm = v
			}
		}
	}
	return inst
}

func parseVreg(s string) int {
	if strings.HasPrefix(s, "r") {
		if v, err := strconv.Atoi(s[1:]); err == nil {
			return v
		}
	}
	return -1
}

// ── Interference Graph + OpBag ───────────────────────────────────────────────

func buildInterferenceAndOpBag(f *MIR2Func) (vir.InterferenceShape, vir.OpBag) {
	// Collect all vregs
	vregs := make(map[int]bool)
	for _, p := range f.Params {
		vregs[p.Vreg] = true
	}
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Dst > 0 {
				vregs[inst.Dst] = true
			}
			for _, s := range inst.Src {
				if s > 0 {
					vregs[s] = true
				}
			}
		}
	}

	// Canonicalize vregs to 0..N-1
	vregList := make([]int, 0, len(vregs))
	for v := range vregs {
		vregList = append(vregList, v)
	}
	sort.Ints(vregList)
	vregIdx := make(map[int]int)
	for i, v := range vregList {
		vregIdx[v] = i
	}

	// Simple liveness: vreg is live from def to last use (within flat op sequence)
	type opRef struct {
		dst  int
		srcs []int
	}
	var flatOps []opRef
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			ref := opRef{dst: inst.Dst}
			for _, s := range inst.Src {
				if s > 0 {
					ref.srcs = append(ref.srcs, s)
				}
			}
			flatOps = append(flatOps, ref)
		}
	}

	// Compute def/lastUse
	defAt := make(map[int]int)
	lastUse := make(map[int]int)
	// Params are defined at -1 (before all instructions)
	for _, p := range f.Params {
		defAt[p.Vreg] = -1
	}
	for i, op := range flatOps {
		if op.dst > 0 {
			defAt[op.dst] = i
		}
		for _, s := range op.srcs {
			lastUse[s] = i
		}
	}
	// Return values count as uses at the end
	for _, b := range f.Blocks {
		if b.Term != nil {
			for _, v := range b.Term.Vals {
				lastUse[v] = len(flatOps)
			}
		}
	}

	// Build interference edges
	edgeSet := make(map[[2]int]bool)
	n := len(flatOps) + 1 // +1 for return point
	for i := 0; i < n; i++ {
		var liveHere []int
		for v := range vregs {
			d, hasDef := defAt[v]
			lu, hasUse := lastUse[v]
			if !hasDef {
				d = -1
			}
			if !hasUse {
				lu = d // dead after def
			}
			if d <= i && i <= lu {
				liveHere = append(liveHere, v)
			}
		}
		sort.Ints(liveHere)
		for a := 0; a < len(liveHere); a++ {
			for b := a + 1; b < len(liveHere); b++ {
				ca, cb := vregIdx[liveHere[a]], vregIdx[liveHere[b]]
				if ca > cb {
					ca, cb = cb, ca
				}
				edgeSet[[2]int{ca, cb}] = true
			}
		}
	}

	var edges [][2]int
	for e := range edgeSet {
		edges = append(edges, e)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i][0] != edges[j][0] {
			return edges[i][0] < edges[j][0]
		}
		return edges[i][1] < edges[j][1]
	})

	shape := vir.InterferenceShape{NVregs: len(vregList), Edges: edges}

	// Op bag
	var bag vir.OpBag
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			switch inst.Op {
			case "add":
				bag.Add++
			case "sub":
				bag.Sub++
			case "mul":
				bag.Mul++
			case "cmp":
				bag.Cmp++
			case "and", "or", "xor":
				bag.Logic++
			case "shl", "shr":
				bag.Shift++
			case "load":
				bag.Load++
			case "store":
				bag.Store++
			case "call":
				bag.Call++
			case "move":
				bag.Move++
			case "const":
				bag.Const++
			case "neg":
				bag.Neg++
			}
		}
	}

	return shape, bag
}

// ── Main ─────────────────────────────────────────────────────────────────────

func main() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		os.Exit(1)
	}

	// Parse S-expr
	root, _ := parseSexpr(string(input))
	if root == nil {
		fmt.Fprintf(os.Stderr, "empty input\n")
		os.Exit(1)
	}

	// Parse MIR2 module
	mod := parseMIR2Module(root)
	table := vir.GetRegAllocTable()

	for _, f := range mod.Funcs {
		shape, bag := buildInterferenceAndOpBag(f)
		sig := vir.EnrichedSignature{
			ShapeHash: shape.Hash(),
			OpBagHash: bag.Hash(),
			NVregs:    shape.NVregs,
		}

		key := fmt.Sprintf("%d:%d", sig.ShapeHash, sig.OpBagHash)
		entry, ok := table.LookupByKey(key)

		fmt.Fprintf(os.Stderr, "[mir2asm] %s: %dv, %d edges, bag={add:%d sub:%d cmp:%d move:%d const:%d}\n",
			f.Name, shape.NVregs, len(shape.Edges), bag.Add, bag.Sub, bag.Cmp, bag.Move, bag.Const)

		if ok {
			fmt.Fprintf(os.Stderr, "[mir2asm] %s: TABLE HIT cost=%d assignment=%v\n", f.Name, entry.Cost, entry.Assignment)
		} else {
			fmt.Fprintf(os.Stderr, "[mir2asm] %s: TABLE MISS (need Z3 fallback)\n", f.Name)

			// Try decomposition
			cuts := shape.FindCutVertices()
			if len(cuts) > 0 {
				comps := shape.DecomposeAtCutVertices()
				fmt.Fprintf(os.Stderr, "[mir2asm] %s: %d cut vertices, %d components\n",
					f.Name, len(cuts), len(comps))
			}
			bip, _ := shape.IsBipartite()
			if bip {
				fmt.Fprintf(os.Stderr, "[mir2asm] %s: bipartite (EXX feasible)\n", f.Name)
			}
		}

		// Emit function header
		fmt.Printf("; fun %s\n", f.Name)
		fmt.Printf("%s:\n", f.Name)
		if ok {
			fmt.Printf("    ; regalloc: O(1) table hit, cost=%d\n", entry.Cost)
		} else {
			fmt.Printf("    ; regalloc: table miss, needs Z3\n")
		}
		fmt.Printf("    RET\n\n")
	}
}
