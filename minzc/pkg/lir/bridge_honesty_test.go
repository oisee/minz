package lir

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestBridgeRejectsUnsupportedSemantics(t *testing.T) {
	for _, tc := range []struct {
		name string
		inst mir2.Inst
	}{
		{"wide", mir2.Inst{Op: mir2.OpAdd, Dst: 3, Src: [2]mir2.Reg{1, 2}, Ty: mir2.TyU32}},
		{"wide_source", mir2.Inst{Op: mir2.OpTrunc, Dst: 3, Src: [2]mir2.Reg{1, 0}, Ty: mir2.TyU8, SrcTy: mir2.TyU32}},
		{"sar", mir2.Inst{Op: mir2.OpSar, Dst: 3, Src: [2]mir2.Reg{1, 2}, Ty: mir2.TyI8}},
		{"cmp_value", mir2.Inst{Op: mir2.OpCmp, Dst: 3, Src: [2]mir2.Reg{1, 2}, Ty: mir2.TyBool, Cond: mir2.CmpUlt}},
		{"asm", mir2.Inst{Op: mir2.OpAsm}},
		{"patch", mir2.Inst{Op: mir2.OpPatch, Src: [2]mir2.Reg{1, 2}}},
		{"push", mir2.Inst{Op: mir2.OpPush, Src: [2]mir2.Reg{1, 0}}},
		{"pop", mir2.Inst{Op: mir2.OpPop}},
		{"out", mir2.Inst{Op: mir2.OpOut8, Src: [2]mir2.Reg{1, 2}}},
		{"unknown", mir2.Inst{Op: mir2.Op(255)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &mir2.Block{Label: "entry", Insts: []*mir2.Inst{&tc.inst}, Term: &mir2.TermRet{Vals: []mir2.Reg{tc.inst.Dst}}}
			if _, err := LowerMIR2Block(b, Z80, nil); err == nil {
				t.Fatal("unsupported semantics silently accepted")
			}
		})
	}
}

func TestBridgeShiftCounts(t *testing.T) {
	for _, opcode := range []mir2.Op{mir2.OpShl, mir2.OpShr} {
		for _, width := range []mir2.Ty{mir2.TyU8, mir2.TyU16} {
			for _, count := range []int64{-1, 0, 1, 2, 3, 8} {
				t.Run(fmt.Sprintf("%s/%s/%d", opcode, width, count), func(t *testing.T) {
					b := &mir2.Block{Label: "entry", Insts: []*mir2.Inst{
						{Op: mir2.OpConst, Dst: 2, Imm: count, Ty: mir2.TyU8},
						{Op: opcode, Dst: 3, Src: [2]mir2.Reg{1, 2}, Ty: width},
					}}
					_, err := LowerMIR2Block(b, Z80, nil)
					wantError := width.Width() == 16 || count != 1
					if (err != nil) != wantError {
						t.Fatalf("count %d width %d: error=%v, want rejection=%v", count, width.Width(), err, wantError)
					}
				})
			}
		}
		b := &mir2.Block{Label: "entry", Insts: []*mir2.Inst{{Op: opcode, Dst: 3, Src: [2]mir2.Reg{1, 2}, Ty: mir2.TyU8}}}
		if _, err := LowerMIR2Block(b, Z80, nil); err == nil {
			t.Fatal("variable shift accepted")
		}
	}
}

func TestBridgeCompareCondRetOnly(t *testing.T) {
	cmp := &mir2.Inst{Op: mir2.OpCmp, Dst: 3, Src: [2]mir2.Reg{1, 2}, Ty: mir2.TyBool, Cond: mir2.CmpEq}
	b := &mir2.Block{Label: "entry", Insts: []*mir2.Inst{cmp}, Term: &mir2.TermCondRet{Cond: 3, Vals: []mir2.Reg{1}, Then: "next"}}
	if _, err := LowerMIR2Block(b, Z80, nil); err != nil {
		t.Fatal(err)
	}
	b.Insts = append(b.Insts, &mir2.Inst{Op: mir2.OpMove, Dst: 4, Src: [2]mir2.Reg{3, 0}, Ty: mir2.TyBool})
	if _, err := LowerMIR2Block(b, Z80, nil); err == nil {
		t.Fatal("compare also consumed as value accepted")
	}
	b.Insts = b.Insts[:1]
	b.Term.(*mir2.TermCondRet).Vals = []mir2.Reg{3}
	if _, err := LowerMIR2Block(b, Z80, nil); err == nil {
		t.Fatal("compare returned as value accepted")
	}
}

func TestLIRPassThroughWord(t *testing.T) {
	f := &mir2.Func{Name: "identity16", Contract: mir2.Contract{
		Params:  []mir2.Param{{Name: "a", Reg: 1, Ty: mir2.TyU16, Class: mir2.ClassIndex}},
		Returns: []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}},
	}, Blocks: []*mir2.Block{{Label: "entry", Term: &mir2.TermRet{Vals: []mir2.Reg{1}}}}}
	asm, err := LIRCodegenFunc(f, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Returning a DE parameter in HL must copy both bytes, not an 8-bit fragment.
	if !strings.Contains(asm, "EX DE, HL") && !(strings.Contains(asm, "LD H, D") && strings.Contains(asm, "LD L, E")) {
		t.Fatalf("missing word return move:\n%s", asm)
	}
}

func TestLIRPassThroughWordExecuted(t *testing.T) {
	f := &mir2.Func{Name: "identity16", Contract: mir2.Contract{
		Params:  []mir2.Param{{Name: "a", Reg: 1, Ty: mir2.TyU16, Class: mir2.ClassIndex}},
		Returns: []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}},
	}, Blocks: []*mir2.Block{{Label: "entry", Term: &mir2.TermRet{Vals: []mir2.Reg{1}}}}}
	asm, err := LIRCodegenFunc(f, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n CALL identity16\n DI\n HALT\n" + asm)
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("assemble: %v %v", err, res.Errors)
	}
	z := emulator.NewRemogattoZ80()
	for input := 0; input < 65536; input++ {
		z.Reset()
		if err := z.LoadMemory(0x8000, res.Binary); err != nil {
			t.Fatal(err)
		}
		z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00, DE: uint16(input), HL: 0xA55A})
		n := 0
		for ; !z.IsHalted() && n < 100; n++ {
			z.Step()
		}
		if n == 100 || z.GetRegisters().HL != uint16(input) {
			t.Fatalf("identity16(%d): got %d (steps %d)\n%s", input, z.GetRegisters().HL, n, asm)
		}
	}
}
