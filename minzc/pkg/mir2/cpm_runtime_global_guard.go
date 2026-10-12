package mir2

import (
	"fmt"
	"reflect"
	"strings"
)

// Roots are derived from the complete SSA graph. Every pointer leaf must be a
// module-global AddrOf; joins and cycles retain authority only with one root.
func cpmGlobalStoreRoots(m *Module) map[*Inst]*Global {
	out := map[*Inst]*Global{}
	globals := map[string]*Global{}
	for n := range m.Globals {
		g := &m.Globals[n]
		globals[g.Name] = g
	}
	for _, f := range m.Funcs {
		defs := map[Reg]*Inst{}
		deps := map[Reg][]Reg{}
		bad := map[Reg]bool{}
		for _, b := range f.Blocks {
			for _, p := range b.Params {
				deps[p.Dst] = nil
				if p.Ty != TyPtr {
					bad[p.Dst] = true
				}
			}
			for _, i := range b.Insts {
				if i.Dst != NoReg {
					defs[i.Dst] = i
				}
			}
		}
		for _, b := range f.Blocks {
			b.Term.ForEachEdge(func(target string, args []Reg, offset int) {
				for _, dst := range f.Blocks {
					if dst.Label != target {
						continue
					}
					for n, p := range dst.Params {
						if offset < 0 || offset > len(dst.Params) || len(args) != len(dst.Params)-offset {
							bad[p.Dst] = true
						}
						if n < offset {
							bad[p.Dst] = true
							continue
						}
						n -= offset
						if n >= len(args) {
							bad[p.Dst] = true
						} else {
							deps[p.Dst] = append(deps[p.Dst], args[n])
						}
					}
				}
			})
		}
		for r, i := range defs {
			if !cpmCleanGlobalAddress(i) {
				bad[r] = true
				continue
			}
			switch i.Op {
			case OpMove, OpField, OpAdd, OpPtrAdd:
				deps[r] = []Reg{i.Src[0]}
			case OpAddrOf:
			default:
				bad[r] = true
			}
		}
		root := func(r Reg) *Global {
			seen := map[Reg]bool{}
			roots := map[string]bool{}
			invalid := false
			var walk func(Reg)
			walk = func(x Reg) {
				if seen[x] {
					return
				}
				seen[x] = true
				if bad[x] {
					invalid = true
					return
				}
				if i := defs[x]; i != nil && i.Op == OpAddrOf && i.Ty == TyPtr {
					if globals[i.Sym] == nil {
						invalid = true
					} else {
						roots[i.Sym] = true
					}
					return
				}
				ds, ok := deps[x]
				if !ok || len(ds) == 0 {
					invalid = true
					return
				}
				for _, d := range ds {
					walk(d)
				}
			}
			walk(r)
			if invalid || len(roots) != 1 {
				return nil
			}
			for name := range roots {
				return globals[name]
			}
			return nil
		}
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Op == OpStore && ByteWidth(i.Ty) >= 1 && ByteWidth(i.Ty) <= 4 {
					if def := defs[i.Src[0]]; def != nil && def.Op == OpAddrOf {
						continue
					}
					if g := root(i.Src[0]); g != nil && !g.IsConst && g.At == nil && g.Storage == StorageNormal && ByteWidth(g.Ty) >= ByteWidth(i.Ty) {
						out[i] = g
					}
				}
			}
		}
	}
	return out
}

// Valid-path guard preserves both scratch registers, flags and stack. Absolute
// bounds cover wrapped pointer arithmetic without introducing object wrap.
func (g *z80cg) genCPMGlobalStoreGuard(i *Inst) {
	global := g.cpmGlobalStores[i]
	if global == nil {
		return
	}
	width := ByteWidth(i.Ty)
	label := fmt.Sprintf(".%s_cpm_store_ok_%d_%d", sanitizeIdent(g.fn.Name), g.blockIdx, g.curInstIdx)
	ptr := g.loc(i.Src[0])
	g.emit("    PUSH AF")
	g.emit("    PUSH HL")
	switch ptr {
	case "HL":
	case "DE":
		g.emit("    LD H,D")
		g.emit("    LD L,E")
	case "BC":
		g.emit("    LD H,B")
		g.emit("    LD L,C")
	case "IX", "IY":
		g.emit("    PUSH " + ptr)
		g.emit("    POP HL")
	default:
		if !isSpill(ptr) {
			panic("CP/M global store unsupported pointer home " + ptr)
		}
		g.emitf("    LD HL,(%s)", ptr)
	}
	name := sanitizeIdent(global.Name)
	lo := label + "_lower"
	g.emitf("    LD A,%s^H", name)
	g.emit("    CP H")
	g.emitf("    JR C,%s", lo)
	g.emit("    JP NZ,__cpm_memory_failure")
	g.emitf("    LD A,%s^L", name)
	g.emit("    CP L")
	g.emitf("    JR C,%s", lo)
	g.emit("    JP NZ,__cpm_memory_failure")
	g.emit(lo + ":")
	g.emitf("    LD A,%s^H", cpmGlobalBoundName(global, width))
	g.emit("    CP H")
	g.emit("    JP C,__cpm_memory_failure")
	g.emitf("    JR NZ,%s", label)
	g.emitf("    LD A,%s^L", cpmGlobalBoundName(global, width))
	g.emit("    CP L")
	g.emit("    JP C,__cpm_memory_failure")
	g.emit(label + ":")
	g.emit("    POP HL")
	g.emit("    POP AF")
}

const cpmMemoryFailure = `
__cpm_memory_failure:
 LD A,2
 LD (__cpm_status),A
 LD SP,(__cpm_saved_sp)
 RET
`

func cpmMemoryGuardPresent(assembly string) bool {
	return strings.Contains(assembly, "__cpm_memory_failure:")
}

func cpmGlobalBoundName(global *Global, width int) string {
	return "__cpm_bound_" + sanitizeIdent(global.Name) + "_" + fmt.Sprint(width)
}

// Provenance never follows narrowed moves or instructions carrying unrelated
// effects/metadata. Dynamic offsets are evaluated by the target; only the
// complete effective address, not an inferred numeric range, gains authority.
func cpmCleanGlobalAddress(i *Inst) bool {
	if i == nil || i.Ty != TyPtr || i.Dst == NoReg {
		return false
	}
	p := *i
	p.Op, p.Dst, p.Ty, p.Cls, p.ClsHard, p.SrcFile, p.SrcLine = 0, 0, nil, 0, false, "", 0
	switch i.Op {
	case OpAddrOf:
		p.Sym = ""
	case OpMove:
		p.Src[0] = 0
	case OpField:
		p.Src[0], p.Imm = 0, 0
	case OpAdd, OpPtrAdd:
		p.Src = [2]Reg{}
	default:
		return false
	}
	return reflect.DeepEqual(p, Inst{})
}
