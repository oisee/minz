package mir2

// Func is a MIR2 function.
//
// A function is a list of basic blocks. The first block is the entry.
// Virtual register numbering is monotonically increasing.  When created via
// Module.AddFunc the counter is module-wide (all funcs share unique IDs).
type Func struct {
	Name     string
	Contract Contract  // calling ABI in target-independent terms
	Attrs    FuncAttrs // boolean property flags
	Blocks   []*Block  // first block = entry (label conventionally "entry")

	nextReg Reg     // next available virtual register
	mod     *Module // parent module; non-nil when created via AddFunc
}

// ── Contract ──────────────────────────────────────────────────────────────────

// Contract describes a function's calling ABI using register classes
// instead of physical register names.  Backends map classes to physical
// resources for each target.
type Contract struct {
	Params   []Param  // ordered parameter slots
	Returns  []Return // ordered return value slots (multi-return = len > 1)
	Clobbers ClassSet // register classes modified by this function (caller-save)
}

// Param is one parameter: a named, typed, class-constrained value that arrives
// in a specific virtual register at function entry.
type Param struct {
	Name  string   // source-level parameter name (for diagnostics)
	Reg   Reg      // virtual register holding this param on entry
	Ty    Ty       // type of the parameter
	Class RegClass // required register class
}

// Return is one return value slot (one element of a multi-return tuple).
//
// When Class == ClassFlag the return value is communicated via the CPU flag
// register rather than a general-purpose register.  FlagCond specifies which
// Z80 condition code means "true":
//
//	CmpLt  → C flag set   (e.g. "return a < b" after CP; no register needed)
//	CmpEq  → Z flag set   (e.g. "return a == b")
//	CmpNe  → NZ (Z clear) (e.g. "return a != b")
//	CmpGe  → NC (C clear)
//
// Callers test with JP <FlagCond_as_cc>, eliminating AND A / CP 0.
// Materialization to a register uses SBC A,A (for C-based) or JR+SCF (for Z-based).
type Return struct {
	Ty       Ty
	Class    RegClass
	FlagCond CmpCond // only meaningful when Class == ClassFlag
}

// ── Attributes ────────────────────────────────────────────────────────────────

// FuncAttrs are boolean flags on a function.
// IsLeaf and IsRecursive are computed by analysis; the rest are set during
// code generation from the source AST.
type FuncAttrs struct {
	// IsExtern: declaration only (no body blocks).
	// The function is defined elsewhere (ROM routine, stdlib, FFI).
	IsExtern bool

	// ExternAddr: if non-zero, the function lives at a fixed address.
	// Z80 codegen emits RST n (if addr is 0x00/0x08/0x10/0x18/0x20/0x28/0x30/0x38)
	// or CALL addr otherwise, instead of CALL sym.
	ExternAddr uint16

	// IsComptime: the function is an @-function that can be evaluated at
	// compile time on the MIR2 VM.  Callers with known arguments may fold
	// the call to a constant.
	IsComptime bool

	// HasAsm: set by the lowerer when the function contains inline asm.
	// Functions with asm have hard register expectations that OptimizeContracts
	// must not change.
	HasAsm bool

	// IsLeaf: computed — no OpCall instructions appear in the body.
	// Leaf functions have precise clobber sets without conservative widening.
	IsLeaf bool

	// IsRecursive: computed — the function calls itself (directly or transitively).
	// Recursive functions cannot use absolute $F0xx locals on Z80 — they need
	// a stack frame.
	IsRecursive bool
}

// ── Construction helpers ──────────────────────────────────────────────────────

// Entry returns the entry block (first block), or nil if the function has no blocks.
func (f *Func) Entry() *Block {
	if len(f.Blocks) == 0 {
		return nil
	}
	return f.Blocks[0]
}

// NewBlock appends a new, unsealed (no terminator) block and returns it.
// The caller is responsible for sealing the block before the function is verified.
func (f *Func) NewBlock(label string) *Block {
	b := &Block{Label: label}
	f.Blocks = append(f.Blocks, b)
	return b
}

// AllocReg returns the next available virtual register number and advances
// the counter.  Register 0 (NoReg) is never returned.
// If this Func was created via Module.AddFunc, the module-wide counter is
// kept in sync so subsequent functions start above this function's regs.
func (f *Func) AllocReg() Reg {
	if f.nextReg < 1 {
		f.nextReg = 1
	}
	r := f.nextReg
	f.nextReg++
	// Keep module counter in sync so the next AddFunc starts past our regs.
	if f.mod != nil && f.nextReg > f.mod.nextReg {
		f.mod.nextReg = f.nextReg
	}
	return r
}

// NextRegNum returns the next register that would be allocated (without allocating).
func (f *Func) NextRegNum() Reg { return f.nextReg }

// BlockByLabel finds a block by label, returning nil if not found.
func (f *Func) BlockByLabel(label string) *Block {
	for _, b := range f.Blocks {
		if b.Label == label {
			return b
		}
	}
	return nil
}

// Sealed reports whether every block in the function has a terminator.
func (f *Func) Sealed() bool {
	for _, b := range f.Blocks {
		if !b.IsSealed() {
			return false
		}
	}
	return true
}
