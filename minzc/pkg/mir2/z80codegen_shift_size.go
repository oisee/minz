package mir2

// Reroll constant shifts as another complete native recipe. The result remains
// width-masked after every step, just like unrolling. Count zero and large
// counts retain the ordinary expansion. One-bit shifts keep carry fusion.
func (g *z80cg) genCompactConstantShift(mnem string, x *Inst, dst string, count int64, width int) bool {
	if count < 2 || count > 255 {
		return false
	}
	pair, counter := "BC", "B"
	if dst == "BC" || parentPair(dst) == "BC" {
		pair, counter = "DE", "D"
	}
	restore := g.saveLiveScratch(x, pair)
	label := g.trampIdx
	g.trampIdx++
	g.emitf("    LD %s, %d", counter, count)
	g.emitf(".shift_size_%d:", label)
	if width == 8 {
		g.emitf("    %s %s", mnem, dst)
	} else if mnem == "SLA" && dst == "HL" {
		g.emit("    ADD HL, HL")
	} else {
		first, second := highByte(dst), lowByte(dst)
		if mnem == "SLA" {
			first, second = second, first
		}
		g.emitf("    %s %s", mnem, first)
		if mnem == "SLA" {
			g.emitf("    RL %s", second)
		} else {
			g.emitf("    RR %s", second)
		}
	}
	if counter == "B" {
		g.emitf("    DJNZ .shift_size_%d", label)
	} else {
		g.emitf("    DEC %s", counter)
		g.emitf("    JRS NZ, .shift_size_%d", label)
	}
	restore()
	g.invalidate(pair)
	return true
}
