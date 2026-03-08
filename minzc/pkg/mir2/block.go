package mir2

// Block is a basic block: a maximal straight-line sequence of instructions
// with a single entry and a single terminator (exit).
//
// # Block arguments (not phi nodes)
//
// MIR2 uses Cranelift/MLIR-style block arguments instead of LLVM phi nodes.
// Each block declares typed parameters; branches supply values for those
// parameters explicitly in their terminator instruction.
//
// Example — a counted loop:
//
//	block @loop_head(%r3: u16 [pointer], %r4: u16 [index], %r5: u8 [counter]):
//	  %r6 = cmp.gt %r5, %r1 : bool [flag]
//	  br_if %r6, @exit(), @loop_body(%r3, %r4, %r5)
//
// The block's Params are the formal parameters (define registers %r3…%r5).
// The terminator's Args are the actual values supplied by each predecessor.
//
// Why block args instead of phis?
//   - No "simultaneous execution" semantics to model specially
//   - Lowering to parallel copies is localised: one place per outgoing edge
//   - VM interpreter: branch args ≡ "push these values then jump"
//   - CFG rewrites never need to update phi operands in other blocks
//   - Verifier checks are uniform: arity + type match per edge
//
// Invariants (enforced by Verify):
//   - Label is unique within the parent Func.
//   - Term is non-nil before the function is verified.
//   - Every Reg referenced in Insts/Term is either a Param reg, a
//     function-level param reg, or the Dst of an earlier instruction.
type Block struct {
	Label  string       // unique identifier; branch target in terminators
	Params []BlockParam // formal block arguments (define registers on entry)
	Insts  []*Inst      // body instructions in program order
	Term   Term         // terminator; nil = unsealed
}

// BlockParam is one formal parameter of a block.
// The register Dst is defined on entry from each predecessor's branch args.
type BlockParam struct {
	Dst   Reg      // virtual register holding this value on block entry
	Ty    Ty       // type of the value
	Class RegClass // preferred register class (soft — see regs.go)
}

// Append adds an instruction to the end of the block body.
// Panics if the block is already sealed.
func (b *Block) Append(inst *Inst) {
	if b.Term != nil {
		panic("mir2: appending instruction to sealed block @" + b.Label)
	}
	b.Insts = append(b.Insts, inst)
}

// Seal sets the block's terminator. After Seal, Append panics.
// Panics if Seal is called twice.
func (b *Block) Seal(term Term) {
	if b.Term != nil {
		panic("mir2: block @" + b.Label + " is already sealed")
	}
	b.Term = term
}

// IsSealed reports whether the block has a terminator.
func (b *Block) IsSealed() bool { return b.Term != nil }

// Successors returns the labels of all successor blocks.
func (b *Block) Successors() []string {
	if b.Term == nil {
		return nil
	}
	return b.Term.Successors()
}

// Defs returns the set of Reg values defined by this block:
// block params + instruction dsts.
func (b *Block) Defs() []Reg {
	out := make([]Reg, 0, len(b.Params)+len(b.Insts))
	for _, p := range b.Params {
		out = append(out, p.Dst)
	}
	for _, inst := range b.Insts {
		if inst.Dst != NoReg {
			out = append(out, inst.Dst)
		}
	}
	return out
}
