package mir2

// Bulk intrinsics are expanded with explicit adapters and liveness-based
// preservation. They never rely on a soft register class landing in BC.
func (g *z80cg) genByteLoop(x *Inst) {
	g.materializePendingAcc(x)
	g.materializePendingFlag(x)
	if x.Sym == fillBytesIntrinsic {
		g.genFillBytes(x)
		return
	}
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

func (g *z80cg) genFillBytes(x *Inst) {
	restore := g.saveLiveScratch(x, "AF", "HL", "DE", "BC")
	e := x.ByteLoop
	moves := []parallelCopy{{srcName: g.loc(x.Args[0]), dstName: "HL", ty: TyPtr}}
	if e.CountSymbol == "" {
		dst := "BC"
		if e.CountTy == TyU8 {
			dst = "C"
		}
		moves = append(moves, parallelCopy{srcName: g.loc(x.Args[1]), dstName: dst, ty: e.CountTy})
	}
	if e.ValueSymbol == "" {
		moves = append(moves, parallelCopy{srcName: g.loc(x.Args[2]), dstName: "E", ty: TyU8})
	}
	g.emitParallelCopy(moves)
	if e.CountSymbol != "" {
		g.emitf("    LD A, (%s)", e.CountSymbol)
		g.emit("    LD C, A")
	}
	if e.CountTy == TyU8 {
		g.emit("    LD B, 0")
	}
	g.emit("    LD A, C")
	if e.CountTy == TyU8 {
		g.emit("    OR A")
	} else {
		g.emit("    OR B")
	}
	label := g.trampIdx
	g.trampIdx++
	g.emitf("    JRS Z, .byte_fill_done_%d", label)
	if e.ValueSymbol != "" {
		g.emitf("    LD A, (%s)", e.ValueSymbol)
	} else {
		g.emit("    LD A, E")
	}
	g.emit("    LD (HL), A")
	if e.CountTy == TyU8 {
		g.emit("    DEC C")
	} else {
		g.emit("    DEC BC")
		g.emit("    LD A, B")
		g.emit("    OR C")
	}
	g.emitf("    JRS Z, .byte_fill_done_%d", label)
	g.emit("    LD D, H")
	g.emit("    LD E, L")
	g.emit("    INC DE")
	g.emit("    LDIR")
	g.emitf(".byte_fill_done_%d:", label)
	restore()
	for _, p := range []string{"A", "HL", "DE", "BC"} {
		g.invalidate(p)
	}
	g.lastFlagsLhs, g.lastFlagsRhs = "", ""
}
