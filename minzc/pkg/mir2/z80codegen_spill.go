package mir2

import "fmt"

// Z80 carries aggregate and pointer values as addresses. Only 24/32-bit
// integers use the main/shadow register representation.
func isZ80WideInt(ty Ty) bool {
	ty = BaseOf(ty)
	return IsInt(ty) && (ty.Width() == 24 || ty.Width() == 32)
}

func z80SpillBytes(ty Ty) int {
	if isZ80WideInt(ty) {
		return ByteWidth(ty)
	}
	if ty.Width() <= 8 {
		return 1
	}
	return 2
}

// genWideSpills stages memory-backed wide operands in main/shadow register
// pairs. Wide emitters operate on pairs; a spill label is never a PUSH, shift,
// or arithmetic operand. Preserve both banks of every temporary pair.
func (g *z80cg) genWideSpills(inst *Inst) bool {
	if !isZ80WideInt(inst.Ty) && !isZ80WideInt(inst.SrcTy) {
		return false
	}
	switch inst.Op {
	case OpConst, OpMove, OpAdd, OpSub, OpAnd, OpOr, OpXor, OpShl, OpShr, OpSar, OpMul, OpExt, OpSext, OpTrunc:
	default:
		return false
	}
	info := g.regInfo
	regs := append([]Reg{inst.Dst}, inst.Src[:]...)
	var spilled []Reg
	used := map[string]bool{}
	seen := map[Reg]bool{}
	for _, r := range regs {
		if r == NoReg || seen[r] {
			continue
		}
		seen[r] = true
		loc := g.loc(r)
		if ri, ok := info[r]; ok && isZ80WideInt(ri.Ty) && isSpill(loc) {
			spilled = append(spilled, r)
		} else if isPairReg(loc) {
			used[loc] = true
		} else if pair, ok := regParent[loc]; ok {
			used[pair] = true
		}
	}
	if len(spilled) == 0 {
		return false
	}
	available := wideSpillPairs(g.fn.Name, inst.Op, used, len(spilled))
	labels := map[Reg]string{}
	for i, r := range spilled {
		pair := available[i]
		labels[r] = g.loc(r)
		g.emitf("    PUSH %s", pair)
		g.emit("    EXX")
		g.emitf("    PUSH %s", pair)
		g.emit("    EXX")
		if r != inst.Dst || inst.Src[0] == r || inst.Src[1] == r {
			g.emitf("    LD %s, (%s)", pair, labels[r])
			g.emit("    EXX")
			if info[r].Ty.Width() == 24 {
				g.emitLD8(lowByte(pair), labels[r]+"+2")
				g.emitLD8(highByte(pair), "0")
			} else {
				g.emitf("    LD %s, (%s+2)", pair, labels[r])
			}
			g.emit("    EXX")
		}
		g.physOverride[r] = pair
	}
	g.genInst(inst)
	if label, ok := labels[inst.Dst]; ok {
		pair := g.loc(inst.Dst)
		g.emitf("    LD (%s), %s", label, pair)
		g.emit("    EXX")
		if info[inst.Dst].Ty.Width() == 24 {
			g.emitLD8(label+"+2", lowByte(pair))
		} else {
			g.emitf("    LD (%s+2), %s", label, pair)
		}
		g.emit("    EXX")
	}
	for i := len(spilled) - 1; i >= 0; i-- {
		r := spilled[i]
		pair := available[i]
		delete(g.physOverride, r)
		g.emit("    EXX")
		g.emitf("    POP %s", pair)
		g.emit("    EXX")
		g.emitf("    POP %s", pair)
		g.invalidate(pair)
	}
	return true
}

// Refuse code generation when staging cannot represent every spilled operand.
func wideSpillPairs(fn string, op Op, used map[string]bool, needed int) []string {
	var available []string
	for _, pair := range []string{"HL", "DE", "BC"} {
		if !used[pair] {
			available = append(available, pair)
		}
	}
	if len(available) < needed {
		panic(fmt.Sprintf("Z80 codegen %s: %s needs %d wide spill staging pairs, only %d available", fn, op, needed, len(available)))
	}
	return available
}
