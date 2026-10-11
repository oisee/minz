package mir2

import (
	"fmt"
	"strings"
)

// NativeCallSpillSnapshot binds one finite save/restore stage, not arbitrary
// stack exchanges or a numeric stack-address range.
type NativeCallSpillSnapshot struct {
	ID              string                       `json:"id"`
	Function        string                       `json:"function"`
	Block           string                       `json:"block"`
	CallID          string                       `json:"call_id"`
	CallInstruction int                          `json:"call_instruction"`
	HomeReg         int                          `json:"home_reg"`
	Symbol          string                       `json:"symbol"`
	Width           int                          `json:"width"`
	Stage           string                       `json:"stage"`
	Instructions    []NativeCallFrameInstruction `json:"instructions"`
}

// NativeSpillWidth is the storage width used by Z80 spill codegen, including
// address-backed aggregates. Wide integers retain their complete byte width.
func NativeSpillWidth(ty Ty) int { return z80SpillBytes(ty) }
func NativeTermUses(t Term) []Reg {
	if t == nil {
		return nil
	}
	return append([]Reg(nil), t.termUses()...)
}

func (g *z80cg) beginNativeCallSpill(h callSpillHome, stage string) {
	if g.nativeCallSpillSink == nil {
		return
	}
	if g.nativeCallSpillActive != nil {
		g.err = fmt.Errorf("nested spill snapshot emission")
		return
	}
	reg := NoReg
	for r := range g.regInfo {
		if g.spillLabel(r) == h.symbol {
			reg = r
			break
		}
	}
	if reg == NoReg {
		g.err = fmt.Errorf("spill snapshot has no owned home")
		return
	}
	id := fmt.Sprintf("f%d_b%d_cs%d", g.nativeFunctionIndex, g.nativeBlockIndex, len(*g.nativeCallSpillSink))
	ii := -1
	for j, i := range g.curBlock.Insts {
		if i == h.call {
			ii = j
			break
		}
	}
	if ii < 0 {
		g.err = fmt.Errorf("spill snapshot call detached")
		return
	}
	callID := fmt.Sprintf("f%d_b%d_i%d", g.nativeFunctionIndex, g.nativeBlockIndex, ii)
	d := NativeCallSpillSnapshot{CallID: callID, CallInstruction: ii, ID: id, Function: g.fn.Name, Block: g.fn.Blocks[g.nativeBlockIndex].Label, HomeReg: int(reg), Symbol: h.symbol, Width: h.width, Stage: stage}
	*g.nativeCallSpillSink = append(*g.nativeCallSpillSink, d)
	g.nativeCallSpillActive = &(*g.nativeCallSpillSink)[len(*g.nativeCallSpillSink)-1]
}
func (g *z80cg) endNativeCallSpill() { g.nativeCallSpillActive = nil }
func (g *z80cg) annotateNativeCallSpill(s string) string {
	d := g.nativeCallSpillActive
	if d == nil || !strings.HasPrefix(s, "    ") || strings.HasPrefix(strings.TrimSpace(s), ";") {
		return s
	}
	id := fmt.Sprintf("%s_s%d", d.ID, len(d.Instructions))
	d.Instructions = append(d.Instructions, NativeCallFrameInstruction{ID: id, Assembly: strings.TrimSpace(s)})
	return s + " ; @minz-call-spill:" + id
}
func CheckNativeCallSpillMarkers(assembly string, ds []NativeCallSpillSnapshot) error {
	n := 0
	for _, d := range ds {
		for _, i := range d.Instructions {
			n++
			if i.ID == "" || NativeAnnotationCount(assembly, "@minz-call-spill:"+i.ID) != 1 {
				return fmt.Errorf("missing/duplicate call-spill marker %q", i.ID)
			}
		}
	}
	if strings.Count(assembly, "@minz-call-spill:") != n {
		return fmt.Errorf("orphan call-spill marker")
	}
	return nil
}

func (g *z80cg) emitNativeSpillCall(inst *Inst, s string) {
	if g.nativeCallSpillSink != nil {
		ii := -1
		for j, i := range g.curBlock.Insts {
			if i == inst {
				ii = j
				break
			}
		}
		id := fmt.Sprintf("f%d_b%d_i%d", g.nativeFunctionIndex, g.nativeBlockIndex, ii)
		found := false
		for _, d := range *g.nativeCallSpillSink {
			if d.CallID == id {
				found = true
				break
			}
		}
		if found {
			s += " ; @minz-spill-call:" + id
		}
	}
	g.emit(s)
}

// NativeAnnotationCount recognizes exact standalone annotation tokens after the
// instruction's first comment delimiter. It permits composed annotations while
// rejecting prefix IDs and annotations in executable text.
func NativeAnnotationCount(assembly, marker string) int {
	n := 0
	for _, line := range strings.Split(assembly, "\n") {
		parts := strings.SplitN(line, ";", 2)
		if len(parts) != 2 {
			continue
		}
		for _, token := range strings.Fields(strings.ReplaceAll(parts[1], ";", " ")) {
			if token == marker {
				n++
			}
		}
	}
	return n
}
