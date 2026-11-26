package mir2

import (
	"fmt"
	"reflect"
	"strings"
)

// Roots are derived from every definition of each register: instruction
// results and the values reaching its block parameters. Every pointer leaf must
// be a module-global AddrOf; joins and cycles retain authority only with one root.
func cpmGlobalStoreRoots(m *Module) map[*Inst]*Global {
	out := map[*Inst]*Global{}
	globals := map[string]*Global{}
	for n := range m.Globals {
		g := &m.Globals[n]
		globals[g.Name] = g
	}
	for _, f := range m.Funcs {
		defs := cpmFuncDefs(f)
		root := func(r Reg) *Global {
			seen := map[Reg]bool{}
			roots := map[string]bool{}
			invalid := false
			var walk func(Reg)
			walk = func(x Reg) {
				if seen[x] || invalid {
					return
				}
				seen[x] = true
				if defs.implicit[x] || len(defs.insts[x]) == 0 && len(defs.feeders[x]) == 0 {
					invalid = true
					return
				}
				for _, p := range defs.params[x] {
					if p.Ty != TyPtr {
						invalid = true
						return
					}
				}
				for _, i := range defs.insts[x] {
					if !cpmCleanGlobalAddress(i) {
						invalid = true
						return
					}
					switch i.Op {
					case OpAddrOf:
						if globals[i.Sym] == nil {
							invalid = true
							return
						}
						roots[i.Sym] = true
					case OpMove, OpField, OpAdd, OpPtrAdd:
						walk(i.Src[0])
					default:
						invalid = true
						return
					}
				}
				for _, a := range defs.feeders[x] {
					walk(a)
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
					if def := defs.sole(i.Src[0]); def != nil && def.Op == OpAddrOf {
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
		p.InBounds = false // proof annotation; authority still comes from the address
	default:
		return false
	}
	return reflect.DeepEqual(p, Inst{})
}
