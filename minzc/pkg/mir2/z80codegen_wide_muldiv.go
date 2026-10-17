package mir2

import "fmt"

// genWideArithmetic uses a private stack frame, not global scratch storage.
// Both register banks, AF and IX are saved before operand staging. Only the
// destination's saved bytes are replaced before restoring the caller state.
// Thus source/destination aliasing and unrelated live registers need no free
// scratch pair, and recursive calls can execute the same emitted kernel.
func (g *z80cg) genWideArithmetic(inst *Inst) {
	const workBytes = 16
	saved := map[string][2]int{"HL": {24, 18}, "DE": {26, 20}, "BC": {28, 22}}
	g.comment("wide arithmetic: 16-byte private workspace, 16-byte register save")
	for _, p := range []string{"AF", "BC", "DE", "HL"} {
		g.emitf("    PUSH %s", p)
	}
	g.emit("    EXX")
	for _, p := range []string{"BC", "DE", "HL"} {
		g.emitf("    PUSH %s", p)
	}
	g.emit("    EXX")
	g.emit("    PUSH IX")
	for i := 0; i < workBytes/2; i++ {
		g.emit("    PUSH AF")
	}
	g.emit("    LD IX, 0")
	g.emit("    ADD IX, SP")

	stage := func(reg Reg, offset int) {
		if value, ok := g.constVals[reg]; ok {
			for i := 0; i < 4; i++ {
				g.emitf("    LD (IX+%d), %d", offset+i, uint32(value)>>uint(8*i)&255)
			}
			return
		}
		loc := g.loc(reg)
		if slots, ok := saved[loc]; ok {
			for i, slot := range []int{slots[0], slots[0] + 1, slots[1], slots[1] + 1} {
				g.emitf("    LD A, (IX+%d)", slot)
				g.emitf("    LD (IX+%d), A", offset+i)
			}
		} else if isSpill(loc) {
			for i := 0; i < (inst.Ty.Width()+7)/8; i++ {
				g.emitNativeSpillf(loc, i, 1, false, "    LD A, (%s+%d)", loc, i)
				g.emitf("    LD (IX+%d), A", offset+i)
			}
		} else {
			panic(fmt.Sprintf("wide arithmetic operand has non-wide location %s", loc))
		}
	}
	stage(inst.Src[0], 0)
	if inst.Op != OpNeg {
		stage(inst.Src[1], 4)
	}
	for i := 8; i < 12; i++ {
		g.emitf("    LD (IX+%d), 0", i)
	}
	g.emit("    LD (IX+12), 32")

	g.trampIdx++
	prefix := fmt.Sprintf(".%s_wide_%d", sanitizeIdent(g.fn.Name), g.trampIdx)
	label := func(s string) string { return prefix + "_" + s }
	negate := func(offset int) {
		g.emit("    XOR A")
		for i := 0; i < 4; i++ {
			if i != 0 {
				g.emit("    LD A, 0")
			}
			if i == 0 {
				g.emitf("    SUB (IX+%d)", offset+i)
			} else {
				g.emitf("    SBC A, (IX+%d)", offset+i)
			}
			g.emitf("    LD (IX+%d), A", offset+i)
		}
	}
	signed := inst.Op == OpSDiv || inst.Op == OpSMod
	if inst.Ty.Width() == 24 {
		// Canonicalize the unused fourth byte. Signed division uses a sign-
		// extended 32-bit magnitude internally, then stores the low 24 bits.
		for _, offset := range []int{0, 4} {
			g.emitf("    LD (IX+%d), 0", offset+3)
			if signed {
				g.emitf("    BIT 7, (IX+%d)", offset+2)
				g.emitf("    JR Z, %s", label(fmt.Sprintf("extend_%d", offset)))
				g.emitf("    LD (IX+%d), 255", offset+3)
				g.emit(label(fmt.Sprintf("extend_%d", offset)) + ":")
			}
		}
	}
	if signed {
		g.emit("    LD A, (IX+3)")
		g.emit("    AND 128")
		g.emit("    LD (IX+14), A")
		g.emit("    LD A, (IX+7)")
		g.emit("    XOR (IX+3)")
		g.emit("    AND 128")
		g.emit("    LD (IX+13), A")
		for _, offset := range []int{0, 4} {
			g.emitf("    BIT 7, (IX+%d)", offset+3)
			g.emitf("    JR Z, %s", label(fmt.Sprintf("positive_%d", offset)))
			negate(offset)
			g.emit(label(fmt.Sprintf("positive_%d", offset)) + ":")
		}
	}
	if inst.Op == OpNeg {
		negate(0)
	} else {
		g.emit(label("loop") + ":")
		if inst.Op == OpMul {
			g.emit("    SRL (IX+7)")
			for i := 6; i >= 4; i-- {
				g.emitf("    RR (IX+%d)", i)
			}
			g.emitf("    JR NC, %s", label("noadd"))
			for i := 0; i < 4; i++ {
				g.emitf("    LD A, (IX+%d)", 8+i)
				if i == 0 {
					g.emitf("    ADD A, (IX+%d)", i)
				} else {
					g.emitf("    ADC A, (IX+%d)", i)
				}
				g.emitf("    LD (IX+%d), A", 8+i)
			}
			g.emit(label("noadd") + ":")
			g.emit("    SLA (IX+0)")
			for i := 1; i < 4; i++ {
				g.emitf("    RL (IX+%d)", i)
			}
		} else {
			// Restoring division: shift the dividend through remainder, try the
			// divisor, restore on borrow, otherwise insert the quotient's low bit.
			g.emit("    SLA (IX+0)")
			for i := 1; i < 4; i++ {
				g.emitf("    RL (IX+%d)", i)
			}
			for i := 8; i < 12; i++ {
				g.emitf("    RL (IX+%d)", i)
			}
			for i := 0; i < 4; i++ {
				g.emitf("    LD A, (IX+%d)", 8+i)
				if i == 0 {
					g.emitf("    SUB (IX+%d)", 4+i)
				} else {
					g.emitf("    SBC A, (IX+%d)", 4+i)
				}
				g.emitf("    LD (IX+%d), A", 8+i)
			}
			g.emitf("    JR NC, %s", label("fits"))
			for i := 0; i < 4; i++ {
				g.emitf("    LD A, (IX+%d)", 8+i)
				if i == 0 {
					g.emitf("    ADD A, (IX+%d)", 4+i)
				} else {
					g.emitf("    ADC A, (IX+%d)", 4+i)
				}
				g.emitf("    LD (IX+%d), A", 8+i)
			}
			g.emitf("    JR %s", label("next"))
			g.emit(label("fits") + ":")
			g.emit("    SET 0, (IX+0)")
			g.emit(label("next") + ":")
		}
		g.emit("    DEC (IX+12)")
		g.emitf("    JP NZ, %s", label("loop"))
	}
	result := 8
	if inst.Op == OpDiv || inst.Op == OpSDiv || inst.Op == OpNeg {
		result = 0
	}
	if signed {
		sign := 13
		if inst.Op == OpSMod {
			sign = 14
		}
		g.emitf("    BIT 7, (IX+%d)", sign)
		g.emitf("    JR Z, %s", label("positive_result"))
		negate(result)
		g.emit(label("positive_result") + ":")
	}
	if inst.Ty.Width() == 24 {
		g.emitf("    LD (IX+%d), 0", result+3)
	}
	dst := g.loc(inst.Dst)
	if slots, ok := saved[dst]; ok {
		for i, slot := range []int{slots[0], slots[0] + 1, slots[1], slots[1] + 1} {
			g.emitf("    LD A, (IX+%d)", result+i)
			g.emitf("    LD (IX+%d), A", slot)
		}
	} else if isSpill(dst) {
		width := (inst.Ty.Width() + 7) / 8
		for i := 0; i < width; i++ {
			g.emitf("    LD A, (IX+%d)", result+i)
			g.emitNativeSpillf(dst, i, 1, true, "    LD (%s+%d), A", dst, i)
		}
	} else {
		panic(fmt.Sprintf("wide arithmetic destination has non-wide location %s", dst))
	}
	for i := 0; i < workBytes/2; i++ {
		g.emit("    POP AF")
	}
	g.emit("    POP IX")
	g.emit("    EXX")
	for _, p := range []string{"HL", "DE", "BC"} {
		g.emitf("    POP %s", p)
	}
	g.emit("    EXX")
	for _, p := range []string{"HL", "DE", "BC", "AF"} {
		g.emitf("    POP %s", p)
	}
	g.invalidate(dst)
}
