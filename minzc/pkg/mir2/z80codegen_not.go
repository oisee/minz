package mir2

import "fmt"

// CPL only complements A. Explicitly stage each byte from its allocated home
// and write it to the result; bool has one significant bit and uses XOR 1.
func (g *z80cg) genNot(inst *Inst) {
	src, dst := g.loc(inst.Src[0]), g.loc(inst.Dst)
	width := inst.Ty.Width()
	preserveA := dst != "A"
	if preserveA {
		g.emit("    PUSH AF")
	}
	if width <= 8 {
		g.emitLDA(src)
		if width == 1 {
			g.emit("    XOR 1")
		} else {
			g.emit("    CPL")
		}
		g.emitMov(dst, "A", 8)
	} else {
		bytes := ByteWidth(inst.Ty)
		for i := 0; i < bytes; i++ {
			if i == 2 && isZ80WideInt(inst.Ty) {
				g.emit("    EXX")
			}
			from, to := lowByte(src), lowByte(dst)
			if i%2 != 0 {
				from, to = highByte(src), highByte(dst)
			}
			if isSpill(src) {
				from = src
				if i != 0 {
					from += fmt.Sprintf("+%d", i)
				}
			}
			if isSpill(dst) {
				to = dst
				if i != 0 {
					to += fmt.Sprintf("+%d", i)
				}
			}
			g.emitLDA(from)
			g.emit("    CPL")
			g.emitMov(to, "A", 8)
		}
		if isZ80WideInt(inst.Ty) {
			if width == 24 {
				g.emitLD8(highByte(dst), "0")
			}
			g.emit("    EXX")
		}
	}
	if preserveA {
		g.emit("    POP AF")
	}
	g.invalidate(dst)
}
