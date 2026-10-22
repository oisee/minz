package mir2

// Scalar alternatives retain the intrinsic's exact sequential memory semantics,
// including zero counts, byte-count 255, overlap and 16-bit pointer wraparound.
// They share the production adapters; only final encoded size admits a recipe.
func (g *z80cg) genCompactByteLoop(x *Inst) {
	if x.Sym == scanZeroIntrinsic {
		pairs := []string{"AF", "HL"}
		if x.Ty != TyPtr {
			pairs = append(pairs, "BC")
		}
		restore := g.saveByteLoopScratch(x, pairs...)
		g.emitParallelCopy([]parallelCopy{{srcName: g.loc(x.Args[0]), dstName: "HL", ty: TyPtr}})
		if x.Ty != TyPtr {
			g.emit("    LD BC, 65535")
		}
		label := g.trampIdx
		g.trampIdx++
		g.emitf(".byte_scan_scalar_%d:", label)
		if x.Ty != TyPtr {
			g.emit("    INC BC")
		}
		g.emit("    LD A, (HL)")
		g.emit("    INC HL")
		g.emit("    OR A")
		g.emitf("    JRS NZ, .byte_scan_scalar_%d", label)
		if x.Ty == TyPtr {
			g.emit("    DEC HL")
		} else {
			g.emit("    LD H, B")
			g.emit("    LD L, C")
		}
		g.emitMov(g.loc(x.Dst), "HL", 16)
		restore()
	} else if x.Sym == fillBytesIntrinsic {
		e := x.ByteLoop
		pairs := []string{"AF", "HL", "BC"}
		if e.CountTy != TyU8 || e.CountSymbol != "" && e.ValueSymbol == "" {
			pairs = append(pairs, "DE")
		}
		restore := g.saveByteLoopScratch(x, pairs...)
		counter, value := "BC", "E"
		if e.CountTy == TyU8 {
			counter, value = "B", "A"
			if e.CountSymbol != "" && e.ValueSymbol == "" {
				value = "E"
			}
		}
		moves := []parallelCopy{{srcName: g.loc(x.Args[0]), dstName: "HL", ty: TyPtr}}
		if e.CountSymbol == "" {
			moves = append(moves, parallelCopy{srcName: g.loc(x.Args[1]), dstName: counter, ty: e.CountTy})
		}
		if e.ValueSymbol == "" {
			moves = append(moves, parallelCopy{srcName: g.loc(x.Args[2]), dstName: value, ty: TyU8})
		}
		g.emitParallelCopy(moves)
		if e.CountSymbol != "" {
			g.emitf("    LD A, (%s)", sanitizeIdent(e.CountSymbol))
			g.emitLD8(counter, "A")
		}
		label := g.trampIdx
		g.trampIdx++
		if e.CountTy == TyU8 {
			g.emit("    INC B")
			g.emit("    DEC B")
		} else {
			g.emit("    LD A, B")
			g.emit("    OR C")
		}
		g.emitf("    JRS Z, .byte_fill_scalar_done_%d", label)
		if e.ValueSymbol != "" {
			g.emitf("    LD A, (%s)", sanitizeIdent(e.ValueSymbol))
			if value != "A" {
				g.emitLD8(value, "A")
			}
		}
		g.emitf(".byte_fill_scalar_%d:", label)
		g.emitf("    LD (HL), %s", value)
		g.emit("    INC HL")
		if e.CountTy == TyU8 {
			g.emitf("    DJNZ .byte_fill_scalar_%d", label)
		} else {
			g.emit("    DEC BC")
			g.emit("    LD A, B")
			g.emit("    OR C")
			g.emitf("    JRS NZ, .byte_fill_scalar_%d", label)
		}
		g.emitf(".byte_fill_scalar_done_%d:", label)
		restore()
	} else {
		restore := g.saveByteLoopScratch(x, "AF", "HL", "DE", "BC")
		counter := "BC"
		if x.ByteLoop.CountTy == TyU8 {
			counter = "B"
		}
		g.emitParallelCopy([]parallelCopy{{srcName: g.loc(x.Args[0]), dstName: "DE", ty: TyPtr}, {srcName: g.loc(x.Args[2]), dstName: "HL", ty: TyPtr}, {srcName: g.loc(x.Args[1]), dstName: counter, ty: x.ByteLoop.CountTy}})
		g.emit("    LD A, B")
		if x.ByteLoop.CountTy == TyU8 {
			g.emit("    OR A")
		} else {
			g.emit("    OR C")
		}
		label := g.trampIdx
		g.trampIdx++
		g.emitf("    JRS Z, .byte_copy_scalar_done_%d", label)
		g.emitf(".byte_copy_scalar_%d:", label)
		g.emit("    LD A, (HL)")
		g.emit("    LD (DE), A")
		g.emit("    INC HL")
		g.emit("    INC DE")
		if x.ByteLoop.CountTy == TyU8 {
			g.emitf("    DJNZ .byte_copy_scalar_%d", label)
		} else {
			g.emit("    DEC BC")
			g.emit("    LD A, B")
			g.emit("    OR C")
			g.emitf("    JRS NZ, .byte_copy_scalar_%d", label)
		}
		g.emitf(".byte_copy_scalar_done_%d:", label)
		restore()
	}
	for _, p := range []string{"A", "HL", "DE", "BC"} {
		g.invalidate(p)
	}
	g.lastFlagsLhs, g.lastFlagsRhs = "", ""
}
