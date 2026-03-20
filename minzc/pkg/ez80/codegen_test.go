package ez80

import (
	"bytes"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/codegen"
	"github.com/minz/minzc/pkg/ir"
)

func TestEZ80BackendRegistered(t *testing.T) {
	b := codegen.GetBackend("ez80", &codegen.BackendOptions{})
	if b == nil {
		t.Fatal("ez80 backend not registered")
	}
	if b.Name() != "ez80" {
		t.Errorf("expected name 'ez80', got %q", b.Name())
	}
	if b.GetFileExtension() != ".asm" {
		t.Errorf("expected extension '.asm', got %q", b.GetFileExtension())
	}
}

func TestEZ80BackendFeatures(t *testing.T) {
	b := codegen.GetBackend("ez80", &codegen.BackendOptions{})
	if b == nil {
		t.Fatal("ez80 backend not registered")
	}

	tests := []struct {
		feature string
		want    bool
	}{
		{codegen.Feature24BitPointers, true},
		{codegen.Feature16BitPointers, false},
		{codegen.FeatureHardwareMultiply, true},
		{codegen.FeatureSelfModifyingCode, false},
		{codegen.FeatureInterrupts, true},
		{codegen.FeatureShadowRegisters, true},
	}

	for _, tt := range tests {
		got := b.SupportsFeature(tt.feature)
		if got != tt.want {
			t.Errorf("SupportsFeature(%q) = %v, want %v", tt.feature, got, tt.want)
		}
	}
}

func TestTargetInfo(t *testing.T) {
	ti := codegen.TargetEZ80ADL
	if ti.PtrSize != 3 {
		t.Errorf("PtrSize = %d, want 3", ti.PtrSize)
	}
	if ti.RegWidth != 24 {
		t.Errorf("RegWidth = %d, want 24", ti.RegWidth)
	}
	if ti.StackSlot != 3 {
		t.Errorf("StackSlot = %d, want 3", ti.StackSlot)
	}
	if !ti.HasMLT {
		t.Error("HasMLT should be true")
	}
	if !ti.IsEZ80() {
		t.Error("IsEZ80() should be true")
	}

	z80 := codegen.TargetZ80Spectrum
	if z80.IsEZ80() {
		t.Error("Z80 should not be eZ80")
	}
	if z80.PtrSize != 2 {
		t.Errorf("Z80 PtrSize = %d, want 2", z80.PtrSize)
	}
}

func TestEZ80GenerateEmptyModule(t *testing.T) {
	var buf bytes.Buffer
	gen := NewEZ80Generator(&buf, codegen.TargetEZ80ADL)

	module := &ir.Module{Name: "test"}

	if err := gen.Generate(module); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, ".ASSUME ADL=1") {
		t.Error("missing .ASSUME ADL=1 directive")
	}
	if !strings.Contains(out, "ORG $040000") {
		t.Error("missing ORG $040000")
	}
	if !strings.Contains(out, "END") {
		t.Error("missing END directive")
	}
}

func TestEZ80GenerateSimpleAdd(t *testing.T) {
	var buf bytes.Buffer
	gen := NewEZ80Generator(&buf, codegen.TargetEZ80ADL)

	u8Type := &ir.BasicType{Kind: ir.TypeU8}

	module := &ir.Module{
		Name: "test",
		Functions: []*ir.Function{
			{
				Name: "add",
				Params: []ir.Parameter{
					{Name: "a", Reg: 1, Type: u8Type},
					{Name: "b", Reg: 2, Type: u8Type},
				},
				ReturnType: u8Type,
				Instructions: []ir.Instruction{
					{Op: ir.OpAdd, Dest: 3, Src1: 1, Src2: 2},
					{Op: ir.OpReturn, Src1: 3},
				},
			},
		},
	}

	if err := gen.Generate(module); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	out := buf.String()

	checks := []struct {
		needle string
		desc   string
	}{
		{"add:", "function label"},
		{"PUSH IX", "prologue PUSH IX"},
		{"ADD HL, DE", "ADD HL, DE instruction"},
		{"LD SP, IX", "epilogue LD SP, IX"},
		{"POP IX", "epilogue POP IX"},
		{"RET", "RET instruction"},
	}

	for _, c := range checks {
		if !strings.Contains(out, c.needle) {
			t.Errorf("missing %s (%q)", c.desc, c.needle)
		}
	}

	t.Logf("Generated assembly:\n%s", out)
}

func TestEZ80MLT(t *testing.T) {
	var buf bytes.Buffer
	gen := NewEZ80Generator(&buf, codegen.TargetEZ80ADL)

	u8Type := &ir.BasicType{Kind: ir.TypeU8}

	module := &ir.Module{
		Name: "test",
		Functions: []*ir.Function{
			{
				Name: "mul8",
				Params: []ir.Parameter{
					{Name: "a", Reg: 1, Type: u8Type},
					{Name: "b", Reg: 2, Type: u8Type},
				},
				ReturnType: u8Type,
				Instructions: []ir.Instruction{
					{Op: ir.OpMul, Dest: 3, Src1: 1, Src2: 2},
					{Op: ir.OpReturn, Src1: 3},
				},
			},
		},
	}

	if err := gen.Generate(module); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "MLT BC") {
		t.Errorf("expected MLT BC for u8*u8 multiply, got:\n%s", out)
	}

	t.Logf("Generated assembly:\n%s", out)
}

func TestEZ80FunctionCall(t *testing.T) {
	var buf bytes.Buffer
	gen := NewEZ80Generator(&buf, codegen.TargetEZ80ADL)

	u8Type := &ir.BasicType{Kind: ir.TypeU8}

	module := &ir.Module{
		Name: "test",
		Functions: []*ir.Function{
			{
				Name:       "callee",
				IsExtern:   true,
				Params:     []ir.Parameter{{Name: "x", Reg: 1, Type: u8Type}},
				ReturnType: u8Type,
			},
			{
				Name: "main",
				Instructions: []ir.Instruction{
					{Op: ir.OpLoadImm, Dest: 1, Imm: 42},
					{Op: ir.OpCall, Dest: 2, Label: "callee", Args: []ir.Register{1}},
					{Op: ir.OpReturn, Src1: 2},
				},
			},
		},
	}

	if err := gen.Generate(module); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "CALL callee") {
		t.Errorf("missing CALL callee in:\n%s", out)
	}

	t.Logf("Generated assembly:\n%s", out)
}
