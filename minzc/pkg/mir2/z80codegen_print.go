package mir2

// Decimal printing has no MIR result. Preserve every live value occupying
// its physical scratch registers, including deferred accumulator/flag values.
// PUSH/POP AF is deliberate: callers can consume the original predicate after
// the inline printer; the arithmetic helpers instead retain their new flags.
func (g *z80cg) saveDecimalPrintScratch(inst *Inst) func() {
	live := g.scratchLiveAfter(inst)
	needed := map[string]bool{}
	for r := range live {
		if r == inst.Dst {
			continue
		}
		loc := g.loc(r)
		for _, pair := range []string{"AF", "BC", "DE"} {
			if loc == pair || parentPair(loc) == pair || pair == "AF" && (loc == "A" || loc == "F") {
				needed[pair] = true
			}
		}
	}
	if g.pendingFlagReg != NoReg {
		if live[g.pendingFlagReg] {
			needed["AF"] = true
		} else {
			g.pendingFlagReg = NoReg
		}
	}
	if g.pendingAccReg != NoReg {
		if live[g.pendingAccReg] {
			needed["AF"] = true
		} else {
			g.pendingAccReg = NoReg
		}
	}
	var saved []string
	for _, pair := range []string{"AF", "BC", "DE"} {
		if needed[pair] {
			g.emitf("    PUSH %s", pair)
			saved = append(saved, pair)
		}
	}
	return func() {
		for i := len(saved) - 1; i >= 0; i-- {
			g.emitf("    POP %s", saved[i])
		}
		for _, reg := range []string{"A", "B", "C", "D"} {
			g.invalidate(reg)
		}
		g.lastFlagsLhs, g.lastFlagsRhs = "", ""
	}
}
