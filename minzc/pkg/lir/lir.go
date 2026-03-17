// Package lir implements the Low-level Intermediate Representation for MinZ.
//
// LIR sits between MIR2 (target-independent) and assembly text. Each LIR
// instruction maps 1:1 to a target machine instruction, but operands are
// expressed as virtual registers with LocSet constraints (allowed physical
// locations).
//
// Architecture:
//
//	MIR2 → isel (pattern table) → LIR → regalloc (LocSet-aware) → emit (template)
//
// LIR is parameterized by a MachineDesc — the same isel/alloc/emit code
// works for any target by swapping the descriptor.
//
// Design goals:
//   - Correctness by construction: impossible operand combos can't be expressed
//   - Multi-target testing: MIR2-VM == LIR-VM(any arch) for convergence
//   - Data-driven: targets defined by tables + constraint rules, not Go code
package lir

// ── LocSet — bitfield of allowed physical locations ─────────────────────────

// LocSet is a bitfield representing a set of physical storage locations.
// Bit N corresponds to the Nth entry in MachineDesc.Locs.
// A zero LocSet means "unconstrained" (any location allowed).
type LocSet uint64

// MaxLocs is the maximum number of physical locations (limited by LocSet width).
const MaxLocs = 64

func (s LocSet) Has(i int) bool    { return s&(1<<i) != 0 }
func (s LocSet) Set(i int) LocSet  { return s | (1 << i) }
func (s LocSet) Clear(i int) LocSet { return s &^ (1 << i) }
func (s LocSet) And(o LocSet) LocSet { return s & o }
func (s LocSet) Or(o LocSet) LocSet  { return s | o }
func (s LocSet) IsEmpty() bool     { return s == 0 }
func (s LocSet) Count() int {
	n := 0
	for x := s; x != 0; x &= x - 1 {
		n++
	}
	return n
}

// Singleton returns a LocSet with only bit i set.
func Singleton(i int) LocSet { return 1 << i }

// Range returns a LocSet with bits [lo, hi) set.
func Range(lo, hi int) LocSet {
	var s LocSet
	for i := lo; i < hi; i++ {
		s = s.Set(i)
	}
	return s
}

// ── Physical Location ───────────────────────────────────────────────────────

// Loc describes one physical storage location on the target machine.
type Loc struct {
	Name   string // "A", "HL", "X", "r0", "$F000"
	Width  int    // bits: 8, 16, 32
	Kind   LocKind
	Alias  LocSet // locations that physically overlap (sub-register aliasing)
}

// LocKind categorizes physical locations.
type LocKind int

const (
	LocReg    LocKind = iota // general-purpose register
	LocAcc                   // accumulator (ALU destination)
	LocIndex                 // index/pointer register
	LocPair                  // register pair (H+L → HL)
	LocFlag                  // CPU flags (boolean results)
	LocMem                   // memory spill slot
	LocImm                   // immediate value (pseudo-location for constants)
)

// ── Machine Descriptor ──────────────────────────────────────────────────────

// MachineDesc defines a target machine: its registers, instruction patterns,
// and constraint rules. This is the ONLY thing that changes between targets.
type MachineDesc struct {
	Name     string           // "z80", "6502", "risc32", "cisc", "micro"
	Locs     []Loc            // physical locations (index = bit in LocSet)
	Patterns []Pattern        // instruction patterns
	Rules    []ConstraintRule // encoding constraints
	WordSize int              // default operand width in bits (8 for Z80/6502)
}

// LocByName returns the index of the named location, or -1.
func (m *MachineDesc) LocByName(name string) int {
	for i, l := range m.Locs {
		if l.Name == name {
			return i
		}
	}
	return -1
}

// LocSetByNames returns a LocSet from location names.
func (m *MachineDesc) LocSetByNames(names ...string) LocSet {
	var s LocSet
	for _, n := range names {
		if i := m.LocByName(n); i >= 0 {
			s = s.Set(i)
		}
	}
	return s
}

// LocsOfWidth returns a LocSet of all locations with the given bit width.
func (m *MachineDesc) LocsOfWidth(w int) LocSet {
	var s LocSet
	for i, l := range m.Locs {
		if l.Width == w {
			s = s.Set(i)
		}
	}
	return s
}

// ── Instruction Pattern ─────────────────────────────────────────────────────

// Pattern describes one target instruction. Patterns are data, not code.
//
// Example (Z80):
//
//	Pattern{
//	    Name:     "add_a_r",
//	    MIROp:    OpAdd,
//	    Width:    8,
//	    DstLocs:  LocSet for {A},
//	    SrcLocs:  [2]LocSet{{A,B,C,D,E,H,L}, {}},
//	    Template: "ADD A, {src0}",
//	    Cost:     4,
//	    Bytes:    1,
//	    Clobbers: LocSet for {F},
//	}
type Pattern struct {
	Name     string    // human-readable: "add_a_r", "ld_r_r", "sbc_hl_rr"
	MIROp    int       // MIR2 opcode (OpAdd, OpSub, etc.) — use int for decoupling
	Width    int       // operand width in bits (0 = any)
	DstLocs  LocSet    // allowed locations for destination
	SrcLocs  [2]LocSet // allowed locations for source operands
	Template string    // assembly template: "ADD A, {src0}", "LD {dst}, {src0}"
	Cost     int       // execution cost (T-states, cycles)
	Bytes    int       // encoded instruction size
	Clobbers LocSet    // registers clobbered (side effects)
	Flags    PatFlags  // commutative, memory access, branch, etc.
}

// PatFlags describe instruction properties.
type PatFlags uint32

const (
	PatCommutative PatFlags = 1 << iota // src0 and src1 are interchangeable
	PatMemRead                          // reads memory via pointer operand
	PatMemWrite                         // writes memory via pointer operand
	PatBranch                           // control flow (branch/jump/call)
	PatCall                             // function call (clobbers volatile regs)
	PatImmediate                        // src0 or src1 can be an immediate
)

// ── Constraint Rules ────────────────────────────────────────────────────────

// ConstraintRule is a declarative encoding constraint.
// "If dst is in set A and src is in set B, add Cost to the edge."
// Cost = MaxCost means forbidden (impossible to encode).
type ConstraintRule struct {
	Name    string // "dd_prefix_conflict", "pair_only_add", etc.
	DstSet  LocSet // applies when dst ∈ DstSet
	SrcSet  LocSet // applies when src ∈ SrcSet
	OpMask  int    // 0 = all ops; nonzero = specific MIR2 op
	Cost    int    // additional cost (MaxCost = forbidden)
}

// MaxCost represents an impossible/forbidden combination.
const MaxCost = 1<<30

// ── LIR Program ─────────────────────────────────────────────────────────────

// Prog is a LIR program (one function).
type Prog struct {
	Name   string
	Blocks []Block
	Desc   *MachineDesc
}

// Block is a basic block of LIR instructions.
type Block struct {
	Label  string
	Params []BlockParam // formal block parameters (define vregs on entry)
	Insts  []Inst
	Term   Term // terminator (jump, branch, return)
}

// BlockParam is a formal parameter of a basic block.
// When control flows into this block, the caller passes a value via the
// corresponding Term.Args edge. The WFC allocator propagates LocSet
// constraints across edges so that caller and callee agree on registers.
type BlockParam struct {
	VReg    int    // virtual register defined by this param
	Allowed LocSet // allowed physical locations (narrowed by WFC)
	Phys    int    // assigned physical location (-1 = unassigned)
}

// Inst is one LIR instruction — a pattern instantiation with concrete operands.
type Inst struct {
	Pat     *Pattern   // which instruction pattern
	Dst     Operand    // destination operand
	Srcs    [2]Operand // source operands
	Imm     int64      // immediate value (for PatImmediate)
}

// Operand is a virtual register with location constraints.
type Operand struct {
	VReg    int    // virtual register number (0 = unused)
	Allowed LocSet // set of allowed physical locations
	Phys    int    // assigned physical location index (-1 = unassigned)
}

// Term is a block terminator.
type Term struct {
	Kind    TermKind
	Cond    Operand     // condition operand (for TermBranch)
	Counter Operand     // counter operand (for TermDJNZ)
	Targets []string    // target block labels
	Args    [][]Operand // arguments per target (for block params)
	RetVals []Operand   // return values (for TermReturn)
}

type TermKind int

const (
	TermNone   TermKind = iota // no terminator (straight-line block)
	TermJump                    // unconditional jump
	TermBranch                  // conditional branch (2 targets: [0]=then, [1]=else)
	TermDJNZ                    // decrement counter, jump if non-zero ([0]=body, [1]=exit)
	TermReturn                  // function return
)
