package hir_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
)

func compileProductionHIRFixture(t *testing.T, hm *hir.Module) hirZ80Fixture {
	t.Helper()
	return compileHIRFixtureWithOptions(t, hm, pipeline.DefaultOptions())
}

func compileHIRFixtureWithOptions(t *testing.T, hm *hir.Module, opts pipeline.Options) hirZ80Fixture {
	t.Helper()
	opts.AssertMode = "none"
	steps, err := pipeline.CompileHIRSteps(hm, opts)
	if err != nil {
		t.Fatalf("production HIR pipeline: %v", err)
	}
	if steps.MIR2Module == nil || steps.Allocation == nil || steps.Assembly == "" {
		t.Fatalf("production HIR pipeline did not expose MIR2, allocation and ASM")
	}
	return hirZ80Fixture{module: steps.MIR2Module, alloc: steps.Allocation, asm: steps.Assembly}
}

// runHIRZ80 uses the lowered function contract and its actual allocation.
// It deliberately rejects ABI shapes the oracle cannot bootstrap or observe.
func runHIRZ80(t *testing.T, fixture hirZ80Fixture, name string, args []int64) (int64, error) {
	t.Helper()
	f := fixture.module.FuncByName(name)
	if f == nil {
		return 0, fmt.Errorf("unknown function %q", name)
	}
	if len(args) != len(f.Contract.Params) {
		return 0, fmt.Errorf("%s: %d arguments, contract requires %d", name, len(args), len(f.Contract.Params))
	}
	if len(f.Contract.Returns) != 1 {
		return 0, fmt.Errorf("%s: oracle requires one return, contract has %d", name, len(f.Contract.Returns))
	}
	var boot strings.Builder
	fmt.Fprintf(&boot, "    ORG 0x%04X\n    LD SP, 0xFF00\n", testLoadAddr)
	for i, p := range f.Contract.Params {
		loc, ok := fixture.alloc.Locs[p.Reg]
		if !ok {
			return 0, fmt.Errorf("%s arg %d: no physical allocation for %s", name, i, p.Name)
		}
		width := p.Ty.Width()
		if width != 8 && width != 16 {
			return 0, fmt.Errorf("%s arg %d: unsupported width %d", name, i, width)
		}
		if loc.Kind != mir2.LocReg && loc.Kind != mir2.LocIXY && loc.Kind != mir2.LocIXY8 {
			return 0, fmt.Errorf("%s arg %d: unsupported location %+v", name, i, loc)
		}
		value := args[i] & 0xFF
		if width == 16 {
			value = args[i] & 0xFFFF
		}
		fmt.Fprintf(&boot, "    LD %s, %d\n", loc.Name, value)
	}
	fmt.Fprintf(&boot, "    CALL %s\n    DI\n    HALT\n", name)
	regs, err := runZ80Registers(t, boot.String()+fixture.asm)
	if err != nil {
		return 0, err
	}
	return hirReturnValue(f, regs)
}

// hirReturnValue reads the single contract return from post-call registers.
func hirReturnValue(f *mir2.Func, regs emulator.Registers) (int64, error) {
	name := f.Name
	ret := f.Contract.Returns[0]
	if ret.Class == mir2.ClassFlag {
		value, err := readZ80Flag(regs.F, ret.FlagCond)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", name, err)
		}
		return value, nil
	}
	var retReg string
	switch ret.Class {
	case mir2.ClassAcc:
		retReg = "A"
		if ret.Ty.Width() == 16 {
			retReg = "HL"
		}
	case mir2.ClassPointer, mir2.ClassPair:
		retReg = "HL"
	case mir2.ClassIndex:
		retReg = "DE"
	case mir2.ClassGeneral:
		retReg = "C"
		if ret.Ty.Width() == 16 {
			retReg = "HL"
		}
	default:
		return 0, fmt.Errorf("%s: unsupported return class %s", name, ret.Class)
	}
	value, ok := readZ80Register(regs, retReg)
	if !ok {
		return 0, fmt.Errorf("%s: unsupported return register %s", name, retReg)
	}
	if ret.Ty.Width() <= 8 {
		value &= 0xFF
	}
	return int64(value), nil
}

func readZ80Flag(flags uint8, cond mir2.CmpCond) (int64, error) {
	carry, zero := flags&0x01 != 0, flags&0x40 != 0
	var value bool
	switch cond {
	case mir2.CmpEq:
		value = zero
	case mir2.CmpNe:
		value = !zero
	case mir2.CmpLt, mir2.CmpUlt, mir2.CmpSubCarry:
		value = carry
	case mir2.CmpGe, mir2.CmpUge, mir2.CmpSubCarryNot:
		value = !carry
	case mir2.CmpLe, mir2.CmpUle:
		value = carry || zero
	case mir2.CmpGt, mir2.CmpUgt:
		value = !carry && !zero
	default:
		return 0, fmt.Errorf("unsupported flag condition %s", cond)
	}
	if value {
		return 1, nil
	}
	return 0, nil
}

func readZ80Register(r emulator.Registers, name string) (uint16, bool) {
	switch name {
	case "A":
		return uint16(r.A), true
	case "B":
		return r.BC >> 8, true
	case "C":
		return r.BC & 0xFF, true
	case "D":
		return r.DE >> 8, true
	case "E":
		return r.DE & 0xFF, true
	case "H":
		return r.HL >> 8, true
	case "L":
		return r.HL & 0xFF, true
	case "BC":
		return r.BC, true
	case "DE":
		return r.DE, true
	case "HL":
		return r.HL, true
	case "IX":
		return r.IX, true
	case "IY":
		return r.IY, true
	}
	return 0, false
}

func TestABIRunnerFlagReturn(t *testing.T) {
	m := &mir2.Module{Name: "flag_abi_oracle"}
	f := m.AddFunc("signed_flag_less")
	b := mir2.NewBuilder(f)
	a := b.Param("a", mir2.TyI8, mir2.ClassAcc)
	c := b.Param("b", mir2.TyI8, mir2.ClassGeneral)
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyBool, Class: mir2.ClassFlag, FlagCond: mir2.CmpLt}}
	b.SwitchToNewBlock("entry")
	result := b.CmpWithSrcTy(mir2.CmpLt, a, c, mir2.ClassFlag, false, mir2.TyI8)
	b.Ret(result)
	if err := mir2.Verify(m); err != nil {
		t.Fatal(err)
	}
	allocation := mir2.PBQPAllocate(f, mir2.ComputeLiveness(f), mir2.Z80CostTable{})
	fixture := hirZ80Fixture{module: m, alloc: allocation, asm: mustZ80Asm(mir2.Z80Codegen(m, allocation))}
	for _, tc := range []struct {
		a, b uint8
		want int64
	}{
		{0, 128, 0}, {128, 0, 1}, {127, 128, 0}, {128, 127, 1}, {0, 0, 0},
	} {
		got, err := runHIRZ80(t, fixture, f.Name, []int64{int64(tc.a), int64(tc.b)})
		if err != nil || got != tc.want {
			t.Fatalf("flag return (%d,%d): got %d, err %v, want %d\n%s", tc.a, tc.b, got, err, tc.want, fixture.asm)
		}
	}
}
