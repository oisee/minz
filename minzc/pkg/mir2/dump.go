package mir2

import (
	"fmt"
	"strings"
)

// Dump returns the full textual representation of the module.
//
// Example:
//
//	; MIR2 module: fib
//
//	fun @fibonacci(%r1: u8 [acc]) -> u16 [pointer]  clobbers {acc, counter, pointer, index}
//	  block @entry:
//	    %r2 = const 2 : u8 [general]
//	    %r3 = cmp.lt %r1, %r2 : bool [flag]
//	    br_if %r3, @base(), @loop_init()
//	  block @loop_init:
//	    jmp @loop_head(%r4, %r5, %r6)
//	  block @loop_head(%r7: u16 [pointer], %r8: u16 [index], %r9: u8 [counter]):
//	    ...
func (m *Module) Dump() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "; MIR2 module: %s\n", m.Name)

	for i := 0; i < m.Strings.Len(); i++ {
		fmt.Fprintf(&sb, "string %s = %q\n", m.Strings.Symbol(i), m.Strings.At(i))
	}
	for _, g := range m.Globals {
		qual := ""
		if g.IsConst {
			qual = "const "
		}
		fmt.Fprintf(&sb, "global %s@%s : %s\n", qual, g.Name, g.Ty.String())
	}
	if m.Strings.Len() > 0 || len(m.Globals) > 0 {
		sb.WriteByte('\n')
	}
	for _, f := range m.Funcs {
		dumpFunc(&sb, f)
	}
	return sb.String()
}

func dumpFunc(sb *strings.Builder, f *Func) {
	if f.Attrs.IsExtern {
		sb.WriteString("extern ")
	}
	if f.Attrs.IsComptime {
		sb.WriteString("[comptime] ")
	}
	sb.WriteString("fun @")
	sb.WriteString(f.Name)
	sb.WriteString("(")
	for i, p := range f.Contract.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(sb, "%%r%d: %s [%s]", p.Reg, p.Ty, p.Class)
	}
	sb.WriteString(")")

	rets := f.Contract.Returns
	if len(rets) > 0 {
		sb.WriteString(" -> ")
		if len(rets) == 1 {
			fmt.Fprintf(sb, "%s [%s]", rets[0].Ty, rets[0].Class)
		} else {
			sb.WriteString("(")
			for i, r := range rets {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(sb, "%s [%s]", r.Ty, r.Class)
			}
			sb.WriteString(")")
		}
	}

	if !f.Contract.Clobbers.Empty() {
		sb.WriteString("  clobbers {")
		first := true
		for c := RegClass(0); c <= ClassFlag; c++ {
			if f.Contract.Clobbers.Has(c) {
				if !first {
					sb.WriteString(", ")
				}
				sb.WriteString(c.String())
				first = false
			}
		}
		sb.WriteString("}")
	}

	if f.Attrs.IsExtern {
		sb.WriteString("\n\n")
		return
	}
	sb.WriteString("\n")
	for _, blk := range f.Blocks {
		dumpBlock(sb, blk)
	}
	sb.WriteString("\n")
}

func dumpBlock(sb *strings.Builder, blk *Block) {
	// Block header with params
	sb.WriteString("  block @")
	sb.WriteString(blk.Label)
	if len(blk.Params) > 0 {
		sb.WriteString("(")
		for i, p := range blk.Params {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(sb, "%%r%d: %s [%s]", p.Dst, p.Ty, p.Class)
		}
		sb.WriteString(")")
	}
	sb.WriteString(":\n")

	for _, inst := range blk.Insts {
		fmt.Fprintf(sb, "    %s\n", DumpInst(inst))
	}
	if blk.Term != nil {
		fmt.Fprintf(sb, "    %s\n", TermString(blk.Term))
	} else {
		sb.WriteString("    ; (unsealed)\n")
	}
}

// DumpInst returns the text form of a single instruction.
func DumpInst(inst *Inst) string {
	var sb strings.Builder

	if inst.Dst != NoReg {
		fmt.Fprintf(&sb, "%%r%d = ", inst.Dst)
	}

	clsSuffix := func() string {
		hard := ""
		if inst.ClsHard {
			hard = "!"
		}
		return fmt.Sprintf(" [%s%s]", inst.Cls, hard)
	}

	switch inst.Op {
	case OpCmp:
		fmt.Fprintf(&sb, "cmp.%s %%r%d, %%r%d : bool%s",
			inst.Cond, inst.Src[0], inst.Src[1], clsSuffix())

	case OpConst:
		fmt.Fprintf(&sb, "const %d : %s%s", inst.Imm, inst.Ty, clsSuffix())

	case OpAddrOf:
		fmt.Fprintf(&sb, "addr_of @%s : ptr%s", inst.Sym, clsSuffix())

	case OpField:
		fmt.Fprintf(&sb, "field %%r%d, +%d : ptr%s", inst.Src[0], inst.Imm, clsSuffix())

	case OpPtrBump:
		fmt.Fprintf(&sb, "ptr_bump %%r%d, +%d : ptr%s", inst.Src[0], inst.Imm, clsSuffix())

	case OpPtrAdd:
		fmt.Fprintf(&sb, "ptr_add %%r%d, %%r%d : ptr%s", inst.Src[0], inst.Src[1], clsSuffix())

	case OpAlloca:
		fmt.Fprintf(&sb, "alloca %d : ptr", inst.Imm)

	case OpStore:
		fmt.Fprintf(&sb, "store %%r%d, %%r%d : %s", inst.Src[0], inst.Src[1], inst.Ty)

	case OpPatch:
		fmt.Fprintf(&sb, "patch %%r%d, %%r%d", inst.Src[0], inst.Src[1])

	case OpCall:
		fmt.Fprintf(&sb, "call @%s(%s)", inst.Sym, regList(inst.Args))
		if inst.Ty != TyVoid {
			fmt.Fprintf(&sb, " : %s%s", inst.Ty, clsSuffix())
		}
		dumpCallAttrs(&sb, inst.CallAttr)

	case OpCallIndirect:
		fmt.Fprintf(&sb, "call_indirect %%r%d(%s)", inst.Src[0], regList(inst.Args))
		if inst.Ty != TyVoid {
			fmt.Fprintf(&sb, " : %s%s", inst.Ty, clsSuffix())
		}
		dumpCallAttrs(&sb, inst.CallAttr)

	case OpPatchSlot:
		fmt.Fprintf(&sb, "patch_slot %d : %s%s", inst.Imm, inst.Ty, clsSuffix())

	case OpLoadPatched:
		fmt.Fprintf(&sb, "load_patched %%r%d : %s%s", inst.Src[0], inst.Ty, clsSuffix())

	case OpExt, OpSext, OpTrunc:
		srcTy := ""
		if inst.SrcTy != nil {
			srcTy = inst.SrcTy.String() + " -> "
		}
		fmt.Fprintf(&sb, "%s %%r%d : %s%s%s", inst.Op, inst.Src[0], srcTy, inst.Ty, clsSuffix())

	case OpAsm:
		if inst.Asm != nil {
			fmt.Fprintf(&sb, "asm %q", inst.Asm.Template)
		} else {
			sb.WriteString("asm <nil>")
		}

	default:
		sb.WriteString(inst.Op.String())
		if inst.Src[1] != NoReg {
			fmt.Fprintf(&sb, " %%r%d, %%r%d : %s%s", inst.Src[0], inst.Src[1], inst.Ty, clsSuffix())
		} else if inst.Src[0] != NoReg {
			fmt.Fprintf(&sb, " %%r%d : %s%s", inst.Src[0], inst.Ty, clsSuffix())
		}
	}

	return sb.String()
}

func dumpCallAttrs(sb *strings.Builder, attr CallAttrs) {
	if attr.IsComptime {
		sb.WriteString(" [comptime]")
	}
	if attr.IsHost {
		sb.WriteString(" [host]")
	}
}
