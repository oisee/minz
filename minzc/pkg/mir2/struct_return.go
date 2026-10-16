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
					returns := inst.IndirectReturnTys
					if len(returns) == 0 && inst.Ty != TyVoid {
						returns = []Ty{inst.Ty}
					}
					for ri, ret := range returns {
						if ty, ok := BaseOf(ret).(*StructTy); ok {
							slots = append(slots, StructReturnBuffer{ReturnIndex: ri, ParamIndex: len(inst.Args) + len(slots), Ty: ty})
						}
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
						if inst.Op == OpCallIndirect {
							inst.IndirectParamTys = append(inst.IndirectParamTys, TyPtr)
						}
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
			type pendingCopy struct {
				dest   Reg
				values []Reg
				ty     Ty
			}
			copies := make([]pendingCopy, 0, len(f.Contract.StructReturnBuffers))
			address := func(base Reg, off int) Reg {
				if off == 0 {
					return base
				}
				r := f.AllocReg()
				b.Insts = append(b.Insts, &Inst{Op: OpField, Dst: r, Src: [2]Reg{base}, Ty: TyPtr, Cls: ClassPointer, Imm: int64(off)})
				return r
			}
			// Snapshot every returned object before any destination write. Hidden
			// buffers may overlap another result's source; slot-by-slot copying is unsafe.
			for _, slot := range f.Contract.StructReturnBuffers {
				if slot.ReturnIndex >= len(ret.Vals) {
					return fmt.Errorf("missing struct return value: %s", f.Name)
				}
				source := ret.Vals[slot.ReturnIndex]
				dest := f.Contract.Params[slot.ParamIndex].Reg
				values := make([]Reg, ByteWidth(slot.Ty))
				for off := range values {
					addr := address(source, off)
					r := f.AllocReg()
					b.Insts = append(b.Insts, &Inst{Op: OpLoad, Dst: r, Src: [2]Reg{addr}, Ty: TyU8, Cls: ClassGeneral, Volatile: VolatileStorageByte(slot.Ty, off)})
					values[off] = r
				}
				copies = append(copies, pendingCopy{dest, values, slot.Ty})
				ret.Vals[slot.ReturnIndex] = dest
			}
			for _, copy := range copies {
				for off, r := range copy.values {
					addr := address(copy.dest, off)
					b.Insts = append(b.Insts, &Inst{Op: OpStore, Src: [2]Reg{addr, r}, Ty: TyU8, Volatile: VolatileStorageByte(copy.ty, off)})
				}
			}

		}
	}
	// Derive this even when contract optimization is disabled. Indirect calls
	// cannot consult the allocation chosen for their dynamically resolved target.
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, inst := range b.Insts {
				if inst.Op == OpAddrOf {
					if target := m.FuncByName(inst.Sym); target != nil {
						target.Attrs.FixedIndirectABI = true
						returnTypes := make([]Ty, len(target.Contract.Returns))
						for i, r := range target.Contract.Returns {
							returnTypes[i] = r.Ty
						}
						returnClasses, err := CanonicalIndirectReturnClasses(returnTypes)
						if err != nil {
							return fmt.Errorf("%s: %w", target.Name, err)
						}
						for i := range target.Contract.Returns {
							target.Contract.Returns[i].Class = returnClasses[i]
						}
						types := make([]Ty, len(target.Contract.Params))
						for i, p := range target.Contract.Params {
							types[i] = p.Ty
						}
						classes, err := CanonicalIndirectParamClasses(types)
						if err != nil {
							return fmt.Errorf("%s: %w", target.Name, err)
						}
						for i := range target.Contract.Params {
							target.Contract.Params[i].Class = classes[i]
						}
					}
				}
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
					returns := inst.IndirectReturnTys
					if len(returns) == 0 && inst.Ty != TyVoid {
						returns = []Ty{inst.Ty}
					}
					count := 0
					for _, ret := range returns {
						if _, ok := BaseOf(ret).(*StructTy); ok {
							count++
						}
					}
					for ri, ret := range returns {
						if ty, ok := BaseOf(ret).(*StructTy); ok {
							slots = append(slots, StructReturnBuffer{ReturnIndex: ri, ParamIndex: len(inst.Args) - count + len(slots), Ty: ty})
						}
					}
					if len(slots) > 0 && len(inst.IndirectParamTys) != len(inst.Args) {
						return fmt.Errorf("missing typed indirect buffer signature")
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
					if inst.Op == OpCallIndirect && !IsPointer(inst.IndirectParamTys[s.ParamIndex]) {
						return fmt.Errorf("indirect struct result parameter must be a pointer")
					}
				}
			}
		}
	}
	return nil
}

func sameStructLayout(a, b Ty) bool { return reflect.DeepEqual(a, b) }

// ValidateIndirectTargetABI checks semantic types before entering a resolved
// VM target. Legacy untyped scalar MIR retains its previous dispatch behavior.
func ValidateIndirectTargetABI(inst *Inst, target *Func) error {
	if target.Attrs.FallibleCY {
		return fmt.Errorf("indirect target fallible CY ABI unsupported: %s", target.Name)
	}
	if len(inst.IndirectParamTys) == 0 && len(inst.IndirectReturnTys) == 0 && len(inst.StructReturnBuffers) == 0 {
		if len(target.Contract.StructReturnBuffers) != 0 {
			return fmt.Errorf("untyped indirect target requires struct result buffers: %s", target.Name)
		}
		return nil
	}
	if len(inst.IndirectParamTys) != len(target.Contract.Params) || len(inst.IndirectReturnTys) != len(target.Contract.Returns) || !reflect.DeepEqual(inst.StructReturnBuffers, target.Contract.StructReturnBuffers) {
		return fmt.Errorf("indirect target ABI shape mismatch: %s", target.Name)
	}
	for i, ty := range inst.IndirectParamTys {
		if !reflect.DeepEqual(ty, target.Contract.Params[i].Ty) {
			return fmt.Errorf("indirect target parameter ABI mismatch: %s", target.Name)
		}
	}
	for i, ty := range inst.IndirectReturnTys {
		if !reflect.DeepEqual(ty, target.Contract.Returns[i].Ty) {
			return fmt.Errorf("indirect target return ABI mismatch: %s", target.Name)
		}
	}
	return nil
}

// SourceParamCount excludes ABI-generated result-buffer parameters.
func SourceParamCount(f *Func) int {
	return len(f.Contract.Params) - len(f.Contract.StructReturnBuffers)
}

func standardIndirectReturnClass(ty Ty, pos int) RegClass {
	if pos == 0 {
		if z80ValueWidth(ty) > 8 {
			return ClassPointer
		}
		return ClassAcc
	}
	if pos == 1 {
		if z80ValueWidth(ty) > 8 {
			return ClassIndex
		}
		return ClassCounter
	}
	return ClassGeneral
}

// CanonicalIndirectParamClasses preserves each legacy preferred slot when
// available, then assigns a free nonoverlapping slot deterministically.
func CanonicalIndirectParamClasses(types []Ty) ([]RegClass, error) {
	return canonicalIndirectClasses(types, standardParamClass)
}

func canonicalIndirectClasses(types []Ty, preferred func(Ty, int) RegClass) ([]RegClass, error) {
	classes := make([]RegClass, len(types))
	used := map[byte]bool{}
	overlap := func(name string) bool {
		for i := range name {
			if used[name[i]] {
				return true
			}
		}
		return false
	}
	for i, ty := range types {
		if z80ValueWidth(ty) > 16 {
			return nil, fmt.Errorf("indirect parameter wider than sixteen bits unsupported")
		}
		choices := []RegClass{preferred(ty, i)}
		if z80ValueWidth(ty) > 8 {
			choices = append(choices, ClassPointer, ClassIndex, ClassIY)
		} else {
			choices = append(choices, ClassAcc, ClassRegC, ClassCounter, ClassRegE, ClassRegD, ClassRegL, ClassRegH)
		}
		found := false
		for _, cls := range choices {
			name := canonicalReturnLoc(cls, ty)
			if !overlap(name) {
				classes[i] = cls
				for j := range name {
					used[name[j]] = true
				}
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("indirect parameter registers exhausted at slot %d", i)
		}
	}
	return classes, nil
}

func CanonicalIndirectReturnClasses(types []Ty) ([]RegClass, error) {
	return canonicalIndirectClasses(types, standardIndirectReturnClass)
}
