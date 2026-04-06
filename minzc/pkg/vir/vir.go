// Package vir implements the Virtual IR level for MinZ.
//
// VIR sits between MIR (target-independent SSA) and PIR (physical, emit-ready).
// VIR instructions use Z80-flavored opcodes but virtual registers — no pattern
// or physical register has been chosen yet.
//
// The unified solver (Z3 primary, WFC fast-path) converts VIR → PIR in one
// pass, simultaneously selecting instruction patterns AND physical registers.
// This eliminates the 5-layer pipeline that created exponential edge cases.
//
// Architecture:
//
//	HIR → MIR → bridge → VIR → solver → PIR → emit → ASM
//
// Terminology (ADR-0039):
//
//	HIR  — High IR        (pkg/hir)   — structured, typed, frontend-independent
//	MIR  — Mid IR         (pkg/mir2)  — SSA, target-independent, block-based
//	VIR  — Virtual IR     (pkg/vir)   — Z80-flavored ops, virtual regs
//	PIR  — Physical IR    (pkg/vir)   — concrete ops, physical regs, emit-ready
//	ASM  — Assembly text  (string)    — final output
package vir

import (
	"fmt"
	"strings"
)

// ── Opcodes ──────────────────────────────────────────────────────────────────
// VIR opcodes are Z80-flavored: they distinguish 8-bit vs 16-bit operations,
// but do NOT specify which Z80 instruction to use. That's the solver's job.

type Op int

const (
	OpConst       Op = iota + 1 // dst = immediate
	OpMove                      // dst = src0 (may cross widths: trunc/zext)
	OpAdd                       // dst = src0 + src1
	OpSub                       // dst = src0 - src1
	OpMul                       // dst = src0 * src1
	OpAnd                       // dst = src0 & src1
	OpOr                        // dst = src0 | src1
	OpXor                       // dst = src0 ^ src1
	OpCmp                       // flags = cmp(src0, src1)
	OpLoad                      // dst = mem[src0]
	OpStore                     // mem[src0] = src1
	OpCall                      // dst = call sym(args...)
	OpRet                       // return src0
	OpShl                       // dst = src0 << src1
	OpShr                       // dst = src0 >> src1
	OpNeg                       // dst = -src0
	OpAddImm                    // dst = src0 + imm
	OpSubImm                    // dst = src0 - imm
	OpAndImm                    // dst = src0 & imm
	OpOrImm                     // dst = src0 | imm
	OpXorImm                    // dst = src0 ^ imm
	OpCmpImm                    // flags = cmp(src0, imm)
	OpStoreGlobal               // mem[sym] = src0
	OpLoadGlobal                // dst = mem[sym]
	OpCondRet                   // if flags, return src0
	OpLoad16LE                  // dst = load16le(src0)
	OpBitGet                    // dst(u8/A) = bit_get(src0, imm=bit)
	OpBitTest                   // flags = bit_test(src0, imm=bit)
	OpBitSet                    // dst = bit_set(src0, imm=bit)
	OpBitReset                  // dst = bit_reset(src0, imm=bit)
	OpAsmBlock                  // inline asm: emit AsmTemplate verbatim, pinned ins/outs
)

// ── VIROp ────────────────────────────────────────────────────────────────────
// A single VIR instruction. Uses virtual register numbers (v1, v2, ...).
// The solver will choose both the Z80 pattern and physical register for each.

type VIROp struct {
	Op    Op     // operation
	Dst   int    // virtual register for result (-1 = no result)
	Src   [2]int // virtual register sources (-1 = unused)
	Imm   int64  // immediate value (OpConst, OpAddImm, etc.)
	Width int    // operand width in bits (8 or 16)
	Sym   string // symbol name (OpCall target, OpStoreGlobal/OpLoadGlobal address)

	// Solver hints (optional, from bridge or PFCCO contracts):
	DstHint  LocSet    // preferred dst locations (0 = unconstrained)
	SrcHint  [2]LocSet // preferred src locations
	Clobbers LocSet    // registers clobbered by this op (OpCall)

	// Inline asm (OpAsmBlock only):
	AsmTemplate string // verbatim Z80 asm lines (newline-separated)
	AsmIns      []int  // input vregs (pinned to specific phys regs via SrcHint)
	AsmOuts     []int  // output vregs (pinned to specific phys regs via DstHint)
}

// ── LocSet ───────────────────────────────────────────────────────────────────
// Bitfield of allowed/possible physical storage locations. Bit N = Nth entry
// in MachineDesc.Locs. Covers ALL tiers: L1 GPR, L2 IX halves, L3 shadow,
// L4 TSMC, L5 memory, L6 stack.

type LocSet uint64

const MaxLocs = 64

func (s LocSet) Has(i int) bool           { return s&(1<<uint(i)) != 0 }
func (s LocSet) Set(i int) LocSet         { return s | (1 << uint(i)) }
func (s LocSet) Clear(i int) LocSet       { return s &^ (1 << uint(i)) }
func (s LocSet) And(o LocSet) LocSet      { return s & o }
func (s LocSet) Or(o LocSet) LocSet       { return s | o }
func (s LocSet) Subtract(o LocSet) LocSet { return s &^ o }
func (s LocSet) IsEmpty() bool            { return s == 0 }

func (s LocSet) Count() int {
	n := 0
	for x := s; x != 0; x &= x - 1 {
		n++
	}
	return n
}

// First returns the index of the lowest set bit, or -1 if empty.
func (s LocSet) First() int {
	if s == 0 {
		return -1
	}
	i := 0
	for s&1 == 0 {
		s >>= 1
		i++
	}
	return i
}

// ForEach calls f for each set bit index. Stops if f returns false.
func (s LocSet) ForEach(f func(int) bool) {
	for x := s; x != 0; {
		i := 0
		tmp := x
		for tmp&1 == 0 {
			tmp >>= 1
			i++
		}
		if !f(i) {
			return
		}
		x &= x - 1
	}
}

func Singleton(i int) LocSet { return 1 << uint(i) }
func Range(lo, hi int) LocSet {
	var s LocSet
	for i := lo; i < hi; i++ {
		s = s.Set(i)
	}
	return s
}

// ── Physical Location ────────────────────────────────────────────────────────

type Loc struct {
	Name  string // "A", "HL", "IXH", "tsmc0", "mem0"
	Width int    // bits: 8, 16
	Kind  LocKind
	Alias LocSet // overlapping locations (sub-register aliasing)
}

type LocKind int

const (
	LocGPR       LocKind = iota // L1: general-purpose register (A,B,C,D,E,H,L)
	LocAcc                      // L1: accumulator (ALU destination)
	LocPair                     // L1: register pair (BC,DE,HL)
	LocIndex                    // L1: index register (IX,IY) — DD/FD prefix
	LocFlag                     // L1: CPU flags
	LocIXHalf                   // L2: IX/IY half registers (IXH,IXL,IYH,IYL)
	LocShadow                   // L3: shadow registers (B',C',D',E',H',L' via EXX)
	LocShadowAcc                // L3: shadow accumulator (A' via EX AF,AF')
	LocTSMC                     // L4: TSMC self-modifying code spill slot
	LocMem                      // L5: memory spill slot (absolute address)
	LocStack                    // L6: stack slot (PUSH/POP)
)

// ── Machine Descriptor ───────────────────────────────────────────────────────

type MachineDesc struct {
	Name           string           // "z80"
	WordSize       int              // default operand width in bits (8 for Z80)
	Locs           []Loc            // all physical locations (index = bit in LocSet)
	LocCost        []int            // per-loc access cost in T-states
	Patterns       []Pattern        // all instruction patterns
	Rules          []ConstraintRule // encoding constraint rules
	NonAllocatable LocSet           // locs excluded from general vreg storage (F, SP, etc.)
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

// LocsOfWidth returns a LocSet of all locations with the given width.
func (m *MachineDesc) LocsOfWidth(w int) LocSet {
	var s LocSet
	for i, l := range m.Locs {
		if l.Width == w {
			s = s.Set(i)
		}
	}
	return s
}

// LocsOfKind returns a LocSet of all locations with the given kind.
func (m *MachineDesc) LocsOfKind(k LocKind) LocSet {
	var s LocSet
	for i, l := range m.Locs {
		if l.Kind == k {
			s = s.Set(i)
		}
	}
	return s
}

// CheapestIn returns the index of the cheapest location in the set, or -1.
func (m *MachineDesc) CheapestIn(s LocSet) int {
	best := -1
	bestCost := int(^uint(0) >> 1) // MaxInt
	s.ForEach(func(i int) bool {
		if i < len(m.LocCost) && m.LocCost[i] < bestCost {
			bestCost = m.LocCost[i]
			best = i
		}
		return true
	})
	return best
}

// ── Pattern ──────────────────────────────────────────────────────────────────
// A Pattern maps one VIROp to a concrete Z80 instruction (or sequence).
// The solver sees ALL matching patterns for each VIROp simultaneously and
// picks the best {pattern, register} combination.

type Pattern struct {
	Name     string    // "add_a_r", "ld_r_n", "sbc_hl_rr"
	Op       Op        // which VIR opcode this implements
	Width    int       // operand width (0 = any)
	DstLocs  LocSet    // allowed dst physical locations
	SrcLocs  [2]LocSet // allowed src physical locations
	Template string    // asm template: "ADD A, {src1}", "LD {dst}, {imm}"
	Cost     int       // execution cost in T-states
	Bytes    int       // encoded instruction size
	Clobbers LocSet    // registers clobbered (side effects)
	Flags    PatFlags

	// Guard constrains matching beyond Op/Width.
	// ImmGuard: if non-nil, only match when op.Imm == *ImmGuard (for INC=+1, DEC=-1)
	// SelfSrc: if true, only match when op.Src[0] == op.Src[1] (for x+x double)
	ImmGuard *int64
	SelfSrc  bool

	// TiedDstSrc: if true, dst is tied to src0 — same physical register.
	// Z80 example: ADD A, r → A = A + r → dst tied to src0, both must be A.
	TiedDstSrc bool
}

type PatFlags uint32

const (
	PatCommutative PatFlags = 1 << iota // src0 and src1 are interchangeable
	PatImmediate                        // uses Imm field (not src1)
	PatMemRead                          // reads memory
	PatMemWrite                         // writes memory
	PatBranch                           // conditional branch
	PatCall                             // function call
)

// Matches returns true if this pattern can implement the given VIROp.
func (p *Pattern) Matches(op VIROp) bool {
	if p.Op != op.Op {
		return false
	}
	if p.Width != 0 && p.Width != op.Width {
		return false
	}
	if p.ImmGuard != nil && op.Imm != *p.ImmGuard {
		return false
	}
	if p.SelfSrc && (op.Src[0] != op.Src[1] || op.Src[0] < 0) {
		return false
	}
	return true
}

func immGuard(v int64) *int64 { return &v }

// ── Register Pair Aliasing ───────────────────────────────────────────────────

// pairAlias defines register pair → component aliasing.
// If a 16-bit vreg is at pair, no 8-bit vreg can be at hi or lo.
type pairAlias struct {
	pair, hi, lo int
}

// Z80 pair aliasing: BC(7)=B(1)+C(2), DE(8)=D(3)+E(4), HL(9)=H(5)+L(6)
var pairAliases = []pairAlias{
	{7, 1, 2}, // BC = B + C
	{8, 3, 4}, // DE = D + E
	{9, 5, 6}, // HL = H + L
}

// ── Constraint Rule ──────────────────────────────────────────────────────────
// Encoding constraints that forbid or penalize certain {pattern, register}
// combinations. Replaces text-level fixups.

type ConstraintRule struct {
	Name   string // "dd_prefix_conflict", "pair_only_add"
	DstSet LocSet // applies when dst in DstSet
	SrcSet LocSet // applies when src in SrcSet
	OpMask Op     // 0 = all ops; nonzero = specific op
	Cost   int    // additional cost (MaxCost = forbidden)
}

const MaxCost = 1 << 30 // impossible/forbidden combination

// ── PIROp ────────────────────────────────────────────────────────────────────
// A physical IR instruction: concrete pattern + physical register assignments.
// PIROp → ASM is trivial template expansion. No fixups needed.

type PIROp struct {
	Pat     *Pattern // selected pattern (nil for meta ops like labels)
	DstPhys int      // physical location index for dst (-1 = none)
	SrcPhys [2]int   // physical location indices for srcs (-1 = unused)
	Imm     int64    // immediate value
	Sym     string   // symbol name
	Comment string   // debug annotation (optional)

	// Inline asm (OpAsmBlock): emit verbatim, bypass pattern template
	AsmText string
}

// Emit expands this PIROp into assembly text using the pattern template.
func (p *PIROp) Emit(m *MachineDesc) string {
	// Inline asm: emit verbatim, split on "/" or newlines.
	// Apply DD/FD prefix conflict fix per-line (inline asm may contain LD IXH,H).
	if p.AsmText != "" {
		// Substitute {dst}/{src0}/{src1}/{imm} before splitting.
		text := p.AsmText
		if p.DstPhys >= 0 && p.DstPhys < len(m.Locs) {
			text = replaceAll(text, "{dst}", m.Locs[p.DstPhys].Name)
		}
		for i, ph := range p.SrcPhys {
			tag := "{src" + string(rune('0'+i)) + "}"
			if ph >= 0 && ph < len(m.Locs) {
				text = replaceAll(text, tag, m.Locs[ph].Name)
			}
		}
		if p.Imm != 0 {
			text = replaceAll(text, "{imm}", fmt.Sprintf("%d", p.Imm))
		}
		// Split on "/" first (compact format), then on newlines (multi-line)
		parts := splitAsm(text)
		var allLines []string
		for _, part := range parts {
			for _, line := range strings.Split(part, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					allLines = append(allLines, line)
				}
			}
		}
		var sb strings.Builder
		for i, line := range allLines {
			if i > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString("    ")
			sb.WriteString(fixDDPrefixConflict(line))
		}
		return sb.String()
	}

	if p.Pat == nil {
		if p.Comment != "" {
			return "; " + p.Comment
		}
		return ""
	}

	s := p.Pat.Template

	// Replace {dst}, {src0}, {src1}, {imm}
	if p.Sym != "" && p.Pat.Op == OpConst && p.DstPhys >= 0 && p.DstPhys < len(m.Locs) {
		dst := m.Locs[p.DstPhys].Name
		if dst == "IX" || dst == "IY" {
			sym := sanitizeSym(p.Sym)
			return "    LD HL, " + sym + "\n    PUSH HL\n    POP " + dst
		}
	}
	if p.DstPhys >= 0 && p.DstPhys < len(m.Locs) {
		s = replaceAll(s, "{dst}", m.Locs[p.DstPhys].Name)
	}
	for i, ph := range p.SrcPhys {
		tag := "{src" + string(rune('0'+i)) + "}"
		if ph >= 0 && ph < len(m.Locs) {
			s = replaceAll(s, tag, m.Locs[ph].Name)
		}
	}

	if p.Sym != "" {
		s = replaceAll(s, "{imm}", sanitizeSym(p.Sym))
	} else {
		s = replaceAll(s, "{imm}", formatImm(p.Imm))
	}

	// Fix DD/FD prefix conflict: LD IXH,H is impossible because under DD
	// prefix H/L are reinterpreted as IXH/IXL. Route through A instead.
	s = fixDDPrefixConflict(s)

	return s
}

// fixDDPrefixConflict detects LD instructions where src/dst mix IXH/IXL with H/L.
// Under DD prefix, H→IXH and L→IXL, so LD IXH,H becomes LD IXH,IXH (no-op).
// Fix: route through A (LD A,H / LD IXH,A) or (LD A,IXH / LD H,A).
func fixDDPrefixConflict(s string) string {
	// Map of invalid combos → replacement (route through A)
	conflicts := [...]struct{ bad, fix string }{
		{"LD IXH, H", "LD A, H\n    LD IXH, A"},
		{"LD IXH, L", "LD A, L\n    LD IXH, A"},
		{"LD IXL, H", "LD A, H\n    LD IXL, A"},
		{"LD IXL, L", "LD A, L\n    LD IXL, A"},
		{"LD H, IXH", "LD A, IXH\n    LD H, A"},
		{"LD H, IXL", "LD A, IXL\n    LD H, A"},
		{"LD L, IXH", "LD A, IXH\n    LD L, A"},
		{"LD L, IXL", "LD A, IXL\n    LD L, A"},
		// IYH/IYL have the same FD prefix conflict
		{"LD IYH, H", "LD A, H\n    LD IYH, A"},
		{"LD IYH, L", "LD A, L\n    LD IYH, A"},
		{"LD IYL, H", "LD A, H\n    LD IYL, A"},
		{"LD IYL, L", "LD A, L\n    LD IYL, A"},
		{"LD H, IYH", "LD A, IYH\n    LD H, A"},
		{"LD H, IYL", "LD A, IYL\n    LD H, A"},
		{"LD L, IYH", "LD A, IYH\n    LD L, A"},
		{"LD L, IYL", "LD A, IYL\n    LD L, A"},
		// Cross-family IX/IY half-register moves are not directly encodable.
		{"LD IXH, IYH", "LD A, IYH\n    LD IXH, A"},
		{"LD IXH, IYL", "LD A, IYL\n    LD IXH, A"},
		{"LD IXL, IYH", "LD A, IYH\n    LD IXL, A"},
		{"LD IXL, IYL", "LD A, IYL\n    LD IXL, A"},
		{"LD IYH, IXH", "LD A, IXH\n    LD IYH, A"},
		{"LD IYH, IXL", "LD A, IXL\n    LD IYH, A"},
		{"LD IYL, IXH", "LD A, IXH\n    LD IYL, A"},
		{"LD IYL, IXL", "LD A, IXL\n    LD IYL, A"},
	}
	for _, c := range conflicts {
		s = replaceAll(s, c.bad, c.fix)
	}
	// Prefer legal pair moves over IX/IY half-register sequences when possible.
	s = replaceAll(s, "LD A, H\n    LD IYH, A\n    LD A, L\n    LD IYL, A", "PUSH HL\n    POP IY")
	s = replaceAll(s, "LD A, H\n    LD IXH, A\n    LD A, L\n    LD IXL, A", "PUSH HL\n    POP IX")
	s = replaceAll(s, "LD A, D\n    LD IYH, A\n    LD A, E\n    LD IYL, A", "PUSH DE\n    POP IY")
	s = replaceAll(s, "LD A, D\n    LD IXH, A\n    LD A, E\n    LD IXL, A", "PUSH DE\n    POP IX")
	s = replaceAll(s, "LD A, B\n    LD IYH, A\n    LD A, C\n    LD IYL, A", "PUSH BC\n    POP IY")
	s = replaceAll(s, "LD A, B\n    LD IXH, A\n    LD A, C\n    LD IXL, A", "PUSH BC\n    POP IX")

	// Fix illegal narrow→index-pair loads that can still leak from some
	// late VIR copy paths. Z80 has no "LD IY, C"; materialize as u8→u16
	// zero-extension instead.
	for _, r := range []string{"A", "B", "C", "D", "E", "H", "L", "IXH", "IXL", "IYH", "IYL"} {
		s = replaceAll(s, "LD IX, "+r, "LD IXL, "+r+"\n    LD IXH, 0")
		s = replaceAll(s, "LD IY, "+r, "LD IYL, "+r+"\n    LD IYH, 0")
	}
	return s
}

func replaceAll(s, old, new string) string {
	for {
		i := indexOf(s, old)
		if i < 0 {
			return s
		}
		s = s[:i] + new + s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// sanitizeSym converts MIR2 symbol names to assembler-safe labels.
// e.g. @mir2.str.0 → _mir2_str_0, fs$fat12$read → fs_fat12_read
func sanitizeSym(s string) string {
	s = strings.ReplaceAll(s, "@", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, "$", "_")
	return s
}

// splitAsm splits an asm template on "/" separators, trimming whitespace.
func splitAsm(tmpl string) []string {
	parts := strings.Split(tmpl, "/")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return []string{tmpl}
	}
	return result
}

func formatImm(v int64) string {
	if v < 0 {
		return "-" + formatImm(-v)
	}
	if v <= 9 {
		return string(rune('0' + v))
	}
	// Simple decimal formatting
	result := ""
	for v > 0 {
		result = string(rune('0'+v%10)) + result
		v /= 10
	}
	return result
}

// ── Block ────────────────────────────────────────────────────────────────────

type Block struct {
	Label  string
	Ops    []VIROp // input: virtual ops
	PIR    []PIROp // output: physical ops (filled by solver)
	Params []int   // block parameter vreg IDs (PHI destinations)
}

// ── Func ─────────────────────────────────────────────────────────────────────

type Func struct {
	Name    string
	Blocks  []Block
	MIRFunc interface{} // *mir2.Func back-reference for CFG queries (avoid import cycle: use interface{})
}
