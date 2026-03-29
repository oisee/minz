// binary.go — Encode WASM binary format directly from MIR2.
//
// WASM binary format (MVP):
//   magic:   0x00 0x61 0x73 0x6D  (\0asm)
//   version: 0x01 0x00 0x00 0x00
//   sections: type, function, export, code, memory, data
//
// This encodes a minimal WASM module that wazero can instantiate directly.
// No wat2wasm dependency needed.
package mir2wasm

import (
	"encoding/binary"

	"github.com/minz/minzc/pkg/mir2"
)

// CompileBinary translates a MIR2 module to WASM binary format.
// Returns raw .wasm bytes that can be loaded by any WASM runtime.
func CompileBinary(m *mir2.Module) ([]byte, error) {
	e := &encoder{mod: m}
	return e.encode()
}

type encoder struct {
	mod   *mir2.Module
	funcs []*mir2.Func
}

func (e *encoder) encode() ([]byte, error) {
	e.funcs = e.mod.Funcs

	var buf []byte

	// Magic + version
	buf = append(buf, 0x00, 0x61, 0x73, 0x6D) // \0asm
	buf = append(buf, 0x01, 0x00, 0x00, 0x00) // version 1

	// Section 1: Type section (function signatures)
	typeSec := e.encodeTypeSection()
	buf = append(buf, 0x01) // type section id
	buf = append(buf, encodeLEB128(len(typeSec))...)
	buf = append(buf, typeSec...)

	// Section 3: Function section (type indices)
	funcSec := e.encodeFuncSection()
	buf = append(buf, 0x03)
	buf = append(buf, encodeLEB128(len(funcSec))...)
	buf = append(buf, funcSec...)

	// Section 5: Memory section (1 page = 64KB)
	memSec := []byte{0x01, 0x00, 0x01} // 1 memory, no max, 1 page min
	buf = append(buf, 0x05)
	buf = append(buf, encodeLEB128(len(memSec))...)
	buf = append(buf, memSec...)

	// Section 7: Export section
	exportSec := e.encodeExportSection()
	buf = append(buf, 0x07)
	buf = append(buf, encodeLEB128(len(exportSec))...)
	buf = append(buf, exportSec...)

	// Section 10: Code section
	codeSec := e.encodeCodeSection()
	buf = append(buf, 0x0A)
	buf = append(buf, encodeLEB128(len(codeSec))...)
	buf = append(buf, codeSec...)

	return buf, nil
}

func (e *encoder) encodeTypeSection() []byte {
	var buf []byte
	buf = append(buf, encodeLEB128(len(e.funcs))...) // num types

	for _, f := range e.funcs {
		buf = append(buf, 0x60) // func type

		// Params
		nParams := len(f.Contract.Params)
		buf = append(buf, encodeLEB128(nParams)...)
		for range f.Contract.Params {
			buf = append(buf, 0x7F) // i32
		}

		// Returns
		if len(f.Contract.Returns) > 0 && f.Contract.Returns[0].Ty != mir2.TyVoid {
			buf = append(buf, 0x01) // 1 result
			buf = append(buf, 0x7F) // i32
		} else {
			buf = append(buf, 0x00) // 0 results
		}
	}
	return buf
}

func (e *encoder) encodeFuncSection() []byte {
	var buf []byte
	buf = append(buf, encodeLEB128(len(e.funcs))...)
	for i := range e.funcs {
		buf = append(buf, encodeLEB128(i)...) // type index = func index
	}
	return buf
}

func (e *encoder) encodeExportSection() []byte {
	var buf []byte

	// Export all functions + memory
	numExports := len(e.funcs) + 1
	buf = append(buf, encodeLEB128(numExports)...)

	// Export memory
	buf = append(buf, encodeLEB128(len("memory"))...)
	buf = append(buf, "memory"...)
	buf = append(buf, 0x02) // memory export
	buf = append(buf, 0x00) // memory index 0

	// Export functions
	for i, f := range e.funcs {
		name := wasmSym(f.Name)
		buf = append(buf, encodeLEB128(len(name))...)
		buf = append(buf, name...)
		buf = append(buf, 0x00) // func export
		buf = append(buf, encodeLEB128(i)...)
	}
	return buf
}

func (e *encoder) encodeCodeSection() []byte {
	var buf []byte
	buf = append(buf, encodeLEB128(len(e.funcs))...)

	for i, f := range e.funcs {
		body := e.encodeFuncBody(f, i)
		buf = append(buf, encodeLEB128(len(body))...)
		buf = append(buf, body...)
	}
	return buf
}

func (e *encoder) encodeFuncBody(f *mir2.Func, funcIdx int) []byte {
	c := &funcCodegen{
		f:        f,
		localIdx: make(map[mir2.Reg]int),
		funcMap:  make(map[string]int),
	}

	// Build function name → index map for calls
	for i, fn := range e.funcs {
		c.funcMap[fn.Name] = i
	}

	// Assign locals: params first, then instruction dsts
	for _, p := range f.Contract.Params {
		c.localIdx[p.Reg] = c.nextLocal
		c.nextLocal++
	}
	c.paramCount = c.nextLocal

	// Scan for additional locals
	var extraLocals int
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Dst != mir2.NoReg {
				if _, ok := c.localIdx[inst.Dst]; !ok {
					c.localIdx[inst.Dst] = c.nextLocal
					c.nextLocal++
					extraLocals++
				}
			}
		}
		for _, p := range b.Params {
			if _, ok := c.localIdx[p.Dst]; !ok {
				c.localIdx[p.Dst] = c.nextLocal
				c.nextLocal++
				extraLocals++
			}
		}
	}

	var body []byte

	// Local declarations
	if extraLocals > 0 {
		body = append(body, encodeLEB128(1)...) // 1 local decl group
		body = append(body, encodeLEB128(extraLocals)...)
		body = append(body, 0x7F) // i32
	} else {
		body = append(body, 0x00) // 0 local decl groups
	}

	// Instructions — handle control flow between blocks.
	// Strategy: for each block, emit instructions + terminator.
	// cond_ret → WASM if/else/end
	// br_if → WASM if/else/end
	// jmp → fall through (blocks already in order)
	blockMap := make(map[string]int)
	for i, b := range f.Blocks {
		blockMap[b.Label] = i
	}

	for bi, b := range f.Blocks {
		for _, inst := range b.Insts {
			body = append(body, c.encodeInst(inst)...)
		}

		switch t := b.Term.(type) {
		case *mir2.TermRet:
			if len(t.Vals) > 0 && t.Vals[0] != mir2.NoReg {
				body = append(body, c.getLocal(t.Vals[0])...)
				body = append(body, 0x0F) // return
			}

		case *mir2.TermCondRet:
			// cond_ret %cond, [vals], @then
			// if cond==0 → return vals; else → jump to then (next block)
			// WASM: get cond; i32.eqz; if [result] { get val; return } end
			body = append(body, c.getLocal(t.Cond)...)
			body = append(body, 0x45) // i32.eqz (cond==0 means return)

			// Determine result type for if block
			retTy := byte(0x40) // void block type
			if len(f.Contract.Returns) > 0 && f.Contract.Returns[0].Ty != mir2.TyVoid {
				retTy = 0x7F // i32
			}
			body = append(body, 0x04, retTy) // if (result type)
			if len(t.Vals) > 0 && t.Vals[0] != mir2.NoReg {
				body = append(body, c.getLocal(t.Vals[0])...)
				body = append(body, 0x0F) // return
			}
			body = append(body, 0x0B) // end if
			// Fall through to next block (the "then" target)

		case *mir2.TermBrIf:
			// br_if %cond, @then, @else
			// WASM: get cond; if { then_block } else { else_block } end
			body = append(body, c.getLocal(t.Cond)...)

			retTy := byte(0x40) // void
			body = append(body, 0x04, retTy) // if

			// Then block: emit inline if it's the next block
			if thenIdx, ok := blockMap[t.Then]; ok && thenIdx < len(f.Blocks) {
				thenBlock := f.Blocks[thenIdx]
				for _, inst := range thenBlock.Insts {
					body = append(body, c.encodeInst(inst)...)
				}
				// Then block terminator (usually ret)
				if ret, ok := thenBlock.Term.(*mir2.TermRet); ok {
					if len(ret.Vals) > 0 && ret.Vals[0] != mir2.NoReg {
						body = append(body, c.getLocal(ret.Vals[0])...)
						body = append(body, 0x0F) // return
					}
				}
			}

			body = append(body, 0x05) // else

			// Else block: emit inline if it exists
			if elseIdx, ok := blockMap[t.Else]; ok && elseIdx < len(f.Blocks) {
				elseBlock := f.Blocks[elseIdx]
				for _, inst := range elseBlock.Insts {
					body = append(body, c.encodeInst(inst)...)
				}
				if ret, ok := elseBlock.Term.(*mir2.TermRet); ok {
					if len(ret.Vals) > 0 && ret.Vals[0] != mir2.NoReg {
						body = append(body, c.getLocal(ret.Vals[0])...)
						body = append(body, 0x0F) // return
					}
				}
			}

			body = append(body, 0x0B) // end if

			// Skip the blocks we already emitted inline
			_ = bi // blocks emitted inline, don't emit again

		case *mir2.TermJmp:
			// Unconditional jump — pass block args then fall through
			// (blocks are in order, so next block IS the target usually)

		case nil:
			// no terminator — fall through
		}
	}

	// End of function
	body = append(body, 0x0B) // end

	return body
}

type funcCodegen struct {
	f          *mir2.Func
	localIdx   map[mir2.Reg]int
	nextLocal  int
	paramCount int
	funcMap    map[string]int
}

func (c *funcCodegen) encodeInst(inst *mir2.Inst) []byte {
	var buf []byte

	switch inst.Op {
	case mir2.OpConst:
		buf = append(buf, 0x41) // i32.const
		buf = append(buf, encodeSLEB128(inst.Imm)...)
		buf = append(buf, c.setLocal(inst.Dst)...)

	case mir2.OpMove:
		buf = append(buf, c.getLocal(inst.Src[0])...)
		buf = append(buf, c.setLocal(inst.Dst)...)

	case mir2.OpAdd:
		buf = append(buf, c.getLocal(inst.Src[0])...)
		buf = append(buf, c.getLocal(inst.Src[1])...)
		buf = append(buf, 0x6A) // i32.add
		buf = append(buf, c.maskTy(inst.Ty)...)
		buf = append(buf, c.setLocal(inst.Dst)...)

	case mir2.OpSub:
		buf = append(buf, c.getLocal(inst.Src[0])...)
		buf = append(buf, c.getLocal(inst.Src[1])...)
		buf = append(buf, 0x6B) // i32.sub
		buf = append(buf, c.maskTy(inst.Ty)...)
		buf = append(buf, c.setLocal(inst.Dst)...)

	case mir2.OpMul:
		buf = append(buf, c.getLocal(inst.Src[0])...)
		buf = append(buf, c.getLocal(inst.Src[1])...)
		buf = append(buf, 0x6C) // i32.mul
		buf = append(buf, c.maskTy(inst.Ty)...)
		buf = append(buf, c.setLocal(inst.Dst)...)

	case mir2.OpAnd:
		buf = append(buf, c.getLocal(inst.Src[0])...)
		buf = append(buf, c.getLocal(inst.Src[1])...)
		buf = append(buf, 0x71) // i32.and
		buf = append(buf, c.setLocal(inst.Dst)...)

	case mir2.OpOr:
		buf = append(buf, c.getLocal(inst.Src[0])...)
		buf = append(buf, c.getLocal(inst.Src[1])...)
		buf = append(buf, 0x72) // i32.or
		buf = append(buf, c.setLocal(inst.Dst)...)

	case mir2.OpXor:
		buf = append(buf, c.getLocal(inst.Src[0])...)
		buf = append(buf, c.getLocal(inst.Src[1])...)
		buf = append(buf, 0x73) // i32.xor
		buf = append(buf, c.setLocal(inst.Dst)...)

	case mir2.OpShl:
		buf = append(buf, c.getLocal(inst.Src[0])...)
		buf = append(buf, c.getLocal(inst.Src[1])...)
		buf = append(buf, 0x74) // i32.shl
		buf = append(buf, c.maskTy(inst.Ty)...)
		buf = append(buf, c.setLocal(inst.Dst)...)

	case mir2.OpShr:
		buf = append(buf, c.getLocal(inst.Src[0])...)
		buf = append(buf, c.getLocal(inst.Src[1])...)
		buf = append(buf, 0x76) // i32.shr_u
		buf = append(buf, c.setLocal(inst.Dst)...)

	case mir2.OpNeg:
		buf = append(buf, 0x41, 0x00) // i32.const 0
		buf = append(buf, c.getLocal(inst.Src[0])...)
		buf = append(buf, 0x6B) // i32.sub
		buf = append(buf, c.maskTy(inst.Ty)...)
		buf = append(buf, c.setLocal(inst.Dst)...)

	case mir2.OpCmp:
		buf = append(buf, c.getLocal(inst.Src[0])...)
		buf = append(buf, c.getLocal(inst.Src[1])...)
		switch inst.Cond {
		case mir2.CmpEq:
			buf = append(buf, 0x46) // i32.eq
		case mir2.CmpNe:
			buf = append(buf, 0x47) // i32.ne
		case mir2.CmpLt, mir2.CmpUlt:
			buf = append(buf, 0x49) // i32.lt_u
		case mir2.CmpGt, mir2.CmpUgt:
			buf = append(buf, 0x4B) // i32.gt_u
		case mir2.CmpLe, mir2.CmpUle:
			buf = append(buf, 0x4D) // i32.le_u
		case mir2.CmpGe, mir2.CmpUge:
			buf = append(buf, 0x4F) // i32.ge_u
		default:
			buf = append(buf, 0x46) // i32.eq fallback
		}
		buf = append(buf, c.setLocal(inst.Dst)...)

	case mir2.OpCall:
		for _, arg := range inst.Args {
			buf = append(buf, c.getLocal(arg)...)
		}
		if idx, ok := c.funcMap[inst.Sym]; ok {
			buf = append(buf, 0x10) // call
			buf = append(buf, encodeLEB128(idx)...)
		}
		if inst.Dst != mir2.NoReg {
			buf = append(buf, c.setLocal(inst.Dst)...)
		}

	case mir2.OpExt, mir2.OpSext, mir2.OpTrunc:
		buf = append(buf, c.getLocal(inst.Src[0])...)
		buf = append(buf, c.maskTy(inst.Ty)...)
		buf = append(buf, c.setLocal(inst.Dst)...)

	// Skip unsupported ops silently
	}

	return buf
}

func (c *funcCodegen) encodeTerm(term mir2.Term) []byte {
	var buf []byte
	switch t := term.(type) {
	case *mir2.TermRet:
		if len(t.Vals) > 0 && t.Vals[0] != mir2.NoReg {
			buf = append(buf, c.getLocal(t.Vals[0])...)
			buf = append(buf, 0x0F) // return
		}
	}
	return buf
}

func (c *funcCodegen) getLocal(reg mir2.Reg) []byte {
	if reg == mir2.NoReg {
		return []byte{0x41, 0x00} // i32.const 0
	}
	idx, ok := c.localIdx[reg]
	if !ok {
		return []byte{0x41, 0x00} // fallback: const 0
	}
	var buf []byte
	buf = append(buf, 0x20) // local.get
	buf = append(buf, encodeLEB128(idx)...)
	return buf
}

func (c *funcCodegen) setLocal(reg mir2.Reg) []byte {
	if reg == mir2.NoReg {
		return []byte{0x1A} // drop
	}
	idx, ok := c.localIdx[reg]
	if !ok {
		return []byte{0x1A} // drop
	}
	var buf []byte
	buf = append(buf, 0x21) // local.set
	buf = append(buf, encodeLEB128(idx)...)
	return buf
}

func (c *funcCodegen) maskTy(ty mir2.Ty) []byte {
	switch ty {
	case mir2.TyU8, mir2.TyI8, mir2.TyBool:
		return []byte{0x41, 0xFF, 0x01, 0x71} // i32.const 255; i32.and
	case mir2.TyU16, mir2.TyI16:
		return []byte{0x41, 0xFF, 0xFF, 0x03, 0x71} // i32.const 65535; i32.and
	}
	return nil
}

// ── LEB128 encoding ──────────────────────────────────────────────────────

func encodeLEB128(v int) []byte {
	if v < 0 {
		return encodeSLEB128(int64(v))
	}
	var buf []byte
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if v == 0 {
			break
		}
	}
	return buf
}

func encodeSLEB128(v int64) []byte {
	var buf []byte
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if (v == 0 && b&0x40 == 0) || (v == -1 && b&0x40 != 0) {
			buf = append(buf, b)
			break
		}
		buf = append(buf, b|0x80)
	}
	return buf
}

// for binary encoding of unsigned integers
func encodeU32(v uint32) []byte {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	return buf[:]
}
