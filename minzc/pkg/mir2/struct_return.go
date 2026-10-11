package mir2

import (
	"fmt"
	"reflect"
)

// StructReturnBuffer is part of the calling ABI, not a source parameter.
// ParamIndex identifies the appended caller-owned pointer; ReturnIndex retains
// the source value slot. Ty describes object layout, never address width.
type StructReturnBuffer struct {
	ReturnIndex int
	ParamIndex  int
	Ty          *StructTy
}

// LowerStructReturnBuffers supplies owned storage for every struct result.
// It runs once on complete MIR, before optimization or native allocation.
func LowerStructReturnBuffers(m *Module) error {
	for _, f := range m.Funcs {
		if len(f.Contract.StructReturnBuffers) > 0 {
			return fmt.Errorf("struct-return ABI already lowered: %s", f.Name)
		}
		for ri, ret := range f.Contract.Returns {
			ty, ok := BaseOf(ret.Ty).(*StructTy)
			if !ok {
				continue
			}
			if f.Attrs.IsExtern {
				return fmt.Errorf("extern struct return requires explicit caller-buffer ABI: %s", f.Name)
			}
			slot := StructReturnBuffer{ReturnIndex: ri, ParamIndex: len(f.Contract.Params), Ty: ty}
			f.Contract.StructReturnBuffers = append(f.Contract.StructReturnBuffers, slot)
			f.Contract.Params = append(f.Contract.Params, Param{Name: fmt.Sprintf("$result%d", ri), Reg: f.AllocReg(), Ty: TyPtr, Class: ClassPointer})
		}
	}
	for _, f := range m.Funcs {
		var resultAllocas []*Inst
		for _, b := range f.Blocks {
			var out []*Inst
			for _, inst := range b.Insts {
				var slots []StructReturnBuffer
				if inst.Op == OpCall || inst.Op == OpCallCond {
					if callee := m.FuncByName(inst.Sym); callee != nil {
						slots = callee.Contract.StructReturnBuffers
					}
				}
				if inst.Op == OpCallIndirect {
					if ty, ok := BaseOf(inst.Ty).(*StructTy); ok {
						slots = []StructReturnBuffer{{ReturnIndex: 0, ParamIndex: len(inst.Args), Ty: ty}}
					}
				}
				if len(slots) > 0 {
					inst.StructReturnBuffers = append([]StructReturnBuffer(nil), slots...)
					for _, slot := range slots {
						ptr := f.AllocReg()
						n := ByteWidth(slot.Ty)
						if n == 0 {
							n = 1
						}
						resultAllocas = append(resultAllocas, &Inst{Op: OpAlloca, Dst: ptr, Ty: TyPtr, Cls: ClassPointer, Imm: int64(n), SrcLine: inst.SrcLine, SrcFile: inst.SrcFile})
						if slot.ParamIndex != len(inst.Args) {
							return fmt.Errorf("struct call argument ABI mismatch: %s to %s", f.Name, inst.Sym)
						}
						inst.Args = append(inst.Args, ptr)
					}
				}
				out = append(out, inst)
			}
			b.Insts = out
		}
		if len(resultAllocas) > 0 {
			f.Entry().Insts = append(resultAllocas, f.Entry().Insts...)
		}
		if len(f.Contract.StructReturnBuffers) == 0 {
			continue
		}
		// Conditional return must copy only on its return arm. Normalize before
		// inserting accesses; later CondRetSink sees a nontrivial effectful block.
		original := append([]*Block(nil), f.Blocks...)
		for _, b := range original {
			if term, ok := b.Term.(*TermCondRet); ok {
				rb := f.NewBlock(fmt.Sprintf("$struct_ret_%d", len(f.Blocks)))
				rb.Term = &TermRet{Vals: append([]Reg(nil), term.Vals...)}
				b.Term = &TermBrIf{Cond: term.Cond, Then: term.Then, ThenArgs: term.ThenArgs, Else: rb.Label}
			}
		}
		for _, b := range f.Blocks {
			ret, ok := b.Term.(*TermRet)
			if !ok {
				continue
			}
			for _, slot := range f.Contract.StructReturnBuffers {
				if slot.ReturnIndex >= len(ret.Vals) {
					return fmt.Errorf("missing struct return value: %s", f.Name)
				}
				source := ret.Vals[slot.ReturnIndex]
				dest := f.Contract.Params[slot.ParamIndex].Reg
				// Snapshot all bytes before any store, preserving value semantics even
				// when a caller buffer aliases the source object. Ordinary byte access
				// instructions retain their own identities and full observer semantics.
				n := ByteWidth(slot.Ty)
				values := make([]Reg, n)
				address := func(base Reg, off int) Reg {
					if off == 0 {
						return base
					}
					r := f.AllocReg()
					b.Insts = append(b.Insts, &Inst{Op: OpField, Dst: r, Src: [2]Reg{base}, Ty: TyPtr, Cls: ClassPointer, Imm: int64(off)})
					return r
				}
				for i := 0; i < n; i++ {
					addr := address(source, i)
					r := f.AllocReg()
					b.Insts = append(b.Insts, &Inst{Op: OpLoad, Dst: r, Src: [2]Reg{addr}, Ty: TyU8, Cls: ClassGeneral})
					values[i] = r
				}
				for i, r := range values {
					addr := address(dest, i)
					b.Insts = append(b.Insts, &Inst{Op: OpStore, Src: [2]Reg{addr, r}, Ty: TyU8})
				}
				ret.Vals[slot.ReturnIndex] = dest
			}
		}
	}
	return nil
}

// BindStructReturnArguments adapts a public source-argument VM invocation.
// Internal MIR calls already carry explicit buffer pointers.
func (vm *VM) BindStructReturnArguments(fn *Func, args []Value) ([]Value, error) {
	slots := fn.Contract.StructReturnBuffers
	if len(slots) == 0 {
		return args, nil
	}
	declared := len(fn.Contract.Params) - len(slots)
	if len(args) != declared {
		return nil, fmt.Errorf("@%s expects %d source args, got %d", fn.Name, declared, len(args))
	}
	out := append([]Value(nil), args...)
	for _, s := range slots {
		if s.ParamIndex != len(out) {
			return nil, fmt.Errorf("invalid struct return param order")
		}
		n := ByteWidth(s.Ty)
		if n == 0 {
			n = 1
		}
		if len(vm.heap)+n > vm.maxMem() {
			return nil, fmt.Errorf("struct result exceeds VM memory")
		}
		out = append(out, vm.AllocHeap(make([]byte, n)))
	}
	return out, nil
}

// VerifyStructReturnBuffers checks ownership ABI independently of lowering.
func VerifyStructReturnBuffers(m *Module) error {
	for _, f := range m.Funcs {
		var expected []int
		for ri, r := range f.Contract.Returns {
			if _, ok := BaseOf(r.Ty).(*StructTy); ok {
				expected = append(expected, ri)
			}
		}
		if len(expected) != len(f.Contract.StructReturnBuffers) {
			return fmt.Errorf("missing struct return buffer ABI: %s", f.Name)
		}
		declared := len(f.Contract.Params) - len(expected)
		for i, s := range f.Contract.StructReturnBuffers {
			if s.Ty == nil || s.ReturnIndex != expected[i] || s.ParamIndex != declared+i || s.ParamIndex < 0 || s.ParamIndex >= len(f.Contract.Params) {
				return fmt.Errorf("invalid struct return buffer slot: %s", f.Name)
			}
			if !sameStructLayout(s.Ty, f.Contract.Returns[s.ReturnIndex].Ty) || !IsPointer(f.Contract.Params[s.ParamIndex].Ty) {
				return fmt.Errorf("invalid struct return buffer type: %s", f.Name)
			}
		}
		for _, b := range f.Blocks {
			for _, inst := range b.Insts {
				if inst.Op != OpCall && inst.Op != OpCallIndirect && inst.Op != OpCallCond {
					if len(inst.StructReturnBuffers) > 0 {
						return fmt.Errorf("struct metadata on non-call")
					}
					continue
				}
				var slots []StructReturnBuffer
				if inst.Op == OpCall || inst.Op == OpCallCond {
					if callee := m.FuncByName(inst.Sym); callee != nil {
						slots = callee.Contract.StructReturnBuffers
					}
				}
				if inst.Op == OpCallIndirect {
					if ty, ok := BaseOf(inst.Ty).(*StructTy); ok {
						slots = []StructReturnBuffer{{ReturnIndex: 0, ParamIndex: len(inst.Args) - 1, Ty: ty}}
					}
				}
				if len(slots) != len(inst.StructReturnBuffers) {
					return fmt.Errorf("struct call buffer ABI mismatch: %s", inst.Sym)
				}
				for i, s := range slots {
					a := inst.StructReturnBuffers[i]
					if s.ReturnIndex != a.ReturnIndex || s.ParamIndex != a.ParamIndex || !sameStructLayout(s.Ty, a.Ty) || s.ParamIndex < 0 || s.ParamIndex >= len(inst.Args) {
						return fmt.Errorf("invalid struct call buffer slot: %s", inst.Sym)
					}
				}
			}
		}
	}
	return nil
}

func sameStructLayout(a, b Ty) bool { return reflect.DeepEqual(a, b) }

// SourceParamCount excludes ABI-generated result-buffer parameters.
func SourceParamCount(f *Func) int {
	return len(f.Contract.Params) - len(f.Contract.StructReturnBuffers)
}
