package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func callSpillTestFixture(t *testing.T) (*z80asm.Result, string, []mir2.NativeCallSpillSnapshot, *mir2.Module, *mir2.AllocResult) {
	t.Helper()
	m := &mir2.Module{}
	f := m.AddFunc("f")
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	r := b.Const(7, mir2.TyU8, mir2.ClassGeneral)
	b.CallVoid("other", nil, mir2.CallAttrs{})
	copy := b.Move(r, mir2.TyU8, mir2.ClassAcc)
	b.Ret(copy)
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{r: {Kind: mir2.LocMem}, copy: {Kind: mir2.LocReg, Name: "A"}}}
	symbol := mir2.Z80SpillLabel(f.Name, r)
	var s strings.Builder
	s.WriteString("ORG 0x400\n")
	var ds []mir2.NativeCallSpillSnapshot
	for _, stage := range []string{"save", "restore"} {
		d := mir2.NativeCallSpillSnapshot{ID: stage, CallID: "f0_b0_i1", CallInstruction: 1, Function: "f", Block: "entry", HomeReg: int(r), Symbol: symbol, Width: 1, Stage: stage}
		ops := []string{"PUSH HL", "PUSH AF", "LD A, (" + symbol + ")", "LD L, A", "POP AF", "EX (SP), HL"}
		if stage == "restore" {
			ops = []string{"EX (SP), HL", "PUSH AF", "LD A, L", "LD (" + symbol + "), A", "POP AF", "POP HL"}
		}
		for j, op := range ops {
			id := fmt.Sprintf("%s_s%d", stage, j)
			d.Instructions = append(d.Instructions, mir2.NativeCallFrameInstruction{ID: id, Assembly: op})
			fmt.Fprintf(&s, " %s ; @minz-call-spill:%s\n", op, id)
		}
		ds = append(ds, d)
		if stage == "save" {
			s.WriteString(" CALL other ; @minz-spill-call:f0_b0_i1\n")
		}
	}
	fmt.Fprintf(&s, "%s: DB 7\nother: RET\n", symbol)
	a, e := z80asm.NewAssembler().AssembleString(s.String())
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	return a, s.String(), ds, m, ar
}

func TestMemoryHostCallSpillBinding(t *testing.T) {
	a, s, d, m, ar := callSpillTestFixture(t)
	if _, _, e := resolveMemoryHostCallSpills(a, s, d, m, ar); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"call-id", "wrong-index", "result-home", "nonlive", "wide", "marker", "opcode", "unpaired", "allocation", "call-binary", "call-target", "call-marker"} {
		t.Run(name, func(t *testing.T) {
			a, s, d, m, ar := callSpillTestFixture(t)
			switch name {
			case "call-binary":
				for _, l := range a.Listing {
					if mir2.NativeAnnotationCount(l.SourceLine, "@minz-spill-call:f0_b0_i1") == 1 {
						a.Binary[l.Address-a.Origin+1] ^= 1
					}
				}
			case "call-target":
				m.Funcs[0].Blocks[0].Insts[1].Sym = "missing"
			case "call-marker":
				a.Listing[6].SourceLine = "CALL other ; @minz-spill-call:f0_b0_i10"
			case "call-id":
				d[0].CallID = "f0_b0_i0"
			case "wrong-index":
				d[0].CallInstruction = 0
			case "result-home":
				d[0].HomeReg = int(m.Funcs[0].Blocks[0].Insts[2].Dst)
			case "nonlive":
				m.Funcs[0].Blocks[0].Insts[2].Src[0] = mir2.NoReg
			case "wide":
				m.Funcs[0].Blocks[0].Insts[0].Ty = mir2.TyU32
			case "marker":
				d[0].Instructions[0].ID = "missing"
			case "opcode":
				a.Listing[6].Bytes = []byte{0xe1}
			case "unpaired":
				d = d[:1]
			case "allocation":
				ar.Locs[mir2.Reg(d[0].HomeReg)] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "D"}
			}
			if _, _, e := resolveMemoryHostCallSpills(a, s, d, m, ar); e == nil {
				t.Fatal("forged spill snapshot accepted")
			}
		})
	}
}
