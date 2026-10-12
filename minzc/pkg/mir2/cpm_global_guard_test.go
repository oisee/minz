package mir2

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func cpmGlobalGuardFixture(offset int64, ty Ty) (*Module, *AllocResult) {
	m := &Module{Globals: []Global{{Name: "object", Ty: &ArrayTy{Elem: TyU8, Len: 4}, Init: []byte{11, 22, 33, 44}}}}
	f := m.AddFunc("main")
	b := f.NewBlock("entry")
	b.Insts = []*Inst{{Op: OpAddrOf, Dst: 1, Ty: TyPtr, Sym: "object", Cls: ClassPointer}, {Op: OpConst, Dst: 2, Ty: TyU16, Imm: offset, Cls: ClassIndex}, {Op: OpPtrAdd, Dst: 3, Ty: TyPtr, Src: [2]Reg{1, 2}, Cls: ClassPointer}, {Op: OpConst, Dst: 4, Ty: ty, Imm: 0x1234, Cls: ClassAcc}, {Op: OpStore, Ty: ty, Src: [2]Reg{3, 4}}}
	if ty == TyU8 {
		b.Insts[3].Imm = 0x34
	}
	b.Term = &TermRet{}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "HL"}, 2: {Kind: LocReg, Name: "BC"}, 3: {Kind: LocReg, Name: "HL"}, 4: {Kind: LocReg, Name: "A"}}}
	if ty == TyU16 {
		ar.Locs[4] = PhysLoc{Kind: LocReg, Name: "DE"}
	}
	return m, ar
}
func TestCPMGlobalGuardNativeBounds(t *testing.T) {
	for _, ty := range []Ty{TyU8, TyU16} {
		for _, off := range []int64{0, 1, 2, 3, 4, 65535} {
			t.Run(fmt.Sprintf("%s/%d", ty, off), func(t *testing.T) {
				m, ar := cpmGlobalGuardFixture(off, ty)
				if e := Verify(m); e != nil {
					t.Fatal(e)
				}
				text, e := Z80Codegen(m, ar, Z80CodegenOptions{Runtime: Z80RuntimeCPM, RuntimeStackBytes: 128})
				if e != nil {
					t.Fatal(e)
				}
				a, e := z80asm.NewAssembler().AssembleString(text)
				if e != nil || len(a.Errors) > 0 {
					t.Fatal(e, a.Errors)
				}
				z := emulator.NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				mem := z.Memory()
				copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
				copy(mem[0x100:], a.Binary)
				copy(mem[0x7000:], []byte{0xcd, 0, 1, 0})
				before := append([]byte(nil), mem...)
				z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357})
				done := false
				minSP := uint16(a.Symbols["__cpm_stack_top"])
				for n := 0; n < 10000; n++ {
					if z.GetPC() == 0x7003 {
						done = true
						break
					}
					if z.GetSP() <= uint16(a.Symbols["__cpm_stack_top"]) && z.GetSP() < minSP {
						minSP = z.GetSP()
					}
					z.Step()
				}
				if !done || z.GetSP() != 0xf000 {
					t.Fatal("return/SP", z.GetRegisters())
				}
				valid := off+int64(ByteWidth(ty)) <= 4
				status := byte(2)
				if valid {
					status = 0
				}
				if mem[a.Symbols["__cpm_status"]] != status {
					t.Fatal("status", mem[a.Symbols["__cpm_status"]], status)
				}
				want := append([]byte(nil), before...)
				if valid {
					for j := 0; j < ByteWidth(ty); j++ {
						want[a.Symbols["object"]+int(off)+j] = byte((uint16(0x1234) >> uint(j*8)) & 255)
					}
				}
				copy(want[minSP:a.Symbols["__cpm_stack_top"]], mem[minSP:a.Symbols["__cpm_stack_top"]])
				copy(want[0xeffe:0xf000], mem[0xeffe:0xf000])
				want[a.Symbols["__cpm_status"]] = status
				copy(want[a.Symbols["__cpm_saved_sp"]:a.Symbols["__cpm_saved_sp"]+2], mem[a.Symbols["__cpm_saved_sp"]:a.Symbols["__cpm_saved_sp"]+2])
				if !bytes.Equal(want, mem) {
					for p := range mem {
						if want[p] != mem[p] {
							t.Fatalf("RAM%04x got%x want%x", p, mem[p], want[p])
						}
					}
				}
				if status == 2 {
					bad := strings.Replace(text, "LD A,2\n LD (__cpm_status),A", "LD A,0\n LD (__cpm_status),A", 1)
					if e := validateCPMRuntimeAssemblyMode(bad, true); e == nil {
						t.Fatal("mutated failure body admitted")
					}
				}
			})
		}
	}
}
func TestCPMGlobalGuardProvenanceRefusals(t *testing.T) {
	for _, kind := range []string{"narrow", "metadata", "paramtype", "ambiguous", "no-stack", "bad-home"} {
		t.Run(kind, func(t *testing.T) {
			m, ar := cpmGlobalGuardFixture(1, TyU8)
			b := m.Funcs[0].Entry()
			stack := 128
			switch kind {
			case "narrow":
				b.Insts[2] = &Inst{Op: OpMove, Dst: 3, Ty: TyU8, Src: [2]Reg{1}}
			case "metadata":
				b.Insts[2].SequentialBytes = true
			case "paramtype":
				b.Insts[2] = &Inst{Op: OpMove, Dst: 3, Ty: TyPtr, Src: [2]Reg{5}}
				b.Params = []BlockParam{{Dst: 5, Ty: TyU16}} // no authority from an entry parameter
			case "ambiguous":
				b.Insts[2].Src[0] = 5
				b.Insts = append([]*Inst{{Op: OpConst, Dst: 5, Ty: TyPtr, Imm: 123}}, b.Insts...)
			case "no-stack":
				stack = 0
			case "bad-home":
				ar.Locs[3] = PhysLoc{Kind: LocReg, Name: "A"}
			}
			text, e := Z80Codegen(m, ar, Z80CodegenOptions{Runtime: Z80RuntimeCPM, RuntimeStackBytes: stack})
			if e == nil || text != "" {
				t.Fatal("unsafe provenance accepted", e)
			}
		})
	}
}
