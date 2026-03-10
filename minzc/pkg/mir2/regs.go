package mir2

// Reg is a virtual register number.
//
//   0  (NoReg) — no result; used for void or side-effecting instructions
//   1+ — virtual registers allocated monotonically per function
type Reg int

const NoReg Reg = 0

// ── Register classes ──────────────────────────────────────────────────────────

// RegClass is a semantic constraint on physical register allocation.
//
// Classes form a soft preference hierarchy.  The allocator assigns a cost
// vector over all physical locations for each class; it picks the minimum-cost
// assignment globally (PBQP-style).  ClsHard on Inst overrides to a hard
// constraint (must satisfy or spill).
//
// # Z80 physical mapping and spill cost hierarchy
//
//	Tier 0 — primary registers (cost 0–4T)
//	  ClassAcc     → A
//	  ClassCounter → B  (required for DJNZ; soft otherwise)
//	  ClassGeneral → A, B, C, D, E, H, L  (any 8-bit)
//	  ClassPointer → HL  (preferred for LD/INC/DEC; soft otherwise)
//	  ClassIndex   → DE  (preferred for LDIR source; soft otherwise)
//	  ClassPair    → HL, DE, BC  (any 16-bit pair)
//	  ClassDWord   → HL+H'L', DE+D'E', BC+B'C'  (32-bit via EXX; ~8T per op)
//
//	Tier 1 — index registers (cost +8T per instr, DD/FD prefix)
//	  ClassIX      → IX  (16-bit; (IX+d) addressing)
//	  ClassIY      → IY  (16-bit; (IY+d) addressing)
//	  ClassIXY8    → IXH, IXL, IYH, IYL  (undocumented 8-bit halves)
//	                 Available on NMOS/CMOS Z80 and clones.
//	                 NOT available on SM83 (Game Boy).
//	                 Spill cost: 16T (8T save + 8T restore) vs 26T for memory.
//
//	Tier 2 — shadow registers (cost ~8T per region: EXX in + EXX out)
//	  ClassShadow  → B', C', D', E', H', L'  (via EXX)
//	  ClassAccShadow → A'  (via EX AF,AF')
//	  Useful for nested interrupt handlers or leaf functions with known EXX usage.
//
//	Tier 3 — stack (cost 21T = PUSH 11T + POP 10T)
//	  ClassStack   → stack slot (PUSH/POP); backend emits frame slot
//
//	Tier 4 — memory (cost 26–32T; current MinZ default — the bottleneck)
//	  ClassMem     → absolute $F0xx slot (LD (nn),r / LD r,(nn))
//
//	Special:
//	  ClassFlag    → CPU flag (CY/Z/NZ/NC); bool only; no materialisation
//
// On register-rich targets (x86-64, ARM) Tier 0–2 all collapse to ClassGeneral.
// The allocator's cost table is target-provided; MIR2 only defines the class names.
type RegClass uint8

const (
	// ── Tier 0: primary ────────────────────────────────────────────────────────
	ClassGeneral RegClass = iota // any general-purpose register (8-bit on Z80)
	ClassAcc                     // accumulator: Z80=A, x86=EAX, ARM=R0
	ClassCounter                 // loop counter: Z80=B (DJNZ), x86=ECX
	ClassPointer                 // memory pointer: Z80=HL, x86/ARM=any GPR
	ClassIndex                   // secondary pointer: Z80=DE, x86/ARM=any GPR
	ClassPair                    // any 16-bit pair: Z80=HL/DE/BC
	ClassDWord                  // 32-bit via shadow pair: Z80=HL+H'L' / DE+D'E' / BC+B'C'

	// ── Tier 1: index registers ────────────────────────────────────────────────
	ClassIX   // Z80=IX (16-bit); spill cost ~16T vs 32T for $F0xx
	ClassIY   // Z80=IY (16-bit); same cost as IX
	ClassIXY8 // Z80=IXH/IXL/IYH/IYL (undocumented 8-bit halves); cost ~16T

	// ── Tier 2: shadow registers ───────────────────────────────────────────────
	ClassShadow    // Z80=B',C',D',E',H',L' via EXX; cost ~8T per protected region
	ClassAccShadow // Z80=A' via EX AF,AF'

	// ── Tier 3: stack ──────────────────────────────────────────────────────────
	ClassStack // PUSH/POP slot; cost 21T per save+restore

	// ── Tier 4: memory ─────────────────────────────────────────────────────────
	ClassMem // absolute memory slot ($F0xx on Z80); cost 26–32T

	// ── Special ────────────────────────────────────────────────────────────────
	ClassFlag // CPU flag (CY/Z on Z80, EFLAGS on x86, CPSR on ARM); bool only

	classCount // sentinel
)

func (c RegClass) String() string {
	if int(c) < len(classNames) {
		return classNames[c]
	}
	return "?class"
}

var classNames = [classCount]string{
	ClassGeneral:   "general",
	ClassAcc:       "acc",
	ClassCounter:   "counter",
	ClassPointer:   "pointer",
	ClassIndex:     "index",
	ClassPair:      "pair",
	ClassDWord:     "dword",
	ClassIX:        "ix",
	ClassIY:        "iy",
	ClassIXY8:      "ixy8",
	ClassShadow:    "shadow",
	ClassAccShadow: "acc'",
	ClassStack:     "stack",
	ClassMem:       "mem",
	ClassFlag:      "flag",
}

// Tier reports the spill cost tier of the class (0=cheapest, 4=most expensive).
func (c RegClass) Tier() int {
	switch c {
	case ClassGeneral, ClassAcc, ClassCounter, ClassPointer, ClassIndex, ClassPair, ClassDWord:
		return 0
	case ClassIX, ClassIY, ClassIXY8:
		return 1
	case ClassShadow, ClassAccShadow:
		return 2
	case ClassStack:
		return 3
	case ClassMem:
		return 4
	case ClassFlag:
		return 0 // flags are free — they're already set by cmp/sub
	}
	return 4
}

// IsPrimary reports whether the class is Tier 0 (no spill overhead on Z80).
func (c RegClass) IsPrimary() bool { return c.Tier() == 0 }

// ── ClassSet ──────────────────────────────────────────────────────────────────

// ClassSet is a bitset of RegClass values (fits in uint32 with current classCount=14).
type ClassSet uint32

func (s ClassSet) Has(c RegClass) bool        { return s&(1<<c) != 0 }
func (s *ClassSet) Add(c RegClass)             { *s |= 1 << c }
func (s *ClassSet) Remove(c RegClass)          { *s &^= 1 << c }
func (s ClassSet) Union(o ClassSet) ClassSet   { return s | o }
func (s ClassSet) Intersect(o ClassSet) ClassSet { return s & o }
func (s ClassSet) Empty() bool                 { return s == 0 }

// AllClasses contains every defined register class.
var AllClasses = ClassSet((1 << classCount) - 1)

// PrimaryClasses contains only Tier-0 classes.
var PrimaryClasses = func() ClassSet {
	var s ClassSet
	for c := RegClass(0); c < classCount; c++ {
		if c.IsPrimary() {
			s.Add(c)
		}
	}
	return s
}()

// ── Default class inference ───────────────────────────────────────────────────

// ClassOf returns the natural default class for a type:
//
//	bool → ClassFlag    (use CPU flag when possible)
//	ptr  → ClassPointer
//	u32  → ClassDWord   (32-bit via shadow pair on Z80)
//	i32  → ClassDWord
//	else → ClassGeneral
func ClassOf(ty Ty) RegClass {
	if ty == TyBool {
		return ClassFlag
	}
	if ty == TyPtr {
		return ClassPointer
	}
	if ty == TyU32 || ty == TyI32 || ty == TyU24 || ty == TyI24 {
		// Z80: all 24/32-bit values use HL+H'L' shadow pair (ClassDWord).
		// u24: upper byte of shadow pair is always zero (3-byte load/store).
		// eZ80: u24 will use native 24-bit ADL registers in a future backend.
		return ClassDWord
	}
	return ClassGeneral
}

// ── PBQP cost model (target-provided) ────────────────────────────────────────

// CostVector maps every physical location for one virtual register to a cost.
// Costs are in abstract "units" — the allocator minimises total cost.
// Convention (for Z80):
//
//	0        = perfect fit (e.g. B for [counter])
//	2–4      = minor overhead (different reg, same tier)
//	6–8      = index register or shadow (one prefix or EXX pair)
//	16–21    = stack PUSH/POP
//	26–32    = memory $F0xx
//	InfCost  = physically impossible (e.g. A for [pointer])
const InfCost = 1<<30 - 1

// PhysLoc is one physical storage location for a value.
// The allocator's output is a map[Reg]PhysLoc.
type PhysLoc struct {
	Kind    LocKind
	Name    string // e.g. "B", "HL", "IX", "IXH", "$F001"
	Offset  int    // for stack slots: frame offset; for memory: absolute address
}

// LocKind classifies a physical location by kind.
type LocKind uint8

const (
	LocReg    LocKind = iota // physical CPU register
	LocIXY                   // IX or IY (with optional +offset addressing)
	LocIXY8                  // IXH/IXL/IYH/IYL (undocumented halves)
	LocShadow                // shadow register (B',C',… via EXX)
	LocStack                 // PUSH/POP stack slot
	LocMem                   // absolute memory address ($F0xx)
	LocFlag                  // CPU flag register
	LocDWord                 // 32-bit shadow pair: main rr + shadow rr' (via EXX)
)

// CostTable maps (RegClass, PhysLoc) → cost.
// Backends provide a concrete CostTable; the allocator uses it for PBQP.
type CostTable interface {
	// Cost returns the cost of assigning a virtual register with preferred
	// class cls to physical location loc.
	Cost(cls RegClass, loc PhysLoc) int

	// Locs returns all physical locations available on this target.
	Locs() []PhysLoc
}
