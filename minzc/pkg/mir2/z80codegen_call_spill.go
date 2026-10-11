package mir2

import (
	"fmt"
	"slices"
)

// Absolute compiler spill homes are shared by recursive activations. Protect
// the caller's live homes before argument copies can overwrite callee slots.
// These snapshots supplement register saves; they do not authorize new native
// memory-host frame accesses.
type callSpillHome struct {
	symbol string
	width  int
	call   *Inst
}

func (g *z80cg) callerSaveSpillHomes(inst *Inst) []callSpillHome {
	excluded := map[Reg]bool{inst.Dst: true}
	for _, r := range inst.ExtraRets {
		excluded[r] = true
	}
	var regs []Reg
	for r := range g.regsLiveAfterInst(inst) {
		if !excluded[r] && g.loc(r) == g.spillLabel(r) {
			regs = append(regs, r)
		}
	}
	slices.Sort(regs)
	var homes []callSpillHome
	for _, r := range regs {
		info, ok := g.regInfo[r]
		// Respect dynamic relocation: a spill home containing an obsolete value
		// need not be restored over a newer call result.
		if !ok || g.loc(r) != g.spillLabel(r) || g.isTSMCSpill(r) {
			continue
		}
		width := z80SpillBytes(info.Ty)
		if width != 1 && width != 2 {
			g.err = fmt.Errorf("caller spill save: unsupported width %d for r%d", width, r)
			continue
		}
		homes = append(homes, callSpillHome{symbol: g.spillLabel(r), width: width, call: inst})
	}
	return homes
}

func (g *z80cg) pushCallSpillHomes(homes []callSpillHome) {
	for _, h := range homes {
		g.beginNativeCallSpill(h, "save")
		g.emit("    PUSH HL")
		if h.width == 2 {
			g.emitNativeSpill(h.symbol, 0, 2, false, fmt.Sprintf("    LD HL, (%s)", h.symbol))
		} else {
			g.emit("    PUSH AF")
			g.emitNativeSpill(h.symbol, 0, 1, false, fmt.Sprintf("    LD A, (%s)", h.symbol))
			g.emit("    LD L, A")
			g.emit("    POP AF")
		}
		// Replace the saved HL word with the home snapshot while restoring HL.
		g.emit("    EX (SP), HL")
		g.endNativeCallSpill()
	}
}

func (g *z80cg) popCallSpillHomes(homes []callSpillHome) {
	for i := len(homes) - 1; i >= 0; i-- {
		h := homes[i]
		g.beginNativeCallSpill(h, "restore")
		// Retain the already-picked-up result HL on the stack during restoration.
		g.emit("    EX (SP), HL")
		if h.width == 2 {
			g.emitNativeSpill(h.symbol, 0, 2, true, fmt.Sprintf("    LD (%s), HL", h.symbol))
		} else {
			g.emit("    PUSH AF")
			g.emit("    LD A, L")
			g.emitNativeSpill(h.symbol, 0, 1, true, fmt.Sprintf("    LD (%s), A", h.symbol))
			g.emit("    POP AF")
		}
		g.emit("    POP HL")
		g.endNativeCallSpill()
	}
}
