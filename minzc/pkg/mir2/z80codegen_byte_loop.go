package mir2

// Bulk intrinsics are expanded with explicit adapters and liveness-based
// preservation. They never rely on a soft register class landing in BC.
func (g *z80cg) genByteLoop(x *Inst) {
	g.materializePendingAcc(x)
	g.materializePendingFlag(x)
	restore := g.saveLiveScratch(x, "AF", "HL", "BC")
	g.emitParallelCopy([]parallelCopy{{srcName: g.loc(x.Args[0]), dstName: "HL", ty: TyPtr}})
	g.emit("    XOR A")
	g.emit("    LD BC, 0")
	label := g.trampIdx
	g.trampIdx++
	g.emitf(".byte_scan_%d:", label)
	g.emit("    CPIR")
	// Exhaustion is not a successful match. A 16-bit scalar walk wraps and
	// continues searching, so the block search must do the same.
	g.emitf("    JP NZ, .byte_scan_%d", label)
	if x.Ty == TyPtr {
		g.emit("    DEC HL")
	} else {
		// BC = -(length+1) modulo 65536. This is HL-1-base without
		// keeping the base alive across the scan.
		g.emit("    LD HL, 65535")
		g.emit("    OR A")
		g.emit("    SBC HL, BC")
	}
	g.emitMov(g.loc(x.Dst), "HL", 16)
	restore()
	for _, p := range []string{"A", "HL", "BC"} {
		g.invalidate(p)
	}
	g.lastFlagsLhs, g.lastFlagsRhs = "", ""
}
