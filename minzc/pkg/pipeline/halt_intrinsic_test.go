package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestTerminalHaltIntrinsicNative(t *testing.T) {
	for _, runtime := range []mir2.Z80Runtime{"", mir2.Z80RuntimeCPM} {
		t.Run(string(runtime), func(t *testing.T) {
			m := &hir.Module{Funcs: []*hir.Func{{Name: "main", RetTy: mir2.TyVoid, Body: &hir.Block{Body: []hir.Stmt{&hir.ExprStmt{Expr: &hir.CallExpr{Fn: "@mir.io.halt", Ty: mir2.TyVoid}}}}}}}
			opts := DefaultOptions()
			opts.Runtime = runtime
			if runtime != "" {
				opts.RuntimeStackBytes = 1024
			}
			steps, err := CompileHIRSteps(m, opts)
			if err != nil {
				t.Fatal(err)
			}
			asm := steps.Assembly
			start := uint16(0x8000)
			if runtime == "" {
				asm = "ORG 0x8000\n" + asm
			} else {
				start = 0x100
			}
			a, err := z80asm.NewAssembler().AssembleString(asm)
			if err != nil || len(a.Errors) > 0 {
				t.Fatalf("assemble %v %v", err, a.Errors)
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			mem := z.Memory()
			copy(mem[int(start):], a.Binary)
			z.SetRegisters(emulator.Registers{PC: start, SP: 0xF000})
			for i := 0; i < 1000000 && !z.IsHalted(); i++ {
				z.Step()
			}
			if !z.IsHalted() || z.GetIFF1() {
				t.Fatal("halt/DI not reached")
			}
			sp, haltPC, cycles := z.GetSP(), z.GetPC(), z.GetCycles()
			r := z.GetRegisters()
			if mem[r.PC] == 0x76 {
				r.PC++
			}
			z.SetRegisters(r)
			z.Unhalt()
			// Resume as after an interrupt: the emitted branch must return to DI/HALT,
			// never execute the function's ordinary RET or restore the CP/M stack.
			for i := 0; i < 1000 && !z.IsHalted(); i++ {
				z.Step()
			}
			if !z.IsHalted() || z.GetSP() != sp || z.GetIFF1() || z.GetPC() != haltPC || z.GetCycles() <= cycles {
				t.Fatal("halt resumed into ordinary control flow")
			}
		})
	}
}
func TestTerminalHaltIntrinsicRejectsArguments(t *testing.T) {
	m := &hir.Module{Funcs: []*hir.Func{{Name: "main", RetTy: mir2.TyVoid, Body: &hir.Block{Body: []hir.Stmt{&hir.ExprStmt{Expr: &hir.CallExpr{Fn: "@mir.io.halt", Ty: mir2.TyVoid, Args: []hir.Expr{&hir.IntLitExpr{Val: 1, Ty: mir2.TyU8}}}}}}}}}
	_, err := CompileHIRSteps(m, DefaultOptions())
	if err == nil || !strings.Contains(err.Error(), "halt requires") {
		t.Fatalf("error %v", err)
	}
}
