package ez80

import (
	"bytes"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/codegen"
	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
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

// TestEZ80_E2E_AssembleToBytes tests the full pipeline:
// IR module → eZ80 assembly → MZA (ADL mode) → binary bytes.
func TestEZ80_E2E_AssembleToBytes(t *testing.T) {
	var buf bytes.Buffer
	gen := NewEZ80Generator(&buf, codegen.TargetEZ80ADL)

	u8Type := &ir.BasicType{Kind: ir.TypeU8}

	// fun add(a: u8, b: u8) -> u8 { return a + b }
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

	asm := buf.String()
	t.Logf("Generated assembly:\n%s", asm)

	// Assemble with MZA in eZ80 ADL mode
	a := z80asm.NewAssembler()
	a.SetCPUMode(z80asm.CPUModeEZ80ADL)
	result, err := a.AssembleString(asm)
	if err != nil {
		t.Fatalf("MZA assembly failed: %v\nSource:\n%s", err, asm)
	}

	if len(result.Binary) == 0 {
		t.Fatal("MZA produced empty binary")
	}

	t.Logf("Binary: %d bytes", len(result.Binary))

	// Verify the binary contains expected patterns:
	// PUSH IX = DD E5
	if !containsBytes(result.Binary, []byte{0xDD, 0xE5}) {
		t.Error("binary missing PUSH IX (DD E5)")
	}
	// POP IX = DD E1
	if !containsBytes(result.Binary, []byte{0xDD, 0xE1}) {
		t.Error("binary missing POP IX (DD E1)")
	}
	// ADD HL, DE = 19
	if !containsByte(result.Binary, 0x19) {
		t.Error("binary missing ADD HL, DE (19)")
	}
	// RET = C9
	if !containsByte(result.Binary, 0xC9) {
		t.Error("binary missing RET (C9)")
	}

	// Verify symbol table has 'add' label
	if _, ok := result.Symbols["add"]; !ok {
		t.Error("symbol table missing 'add' label")
	}
}

// TestEZ80_E2E_MLT_Binary tests MLT BC assembles correctly.
func TestEZ80_E2E_MLT_Binary(t *testing.T) {
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

	asm := buf.String()

	a := z80asm.NewAssembler()
	a.SetCPUMode(z80asm.CPUModeEZ80ADL)
	result, err := a.AssembleString(asm)
	if err != nil {
		t.Fatalf("MZA assembly failed: %v\nSource:\n%s", err, asm)
	}

	// MLT BC = ED 4C
	if !containsBytes(result.Binary, []byte{0xED, 0x4C}) {
		t.Errorf("binary missing MLT BC (ED 4C), got: %X", result.Binary)
	}

	t.Logf("Binary: %d bytes, hex: %X", len(result.Binary), result.Binary)
}

func containsBytes(haystack, needle []byte) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func containsByte(haystack []byte, b byte) bool {
	for _, v := range haystack {
		if v == b {
			return true
		}
	}
	return false
}

// TestEZ80_MIR2Pipeline tests the MIR2→eZ80 assembly pipeline integration.
func TestEZ80_MIR2Pipeline(t *testing.T) {
	// Build a MIR2 module with one function: add(a: u8, b: u8) -> u8
	m := &mir2.Module{Name: "test_ez80"}
	addFn := m.AddFunc("add")
	addFn.Contract.Params = []mir2.Param{
		{Name: "a", Ty: mir2.TyU8, Class: mir2.ClassAcc},
		{Name: "b", Ty: mir2.TyU8, Class: mir2.ClassCounter},
	}
	addFn.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	entry := addFn.NewBlock("entry")
	rA := addFn.AllocReg()
	rB := addFn.AllocReg()
	entry.Params = []mir2.BlockParam{
		{Dst: rA, Ty: mir2.TyU8, Class: mir2.ClassAcc},
		{Dst: rB, Ty: mir2.TyU8, Class: mir2.ClassCounter},
	}
	rSum := addFn.AllocReg()
	entry.Insts = append(entry.Insts, &mir2.Inst{
		Op: mir2.OpAdd, Dst: rSum, Ty: mir2.TyU8,
		Src: [2]mir2.Reg{rA, rB},
	})
	entry.Seal(&mir2.TermRet{Vals: []mir2.Reg{rSum}})

	// Allocate registers
	ct := mir2.Z80CostTable{}
	lr := mir2.ComputeLiveness(addFn)
	ar := mir2.PBQPAllocate(addFn, lr, ct)

	// Generate eZ80 assembly
	asm := MIR2Codegen(m, ar)

	if !strings.Contains(asm, ".ASSUME ADL=1") {
		t.Error("missing .ASSUME ADL=1")
	}
	if !strings.Contains(asm, "ORG $040045") {
		t.Error("missing ORG $040045")
	}
	if !strings.Contains(asm, "add:") {
		t.Error("missing function label 'add:'")
	}

	// Assemble the output with MZA in ADL mode
	a := z80asm.NewAssembler()
	a.SetCPUMode(z80asm.CPUModeEZ80ADL)
	result, err := a.AssembleString(asm)
	if err != nil {
		t.Fatalf("MZA assembly failed: %v\nSource:\n%s", err, asm)
	}

	if len(result.Binary) == 0 {
		t.Fatal("empty binary")
	}

	t.Logf("eZ80 assembly (%d bytes binary):\n%s", len(result.Binary), asm)
}
