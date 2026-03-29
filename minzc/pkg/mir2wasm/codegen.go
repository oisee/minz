// Package mir2wasm translates MIR2 modules to WebAssembly text format (WAT).
//
// WASM is a stack-based virtual machine with i32/i64/f32/f64 types.
// MIR2 is SSA-based with u8/u16/ptr types. The translation:
//
//   MIR2 types → WASM types:
//     TyBool, TyU8, TyI8  → i32 (WASM minimum is 32-bit)
//     TyU16, TyI16         → i32
//     TyPtr                → i32 (linear memory offset)
//     TyVoid               → (no return)
//
//   MIR2 SSA → WASM locals:
//     Each MIR2 vreg becomes a WASM local variable.
//     SSA form means each vreg is assigned exactly once.
//
//   MIR2 blocks → WASM blocks/loops:
//     Simple: flatten blocks with br/br_if for control flow.
//     Loop heads → (loop ...) with br back-edge.
//
// Output: WAT text that can be compiled to .wasm via wat2wasm or
// executed via wazero/wasmtime/wasmer.
//
// Primary use: run MinZ programs in the browser or as WASI CLI apps.
package mir2wasm

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

// Compile translates a MIR2 module to WAT (WebAssembly Text) format.
func Compile(m *mir2.Module) (string, error) {
	g := &gen{sb: &strings.Builder{}, mod: m}
	g.emitModule()
	return g.sb.String(), nil
}

type gen struct {
	sb  *strings.Builder
	mod *mir2.Module

	// Per-function state
	localDecls []string          // local variable declarations
	localIdx   map[mir2.Reg]int  // vreg → local index
	nextLocal  int               // next local index
	paramCount int               // number of function params (locals 0..paramCount-1)
}

func (g *gen) emitModule() {
	g.sb.WriteString("(module\n")

	// Memory: 1 page (64KB) — enough for Z80's address space
	g.sb.WriteString("  (memory (export \"memory\") 1)\n\n")

	// Globals
	for _, gl := range g.mod.Globals {
		w := mir2.ByteWidth(gl.Ty)
		if w == 0 {
			w = 2
		}
		g.sb.WriteString(fmt.Sprintf("  ;; global %s (%d bytes)\n", gl.Name, w))
	}

	// String data — emit as data segments
	if g.mod.Strings.Len() > 0 {
		offset := 0xF000 // put strings high in memory (like Z80 ROM area)
		for i := 0; i < g.mod.Strings.Len(); i++ {
			s := g.mod.Strings.At(i)
			sym := g.mod.Strings.Symbol(i)
			g.sb.WriteString(fmt.Sprintf("  ;; string %s = %q at 0x%04X\n", sym, s, offset))
			g.sb.WriteString(fmt.Sprintf("  (data (i32.const %d) %q)\n", offset, s+"\x00"))
			offset += len(s) + 1
		}
		g.sb.WriteString("\n")
	}

	// Functions
	for _, f := range g.mod.Funcs {
		g.emitFunc(f)
	}

	g.sb.WriteString(")\n")
}

func (g *gen) emitFunc(f *mir2.Func) {
	// Reset per-function state
	g.localDecls = nil
	g.localIdx = make(map[mir2.Reg]int)
	g.nextLocal = 0
	g.paramCount = 0

	name := wasmSym(f.Name)

	// Params
	var paramTypes []string
	for _, p := range f.Contract.Params {
		ty := wasmType(p.Ty)
		paramTypes = append(paramTypes, fmt.Sprintf("(param $p%d %s)", g.paramCount, ty))
		g.localIdx[p.Reg] = g.nextLocal
		g.nextLocal++
		g.paramCount++
	}

	// Return type
	retType := ""
	if len(f.Contract.Returns) > 0 && f.Contract.Returns[0].Ty != mir2.TyVoid {
		retType = fmt.Sprintf("(result %s)", wasmType(f.Contract.Returns[0].Ty))
	}

	// Scan all vregs to declare locals
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Dst != mir2.NoReg {
				if _, ok := g.localIdx[inst.Dst]; !ok {
					g.localIdx[inst.Dst] = g.nextLocal
					g.localDecls = append(g.localDecls, fmt.Sprintf("    (local $v%d i32)", inst.Dst))
					g.nextLocal++
				}
			}
		}
		// Block params
		for _, p := range b.Params {
			if _, ok := g.localIdx[p.Dst]; !ok {
				g.localIdx[p.Dst] = g.nextLocal
				g.localDecls = append(g.localDecls, fmt.Sprintf("    (local $v%d i32)", p.Dst))
				g.nextLocal++
			}
		}
	}

	// Emit function header
	g.sb.WriteString(fmt.Sprintf("  (func $%s (export %q) %s %s\n",
		name, name, strings.Join(paramTypes, " "), retType))

	// Local declarations
	for _, ld := range g.localDecls {
		g.sb.WriteString(ld + "\n")
	}

	// Emit blocks — flatten for now (simple linear emission)
	for _, b := range f.Blocks {
		g.sb.WriteString(fmt.Sprintf("    ;; block %s\n", b.Label))
		for _, inst := range b.Insts {
			g.emitInst(inst)
		}
		g.emitTerm(b.Term, f)
	}

	g.sb.WriteString("  )\n\n")
}

func (g *gen) emitInst(inst *mir2.Inst) {
	dst := inst.Dst
	src0 := inst.Src[0]
	src1 := inst.Src[1]

	switch inst.Op {
	case mir2.OpConst:
		g.sb.WriteString(fmt.Sprintf("    i32.const %d\n", inst.Imm))
		g.emitSetLocal(dst)

	case mir2.OpMove:
		g.emitGetLocal(src0)
		g.emitSetLocal(dst)

	case mir2.OpAdd:
		g.emitGetLocal(src0)
		g.emitGetLocal(src1)
		g.sb.WriteString("    i32.add\n")
		g.emitMask(inst.Ty)
		g.emitSetLocal(dst)

	case mir2.OpSub:
		g.emitGetLocal(src0)
		g.emitGetLocal(src1)
		g.sb.WriteString("    i32.sub\n")
		g.emitMask(inst.Ty)
		g.emitSetLocal(dst)

	case mir2.OpMul:
		g.emitGetLocal(src0)
		g.emitGetLocal(src1)
		g.sb.WriteString("    i32.mul\n")
		g.emitMask(inst.Ty)
		g.emitSetLocal(dst)

	case mir2.OpAnd:
		g.emitGetLocal(src0)
		g.emitGetLocal(src1)
		g.sb.WriteString("    i32.and\n")
		g.emitSetLocal(dst)

	case mir2.OpOr:
		g.emitGetLocal(src0)
		g.emitGetLocal(src1)
		g.sb.WriteString("    i32.or\n")
		g.emitSetLocal(dst)

	case mir2.OpXor:
		g.emitGetLocal(src0)
		g.emitGetLocal(src1)
		g.sb.WriteString("    i32.xor\n")
		g.emitSetLocal(dst)

	case mir2.OpShl:
		g.emitGetLocal(src0)
		g.emitGetLocal(src1)
		g.sb.WriteString("    i32.shl\n")
		g.emitMask(inst.Ty)
		g.emitSetLocal(dst)

	case mir2.OpShr:
		g.emitGetLocal(src0)
		g.emitGetLocal(src1)
		g.sb.WriteString("    i32.shr_u\n")
		g.emitSetLocal(dst)

	case mir2.OpSar:
		g.emitGetLocal(src0)
		g.emitGetLocal(src1)
		g.sb.WriteString("    i32.shr_s\n")
		g.emitSetLocal(dst)

	case mir2.OpNeg:
		g.sb.WriteString("    i32.const 0\n")
		g.emitGetLocal(src0)
		g.sb.WriteString("    i32.sub\n")
		g.emitMask(inst.Ty)
		g.emitSetLocal(dst)

	case mir2.OpNot:
		g.emitGetLocal(src0)
		g.sb.WriteString("    i32.const -1\n")
		g.sb.WriteString("    i32.xor\n")
		g.emitMask(inst.Ty)
		g.emitSetLocal(dst)

	case mir2.OpCmp:
		g.emitGetLocal(src0)
		g.emitGetLocal(src1)
		switch inst.Cond {
		case mir2.CmpEq:
			g.sb.WriteString("    i32.eq\n")
		case mir2.CmpNe:
			g.sb.WriteString("    i32.ne\n")
		case mir2.CmpLt, mir2.CmpUlt:
			g.sb.WriteString("    i32.lt_u\n")
		case mir2.CmpLe, mir2.CmpUle:
			g.sb.WriteString("    i32.le_u\n")
		case mir2.CmpGt, mir2.CmpUgt:
			g.sb.WriteString("    i32.gt_u\n")
		case mir2.CmpGe, mir2.CmpUge:
			g.sb.WriteString("    i32.ge_u\n")
		default:
			g.sb.WriteString(fmt.Sprintf("    ;; TODO: cmp cond %d\n", inst.Cond))
			g.sb.WriteString("    i32.eq\n")
		}
		g.emitSetLocal(dst)

	case mir2.OpLoad:
		g.emitGetLocal(src0) // address
		switch tyWidth(inst.Ty) {
		case 1:
			g.sb.WriteString("    i32.load8_u\n")
		case 2:
			g.sb.WriteString("    i32.load16_u\n")
		default:
			g.sb.WriteString("    i32.load\n")
		}
		g.emitSetLocal(dst)

	case mir2.OpStore:
		g.emitGetLocal(src0) // address
		g.emitGetLocal(src1) // value
		switch tyWidth(inst.Ty) {
		case 1:
			g.sb.WriteString("    i32.store8\n")
		case 2:
			g.sb.WriteString("    i32.store16\n")
		default:
			g.sb.WriteString("    i32.store\n")
		}

	case mir2.OpExt, mir2.OpSext, mir2.OpTrunc:
		g.emitGetLocal(src0)
		g.emitMask(inst.Ty)
		g.emitSetLocal(dst)

	case mir2.OpCall:
		// Push arguments
		for _, arg := range inst.Args {
			g.emitGetLocal(arg)
		}
		g.sb.WriteString(fmt.Sprintf("    call $%s\n", wasmSym(inst.Sym)))
		if dst != mir2.NoReg {
			g.emitSetLocal(dst)
		}

	case mir2.OpAddrOf:
		// Address of global — for now emit a constant placeholder
		g.sb.WriteString(fmt.Sprintf("    i32.const 0 ;; addr_of %s\n", inst.Sym))
		g.emitSetLocal(dst)

	default:
		g.sb.WriteString(fmt.Sprintf("    ;; TODO: op %d\n", inst.Op))
	}
}

func (g *gen) emitTerm(term mir2.Term, f *mir2.Func) {
	switch t := term.(type) {
	case *mir2.TermRet:
		if len(t.Vals) > 0 && t.Vals[0] != mir2.NoReg {
			g.emitGetLocal(t.Vals[0])
			g.sb.WriteString("    return\n")
		} else {
			g.sb.WriteString("    return\n")
		}
	case *mir2.TermJmp:
		// Simple jump — in flattened output, just fall through
		// Pass block args to target params
		g.sb.WriteString(fmt.Sprintf("    ;; jmp %s\n", t.Target))
	case *mir2.TermBrIf:
		g.emitGetLocal(t.Cond)
		g.sb.WriteString(fmt.Sprintf("    ;; br_if %s / %s\n", t.Then, t.Else))
	case nil:
		// no terminator
	default:
		g.sb.WriteString("    ;; TODO: terminator\n")
	}
}

// ── helpers ──────────────────────────────────────────────────────────────

func (g *gen) emitGetLocal(reg mir2.Reg) {
	if reg == mir2.NoReg {
		g.sb.WriteString("    i32.const 0 ;; NoReg\n")
		return
	}
	idx, ok := g.localIdx[reg]
	if !ok {
		g.sb.WriteString(fmt.Sprintf("    i32.const 0 ;; unknown vreg %d\n", reg))
		return
	}
	if idx < g.paramCount {
		g.sb.WriteString(fmt.Sprintf("    local.get $p%d\n", idx))
	} else {
		g.sb.WriteString(fmt.Sprintf("    local.get $v%d\n", reg))
	}
}

func (g *gen) emitSetLocal(reg mir2.Reg) {
	if reg == mir2.NoReg {
		g.sb.WriteString("    drop\n")
		return
	}
	idx, ok := g.localIdx[reg]
	if !ok {
		g.sb.WriteString(fmt.Sprintf("    drop ;; unknown dst vreg %d\n", reg))
		return
	}
	if idx < g.paramCount {
		g.sb.WriteString(fmt.Sprintf("    local.set $p%d\n", idx))
	} else {
		g.sb.WriteString(fmt.Sprintf("    local.set $v%d\n", reg))
	}
}

// emitMask masks a value to the width of ty (e.g., u8 → AND 0xFF)
func (g *gen) emitMask(ty mir2.Ty) {
	switch ty {
	case mir2.TyU8, mir2.TyI8, mir2.TyBool:
		g.sb.WriteString("    i32.const 255\n    i32.and\n")
	case mir2.TyU16, mir2.TyI16:
		g.sb.WriteString("    i32.const 65535\n    i32.and\n")
	}
	// i32/ptr: no mask needed
}

func wasmType(ty mir2.Ty) string {
	switch ty {
	case mir2.TyVoid:
		return ""
	default:
		return "i32" // everything is i32 for now
	}
}

func wasmSym(name string) string {
	s := strings.ReplaceAll(name, "@", "_")
	s = strings.ReplaceAll(s, "$", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func tyWidth(ty mir2.Ty) int {
	if ty == nil {
		return 1
	}
	w := ty.Width()
	if w == 0 {
		return 1
	}
	return (w + 7) / 8
}
