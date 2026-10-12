package mir2

import "fmt"

// Keep the address in IX while EXX exposes the upper value word. BC/DE/HL
// addresses would switch to the unrelated shadow address at that point.
// A is the encoding bridge for every byte, including memory-backed values.
func (g *z80cg) genWideMemory(inst *Inst) bool {
	if (inst.Op != OpLoad && inst.Op != OpStore) || !isZ80WideInt(inst.Ty) {
		return false
	}
	ptr := g.loc(inst.Src[0])
	valueReg := inst.Dst
	value := g.loc(inst.Dst)
	if inst.Op == OpStore {
		valueReg = inst.Src[1]
		value = g.loc(valueReg)
	}
	valueWidth := inst.Ty.Width()
	if info, ok := g.regInfo[valueReg]; ok {
		valueWidth = info.Ty.Width()
	}
	if inst.Op == OpLoad {
		valueWidth = inst.Ty.Width()
	}
	if valueWidth > 16 && !isSpill(value) && value != "HL" && value != "DE" && value != "BC" {
		g.err = fmt.Errorf("wide memory %s %s: invalid value home %s (src=%v dst=%d)", g.fn.Name, inst.Op, value, inst.Src, inst.Dst)
		return true
	}
	g.emit("    PUSH AF")
	g.emit("    PUSH IX")
	if isSpill(ptr) {
		g.emitNativeSpillf(ptr, 0, 2, false, "    LD IX, (%s)", ptr)
	} else if ptr != "IX" {
		g.emitf("    PUSH %s", ptr)
		g.emit("    POP IX")
	}
	width := ByteWidth(inst.Ty)
	for offset := 0; offset < width; offset++ {
		if offset == 2 && valueWidth > 16 && !isSpill(value) {
			g.emit("    EXX")
		}
		byteReg := lowByte(value)
		if offset%2 != 0 {
			byteReg = highByte(value)
		}
		symbol := value
		if offset != 0 {
			symbol += fmt.Sprintf("+%d", offset)
		}
		if inst.Op == OpLoad {
			g.emitNativeMemoryf(inst, offset, 1, "    LD A, (IX+%d)", offset)
			if isSpill(value) {
				g.emitNativeSpillf(value, offset, 1, true, "    LD (%s), A", symbol)
			} else {
				g.emitLD8(byteReg, "A")
			}
		} else {
			if offset*8 >= valueWidth {
				g.emit("    LD A, 0")
			} else if isSpill(value) {
				g.emitNativeSpillf(value, offset, 1, false, "    LD A, (%s)", symbol)
			} else {
				if valueWidth <= 8 {
					byteReg = value
				}
				g.emitLD8("A", byteReg)
			}
			g.emitNativeMemoryf(inst, offset, 1, "    LD (IX+%d), A", offset)
		}
	}
	if !isSpill(value) && valueWidth > 16 {
		if width == 3 && inst.Op == OpLoad {
			g.emitLD8(highByte(value), "0")
		}
		g.emit("    EXX")
	}
	g.emit("    POP IX")
	g.emit("    POP AF")
	if inst.Op == OpLoad {
		g.invalidate(value)
	}
	return true
}
