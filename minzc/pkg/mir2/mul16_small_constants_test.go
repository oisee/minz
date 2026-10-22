package mir2

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMul16Constants10And12Physical(t *testing.T) {
	var rows []map[string]any
	for _, k := range []int64{10, 12} {
		for _, dst := range []string{"HL", "DE", "IX", "spill"} {
			m := &Module{}
			f := m.AddFunc("multiply")
			b := f.NewBlock("entry")
			x, c, out := f.AllocReg(), f.AllocReg(), f.AllocReg()
			f.Contract.Params = []Param{{Name: "x", Reg: x, Ty: TyU16, Class: ClassPointer}}
			b.Append(&Inst{Op: OpConst, Dst: c, Imm: k, Ty: TyU16, Cls: ClassGeneral})
			mul := &Inst{Op: OpMul, Dst: out, Src: [2]Reg{x, c}, Ty: TyU16, Cls: ClassPointer}
			// The preservation assertions below require actual post-multiply uses.
			// Keep BC live and keep the original HL operand when dst is elsewhere.
			liveBC := f.AllocReg()
			b.Append(&Inst{Op: OpConst, Dst: liveBC, Ty: TyU16, Imm: 0x1234})
			b.Append(mul)
			b.Append(&Inst{Op: OpMove, Dst: f.AllocReg(), Src: [2]Reg{liveBC}, Ty: TyU16})
			if dst != "HL" {
				b.Append(&Inst{Op: OpMove, Dst: f.AllocReg(), Src: [2]Reg{x}, Ty: TyU16})
			}
			b.Seal(&TermRet{Vals: []Reg{out}})
			ar := &AllocResult{Locs: map[Reg]PhysLoc{x: {Kind: LocReg, Name: "HL"}, c: {Kind: LocReg, Name: "BC"}, out: {Kind: LocReg, Name: dst}}}
			ar.Locs[liveBC] = PhysLoc{Kind: LocReg, Name: "BC"}
			if dst == "spill" {
				ar.Locs[out] = PhysLoc{Kind: LocMem}
				ar.Spilled = []Reg{out}
			}
			buf := new(strings.Builder)
			g := &z80cg{mod: m, fn: f, ar: ar, sb: buf, curBlock: b, liveness: ComputeLiveness(f), regInfo: collectRegInfo(f), constVals: map[Reg]int64{c: k}, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, pendingAccReg: NoReg, pendingFlagReg: NoReg}
			g.genMul16(mul)
			asm := "ORG $8000\nmultiply:\n" + buf.String() + " RET\n"
			if dst == "spill" {
				asm += g.spillLabel(out) + ": DW 0\n"
			}
			result, err := z80asm.NewAssembler().AssembleString(asm)
			if err != nil || len(result.Errors) > 0 {
				t.Fatalf("assemble%v %v", err, result.Errors)
			}
			for _, value := range []uint16{0, 1, 2, 255, 256, 257, 5461, 6553, 32767, 32768, 65534, 65535} {
				for _, carry := range []byte{0, 1} {
					vm, err := NewVM(m).Call(f.Name, []Value{{I: int64(value)}})
					want := uint16(uint32(value) * uint32(k))
					if err != nil || len(vm) != 1 || uint16(vm[0].I) != want {
						t.Fatalf("VM %v %v", vm, err)
					}
					z := emulator.NewRemogattoZ80()
					z.MaxCycles = 10000
					z.LoadMemory(0x8000, result.Binary)
					regs := z.GetRegisters()
					regs.PC = 0x8000
					regs.SP = 0xf000
					regs.HL = value
					regs.BC = 0x1234
					regs.DE = 0x5678
					regs.IX = 0x9abc
					regs.A = 0x37
					regs.F = 0xa4 | carry
					z.SetRegisters(regs)
					z.SetMemory(0xf000, 0)
					z.SetMemory(0xf001, 0x70)
					for steps := 0; z.GetPC() != 0x7000; steps++ {
						if steps > 10000 {
							t.Fatal("did not return")
						}
						z.Step()
					}
					after := z.GetRegisters()
					got := after.HL
					switch dst {
					case "DE":
						got = after.DE
					case "IX":
						got = after.IX
					case "spill":
						addr := uint16(result.Symbols[g.spillLabel(out)])
						got = uint16(z.GetMemory(addr)) | uint16(z.GetMemory(addr+1))<<8
					}
					flagMask := byte(0xc4) // ADD HL,rr preserves S/Z/PV; H/N/C describe its result.
					if dst == "spill" {
						flagMask = 0
					} // spill-store helper legitimately modifies flags.
					if after.F&flagMask != regs.F&flagMask {
						t.Fatalf("k%d dst%s live S/Z/PV changed", k, dst)
					}
					if got != want || after.A != regs.A || after.BC != regs.BC || after.SP != 0xf002 || (dst != "HL" && after.HL != regs.HL) || (dst != "DE" && after.DE != regs.DE) || (dst != "IX" && after.IX != regs.IX) {
						t.Fatalf("k%d dst%s x%d result%d want%d regs%+v", k, dst, value, got, want, after)
					}
					rows = append(rows, map[string]any{"constant": k, "destination": dst, "input": value, "carry": carry, "result": got, "bytes": len(result.Binary), "t_states": z.GetCycles(), "before": regs, "after": after})
				}
			}
			if dir := os.Getenv("MINZ_MUL16_EVIDENCE"); dir != "" {
				os.MkdirAll(dir, 0755)
				base := filepath.Join(dir, fmt.Sprintf("mul%d-%s", k, dst))
				os.WriteFile(base+".a80", []byte(asm), 0644)
				os.WriteFile(base+".bin", result.Binary, 0644)
				os.WriteFile(base+".mir2", []byte(m.Dump()), 0644)
			}
		}
	}
	if dir := os.Getenv("MINZ_MUL16_EVIDENCE"); dir != "" {
		data, _ := json.MarshalIndent(rows, "", "  ")
		os.WriteFile(filepath.Join(dir, "results.json"), data, 0644)
	}
	t.Logf("%d physical VM/Z80 checks", len(rows))
}
