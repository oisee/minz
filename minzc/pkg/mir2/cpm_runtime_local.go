package mir2

import (
	"fmt"
	"reflect"
	"strings"
)

// The only admitted source ASM is the complete one-byte BDOS2 adapter. Its
// effective CALL clobber contract remains conservative at ordinary callers.
func cpmSourceBDOS2(f *Func, ar *AllocResult) bool {
	if f.Attrs.IsExtern || f.Attrs.ExternAddr != 0 || f.Attrs.FallibleCY || len(f.Contract.Params) != 1 || len(f.Contract.Returns) != 0 || len(f.Blocks) != 1 {
		return false
	}
	p := f.Contract.Params[0]
	b := f.Blocks[0]
	if p.Ty != TyU8 || ar.Loc(p.Reg).Name != "A" || len(b.Insts) != 1 {
		return false
	}
	t, ok := b.Term.(*TermRet)
	if !ok || len(t.Vals) != 0 {
		return false
	}
	i := b.Insts[0]
	if i.Op != OpAsm || i.Asm == nil || i.Dst != NoReg || len(i.Asm.Ins) != 1 || i.Asm.Ins[0] != p.Reg || len(i.Asm.Outs) != 0 || i.Asm.Clobbers != AllClasses {
		return false
	}
	lines := strings.Split(i.Asm.Template, "\n")
	if len(lines) != 3 {
		return false
	}
	want := []string{"LDE,A", "LDC,2", "CALL5"}
	for n, line := range lines {
		if strings.Join(strings.Fields(strings.ToUpper(line)), "") != want[n] {
			return false
		}
	}
	return true
}

// Only native-width pointer operations with their exact semantic fields may
// carry local-object authority. Value casts and proof/call metadata do not.
func cpmCleanLocalAddress(i *Inst) bool {
	if i == nil || i.Ty != TyPtr || i.Dst == NoReg {
		return false
	}
	p := *i
	p.Op, p.Dst, p.Ty, p.Cls, p.ClsHard, p.SrcFile, p.SrcLine = 0, 0, nil, 0, false, "", 0
	switch i.Op {
	case OpAlloca:
		p.Imm = 0
	case OpMove:
		p.Src[0] = 0
	case OpField:
		p.Src[0], p.Imm = 0, 0
	case OpPtrAdd:
		p.Src = [2]Reg{}
	default:
		return false
	}
	return reflect.DeepEqual(p, Inst{})
}

func cpmLocalOffset(i *Inst) (int64, bool) {
	if i == nil || i.Op != OpConst || !(i.Ty == TyU8 || i.Ty == TyI8 || i.Ty == TyU16 || i.Ty == TyI16) {
		return 0, false
	}
	p := *i
	p.Op, p.Dst, p.Imm, p.Ty, p.Cls, p.ClsHard, p.SrcFile, p.SrcLine = 0, 0, 0, nil, 0, false, "", 0
	if !reflect.DeepEqual(p, Inst{}) {
		return 0, false
	}
	stored := maskToWidth(i.Imm, i.Ty)
	if stored != i.Imm {
		return 0, false
	}
	return stored, true
}

type cpmLocalAddress struct {
	root         Reg
	offset, size int64
}

// Local object addresses are admitted only for bounded entry allocations and
// constant derived fields. No pointer arithmetic based on runtime values.
func cpmLocalAddresses(f *Func) (map[Reg]cpmLocalAddress, error) {
	defs := map[Reg]*Inst{}
	entry := f.Entry()
	if entry == nil {
		return map[Reg]cpmLocalAddress{}, nil
	}
	total := int64(0)
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i.Dst != NoReg {
				defs[i.Dst] = i
			}
			if i.Op == OpAlloca {
				if b != entry || !cpmCleanLocalAddress(i) || i.Imm <= 0 || i.Imm > 256 || total+i.Imm > 256 {
					return nil, fmt.Errorf("CP/M runtime requires bounded entry-local allocations")
				}
				total += i.Imm
			}
		}
	}
	out := map[Reg]cpmLocalAddress{}
	busy := map[Reg]bool{}
	var resolve func(Reg) (cpmLocalAddress, bool)
	resolve = func(r Reg) (cpmLocalAddress, bool) {
		if x, ok := out[r]; ok {
			return x, true
		}
		if busy[r] {
			return cpmLocalAddress{}, false
		}
		busy[r] = true
		defer delete(busy, r)
		i := defs[r]
		if !cpmCleanLocalAddress(i) {
			return cpmLocalAddress{}, false
		}
		var x cpmLocalAddress
		ok := false
		switch i.Op {
		case OpAlloca:
			x = cpmLocalAddress{r, 0, i.Imm}
			ok = true
		case OpMove:
			x, ok = resolve(i.Src[0])
		case OpField:
			x, ok = resolve(i.Src[0])
			if i.Imm < 0 || i.Imm >= x.size || x.offset > x.size-i.Imm {
				ok = false
			} else {
				x.offset += i.Imm
			}
		case OpPtrAdd:
			if off, valid := cpmLocalOffset(defs[i.Src[1]]); valid {
				x, ok = resolve(i.Src[0])
				if off < 0 || off >= x.size || x.offset > x.size-off {
					ok = false
				} else {
					x.offset += off
				}
			}
		}
		if ok && x.offset >= 0 && x.offset < x.size {
			out[r] = x
			return x, true
		}
		return cpmLocalAddress{}, false
	}
	for r := range defs {
		resolve(r)
	}
	return out, nil
}

// A local pointer may cross a direct call only into a closed read-only leaf
// parameter. It cannot be returned, stored, or forwarded elsewhere.
func cpmReadOnlyPointerParam(f *Func, index int, size int64) bool {
	if f == nil || f.Attrs.IsExtern || f.Attrs.HasAsm || index < 0 || index >= len(f.Contract.Params) || f.Contract.Params[index].Ty != TyPtr {
		return false
	}
	pointers := map[Reg]bool{f.Contract.Params[index].Reg: true}
	offsets := map[Reg]int64{f.Contract.Params[index].Reg: 0}
	count := 0
	for _, b := range f.Blocks {
		count += len(b.Insts)
	}
	for pass := 0; pass <= count; pass++ {
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if cpmCleanLocalAddress(i) && (i.Op == OpField || i.Op == OpMove) && pointers[i.Src[0]] {
					pointers[i.Dst] = true
					offsets[i.Dst] = offsets[i.Src[0]]
					if i.Op == OpField {
						offsets[i.Dst] += i.Imm
					}
				}
			}
		}
	}
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			for _, r := range i.Uses() {
				if !pointers[r] {
					continue
				}
				switch i.Op {
				case OpLoad:
					if i.Src[0] != r || offsets[r] < 0 || offsets[r]+int64(ByteWidth(i.Ty)) > size {
						return false
					}
				case OpField, OpMove:
					if !cpmCleanLocalAddress(i) || i.Src[0] != r || offsets[i.Dst] < 0 || offsets[i.Dst] >= size {
						return false
					}
				default:
					return false
				}
			}
		}
		for _, r := range b.Term.termUses() {
			if pointers[r] {
				return false
			}
		}
	}
	return true
}
