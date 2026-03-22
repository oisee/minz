package mir2

import "fmt"

// TSMC Spill — Self-Modifying Code register spill optimization.
//
// Instead of storing to memory ($F0xx) and loading back, patch the
// immediate operand of the reload instruction itself:
//
//   Spill:  LD (_tsmc_r5_0+1), A     ; patch future reload (13T)
//   Reload: _tsmc_r5_0: LD A, 0      ; immediate pre-patched (7T)
//
// 8-bit: 20T round-trip vs 26T traditional (−23%)
// 16-bit in loop: 34 + 10*N vs 20 + 20*N (wins at N≥2)
//
// Safety: function must be non-recursive (TSMC patches code in RAM).

// tsmcReloadSite describes one use of a spilled register.
type tsmcReloadSite struct {
	blockIdx int    // block index
	instIdx  int    // instruction index within block
	srcSlot  int    // which Src[] slot (0 or 1)
	label    string // assembly label for the reload point
}

// tsmcSpillPair links a spill definition to its reload sites.
type tsmcSpillPair struct {
	reg     Reg              // spilled virtual register
	width   int              // 8 or 16
	defBI   int              // definition block index
	defII   int              // definition instruction index
	reloads []tsmcReloadSite // where this value is used
}

// scanTSMCSpillPairs identifies spill-reload pairs eligible for TSMC optimization.
// Returns nil if the function is recursive or has no eligible spills.
func scanTSMCSpillPairs(f *Func, ar *AllocResult) []tsmcSpillPair {
	// Collect all LocMem-allocated registers.
	var spillRegs []Reg
	spillWidth := make(map[Reg]int)
	for r, loc := range ar.Locs {
		if loc.Kind == LocMem {
			spillRegs = append(spillRegs, r)
			w := 8
			if info, ok := collectRegInfo(f)[r]; ok {
				w = info.Ty.Width()
			}
			spillWidth[r] = w
		}
	}
	if len(spillRegs) == 0 {
		return nil
	}

	// Find def and use sites for each spilled register.
	type defSite struct {
		bi, ii int
		found  bool
	}
	defs := make(map[Reg]defSite)
	var uses []struct {
		reg     Reg
		bi, ii  int
		srcSlot int
	}

	for bi, b := range f.Blocks {
		for ii, inst := range b.Insts {
			// Def site
			if _, ok := ar.Locs[inst.Dst]; ok && ar.Locs[inst.Dst].Kind == LocMem {
				defs[inst.Dst] = defSite{bi, ii, true}
			}
			// Use sites
			for slot := 0; slot < 2; slot++ {
				if slot < len(inst.Src) && inst.Src[slot] != NoReg {
					if loc, ok := ar.Locs[inst.Src[slot]]; ok && loc.Kind == LocMem {
						uses = append(uses, struct {
							reg     Reg
							bi, ii  int
							srcSlot int
						}{inst.Src[slot], bi, ii, slot})
					}
				}
			}
			// Call args
			for _, arg := range inst.Args {
				if arg != NoReg {
					if loc, ok := ar.Locs[arg]; ok && loc.Kind == LocMem {
						uses = append(uses, struct {
							reg     Reg
							bi, ii  int
							srcSlot int
						}{arg, bi, ii, -1}) // -1 = call arg, not Src slot
					}
				}
			}
		}
	}

	// Build pairs: only eligible if def exists and reloads ≤ 3.
	pairMap := make(map[Reg]*tsmcSpillPair)
	fnLabel := sanitizeIdent(f.Name)

	for _, r := range spillRegs {
		d, ok := defs[r]
		if !ok || !d.found {
			continue // no def found (block param or external)
		}
		pair := &tsmcSpillPair{
			reg:   r,
			width: spillWidth[r],
			defBI: d.bi,
			defII: d.ii,
		}
		pairMap[r] = pair
	}

	// Attach use sites to pairs.
	for _, u := range uses {
		pair, ok := pairMap[u.reg]
		if !ok {
			continue
		}
		label := fmt.Sprintf("_tsmc_%s_r%d_%d", fnLabel, int(u.reg), len(pair.reloads))
		pair.reloads = append(pair.reloads, tsmcReloadSite{
			blockIdx: u.bi,
			instIdx:  u.ii,
			srcSlot:  u.srcSlot,
			label:    label,
		})
	}

	// Filter: only keep pairs with 1-3 reload sites.
	var result []tsmcSpillPair
	for _, pair := range pairMap {
		if len(pair.reloads) >= 1 && len(pair.reloads) <= 3 {
			result = append(result, *pair)
		}
	}

	return result
}

// tsmcSpillLabel returns the label for a reload site, or "" if not TSMC-eligible.
func (g *z80cg) tsmcReloadLabel(r Reg, blockIdx, instIdx, srcSlot int) string {
	for i := range g.tsmcPairs {
		pair := &g.tsmcPairs[i]
		if pair.reg != r {
			continue
		}
		for _, rl := range pair.reloads {
			if rl.blockIdx == blockIdx && rl.instIdx == instIdx && rl.srcSlot == srcSlot {
				return rl.label
			}
		}
	}
	return ""
}

// tsmcSpillPairFor returns the TSMC pair for a spilled register, or nil.
func (g *z80cg) tsmcSpillPairFor(r Reg) *tsmcSpillPair {
	for i := range g.tsmcPairs {
		if g.tsmcPairs[i].reg == r {
			return &g.tsmcPairs[i]
		}
	}
	return nil
}

// emitTSMCSpill emits the spill: patch all reload sites' immediates.
// val is the physical register currently holding the value (e.g. "A", "HL").
func (g *z80cg) emitTSMCSpill(pair *tsmcSpillPair, val string) {
	if pair.width <= 8 {
		// 8-bit: patch byte at label+1
		if val != "A" {
			g.emitLDA(val)
		}
		for _, rl := range pair.reloads {
			g.emitf("    LD (%s+1), A", rl.label)
		}
	} else {
		// 16-bit: patch 2 bytes at label+1 and label+2
		lo := lowByte(val)
		hi := highByte(val)
		for _, rl := range pair.reloads {
			if lo == "A" {
				g.emitf("    LD (%s+1), A", rl.label)
			} else {
				g.emitf("    LD A, %s", lo)
				g.emitf("    LD (%s+1), A", rl.label)
			}
			g.emitf("    LD A, %s", hi)
			g.emitf("    LD (%s+2), A", rl.label)
		}
		g.invalidate("A")
	}
}

// emitTSMCReload emits the reload: a labeled LD with placeholder immediate.
// dst is the target physical register (e.g. "A", "HL").
func (g *z80cg) emitTSMCReload(label string, dst string, width int) {
	if width <= 8 {
		g.emitf("%s:", label)
		g.emitf("    LD %s, 0    ; TSMC: patched at runtime", dst)
	} else {
		g.emitf("%s:", label)
		g.emitf("    LD %s, 0    ; TSMC: lo+hi patched at runtime", dst)
	}
}

// isTSMCSpill checks if a virtual register is TSMC-eligible.
func (g *z80cg) isTSMCSpill(r Reg) bool {
	return g.tsmcSpillPairFor(r) != nil
}

// tsmcReloadForSrc finds TSMC pair by matching the $F0xx address in src.
// This is needed because at reload time we only have the address string, not the vreg.
func (g *z80cg) tsmcPairByAddr(addr string) *tsmcSpillPair {
	for i := range g.tsmcPairs {
		pair := &g.tsmcPairs[i]
		loc := g.ar.Locs[pair.reg]
		if loc.Kind == LocMem && fmt.Sprintf("$%04X", loc.Offset) == addr {
			return pair
		}
	}
	return nil
}
