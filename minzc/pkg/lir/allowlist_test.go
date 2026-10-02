package lir

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

// Exercise every opcode (including future/unknown numeric values), not just
// today's known miscompiles. The expected set is independent of allowedPairs.
func TestBridgeAllowlist(t *testing.T) {
	t.Run("NativeConstants", judgeNativeConstants)
	for opcode := 0; opcode < 256; opcode++ {
		for _, ty := range []mir2.Ty{mir2.TyU8, mir2.TyU16, mir2.TyU32} {
			op := mir2.Op(opcode)
			allowed := ty.Width() == 8 && (op == mir2.OpConst || op == mir2.OpAdd || op == mir2.OpSub || op == mir2.OpCmp || op == mir2.OpShl || op == mir2.OpShr)
			if allowed {
				continue
			}
			t.Run(fmt.Sprintf("%d/%d", opcode, ty.Width()), func(t *testing.T) {
				for _, dst := range []mir2.Reg{3, mir2.NoReg} {
					inst := &mir2.Inst{Op: op, Dst: dst, Src: [2]mir2.Reg{1, 2}, Ty: ty, Cond: mir2.CmpEq, Sym: "callee"}
					if _, err := translateInst(inst, Z80); err == nil {
						t.Fatalf("translateInst accepted %+v", inst)
					}
					b := &mir2.Block{Label: "entry", Insts: []*mir2.Inst{inst}, Term: &mir2.TermRet{Vals: []mir2.Reg{3}}}
					if _, err := LowerMIR2Block(b, Z80, &mir2.Module{}); err == nil {
						t.Fatal("flat bridge accepted forbidden op")
					}
					if _, _, err := LowerMIR2BlockEGraph(b, Z80, &mir2.Module{}); err == nil {
						t.Fatal("egraph bridge accepted forbidden op")
					}
					if variants := TranslateInstEGraph(inst, Z80); len(variants) != 0 {
						t.Fatal("egraph variants accepted forbidden op")
					}
					if op == mir2.OpMul {
						if _, err := translateMul(inst, Z80); err == nil {
							t.Fatal("multiply entry accepted")
						}
					}
					if op == mir2.OpCall || op == mir2.OpCallIndirect {
						if _, err := translateCall(inst, Z80, &mir2.Module{}); err == nil {
							t.Fatal("call entry accepted")
						}
					}
				}
			})
		}
	}
}

// Positive evidence for constants: all byte constants execute through the
// research LIRCodegenFunc. Byte MIR2 moves remain off: B(1) returned 0
// with a selected "trunc HL→L (alias)" instruction on 2026-10-02.
func judgeNativeConstants(t *testing.T) {
	run := func(asm, reg string, input int) {
		t.Helper()
		res, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n CALL leaf\n DI\n HALT\n" + asm)
		if err != nil || len(res.Errors) > 0 {
			t.Fatalf("assemble: %v %v", err, res.Errors)
		}
		z := emulator.NewRemogattoZ80()
		if err := z.LoadMemory(0x8000, res.Binary); err != nil {
			t.Fatal(err)
		}
		z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
		if reg != "" {
			if err := z.SetRegister8(reg, uint8(input)); err != nil {
				t.Fatal(err)
			}
		}
		n := 0
		for ; !z.IsHalted() && n < 100; n++ {
			z.Step()
		}
		if n == 100 || z.GetRegisters().A != uint8(input) {
			t.Fatalf("%s(%d): A=%02x steps=%d\n%s", reg, input, z.GetRegisters().A, n, asm)
		}
	}
	f := &mir2.Func{Name: "leaf", Contract: mir2.Contract{Returns: []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}}, Blocks: []*mir2.Block{{Label: "entry", Term: &mir2.TermRet{Vals: []mir2.Reg{2}}}}}
	for input := 0; input < 256; input++ {
		f.Blocks[0].Insts = []*mir2.Inst{{Op: mir2.OpConst, Dst: 2, Ty: mir2.TyU8, Imm: int64(input)}}
		asm, err := LIRCodegenFunc(f, nil)
		if err != nil {
			t.Fatal(err)
		}
		run(asm, "", input)
	}
	f.Contract.Params = []mir2.Param{{Reg: 1, Name: "a", Ty: mir2.TyU8, Class: mir2.ClassGeneral}}
	f.Blocks[0].Insts = []*mir2.Inst{{Op: mir2.OpMove, Dst: 2, Src: [2]mir2.Reg{1, mir2.NoReg}, Ty: mir2.TyU8, SrcTy: mir2.TyU8}}
	for _, reg := range []string{"A", "B", "C", "D", "E", "H", "L"} {
		if _, err := LIRCodegenFunc(f, nil, AllocHints{1: Z80.LocByName(reg)}); err == nil {
			t.Fatalf("unjudged move from %s accepted", reg)
		}
	}
}

// A comparison returned as a value is outside the production allowlist.
// Research must reach the raw backend, without changing production policy.
func TestResearchCodegenBypassesAllowlist(t *testing.T) {
	f := &mir2.Func{Name: "raw_cmp", Contract: mir2.Contract{
		Params:  []mir2.Param{{Reg: 1, Ty: mir2.TyU8, Class: mir2.ClassAcc}, {Reg: 2, Ty: mir2.TyU8, Class: mir2.ClassGeneral}},
		Returns: []mir2.Return{{Ty: mir2.TyBool, Class: mir2.ClassAcc}},
	}, Blocks: []*mir2.Block{{Label: "entry", Insts: []*mir2.Inst{{
		Op: mir2.OpCmp, Dst: 3, Src: [2]mir2.Reg{1, 2}, Ty: mir2.TyU8, Cond: mir2.CmpUlt,
	}}, Term: &mir2.TermRet{Vals: []mir2.Reg{3}}}}}
	checkProduction := func() {
		t.Helper()
		if _, err := LIRCodegenFunc(f, nil); err == nil || !strings.Contains(err.Error(), "unsupported op cmp") {
			t.Fatalf("production must reject comparison values through the allowlist: %v", err)
		}
	}
	checkProduction()
	asm, err := LIRCodegenMultiBlockForResearch(f, nil)
	if err != nil {
		t.Fatalf("research must bypass the allowlist: %v", err)
	}
	if !strings.Contains(asm, "raw_cmp:") || !strings.Contains(asm, "CP ") {
		t.Fatalf("research must emit native comparison assembly:\n%s", asm)
	}
	checkProduction()
	// CmpNe also exercises bypassing the instruction-level condition allowlist.
	f.Blocks[0].Insts[0].Cond = mir2.CmpNe
	checkProduction()
	if _, err := LIRCodegenMultiBlockForResearch(f, nil); err != nil {
		t.Fatalf("research must bypass the instruction allowlist: %v", err)
	}
	checkProduction()
}
